package echo

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/sagernet/sing/common/metadata"
)

// Client 表示TCP Echo客户端
type Client struct {
	serverAddr string
	timeout    time.Duration
	conn       net.Conn
	stats      *Stats
	connTime   time.Duration
	mu         sync.Mutex
	dialer     func(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error)
}

// Stats 包含延迟统计信息
type Stats struct {
	mu          sync.Mutex
	Latencies   []time.Duration
	Min         time.Duration
	Max         time.Duration
	Sum         time.Duration
	Count       int
	Errors      int
	ConnectTime time.Duration
}

// Result 表示单次测试结果
type Result struct {
	Latency    time.Duration
	Error      error
	Success    bool
	DataSize   int
	IsMatching bool
}

// ClientOption 是Client的选项函数类型
type ClientOption func(*Client)

// NewClient 创建一个新的TCP Echo客户端
func NewClient(serverAddr string, options ...ClientOption) *Client {
	client := &Client{
		serverAddr: serverAddr,
		timeout:    5 * time.Second,
		stats:      &Stats{},
	}

	// 应用所有选项
	for _, option := range options {
		option(client)
	}

	return client
}

// WithTimeout 设置超时选项
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithDialer 设置自定义拨号器选项
// 通过此选项可以自定义TCP连接的各种参数，例如：
// - 设置本地地址
// - 设置TCP保活时间
// - 设置TCP缓冲区大小
// - 其他net.Dialer支持的参数
// 例如:
//
//	dialer := &net.Dialer{
//	  LocalAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100")},
//	  KeepAlive: 30 * time.Second,
//	}
//	client := NewClient("example.com:8080", WithDialer(dialer))
func WithDialer(dialer func(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error)) ClientOption {
	return func(c *Client) {
		c.dialer = dialer
	}
}

// Connect 建立到服务器的连接
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已经连接，先关闭
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	// 创建一个带超时的上下文
	ctxTimeout, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 计时并创建连接
	start := time.Now()

	var conn net.Conn
	var err error

	// 使用自定义拨号器或创建默认拨号器
	if c.dialer != nil {
		conn, err = c.dialer(ctxTimeout, "tcp", metadata.ParseSocksaddr(c.serverAddr))
	} else {
		var d net.Dialer
		conn, err = d.DialContext(ctxTimeout, "tcp", c.serverAddr)
	}

	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	c.conn = conn
	c.connTime = elapsed
	c.stats.ConnectTime = elapsed

	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// ConnectTime 返回连接建立时间
func (c *Client) ConnectTime() time.Duration {
	return c.connTime
}

// Send 发送数据并等待回应
func (c *Client) Send(ctx context.Context, data []byte) ([]byte, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, 0, fmt.Errorf("未连接到服务器")
	}

	// 设置读写超时
	_ = c.conn.SetDeadline(time.Now().Add(c.timeout))

	// 记录开始时间
	start := time.Now()

	// 发送数据
	_, err := c.conn.Write(data)
	if err != nil {
		c.stats.addError()
		return nil, 0, fmt.Errorf("发送失败: %w", err)
	}

	// 接收响应
	response := make([]byte, len(data))
	_, err = io.ReadFull(c.conn, response)

	// 计算延迟
	elapsed := time.Since(start)

	if err != nil {
		c.stats.addError()
		if err == io.EOF {
			return nil, elapsed, fmt.Errorf("连接被服务器关闭")
		}
		return nil, elapsed, fmt.Errorf("接收失败: %w", err)
	}

	// 添加到统计信息
	c.stats.addLatency(elapsed)
	return response, elapsed, nil
}

// Test 进行一次测试，返回测试结果
func (c *Client) Test(ctx context.Context, data []byte) Result {
	response, latency, err := c.Send(ctx, data)

	result := Result{
		Latency:  latency,
		Error:    err,
		Success:  err == nil,
		DataSize: len(data),
	}

	if err == nil {
		// 检查数据是否匹配
		result.IsMatching = compareData(data, response)
	}

	return result
}

// MultiTest 执行多次测试并返回所有结果
func (c *Client) MultiTest(ctx context.Context, data []byte, count int, interval time.Duration) []Result {
	results := make([]Result, 0, count)

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return results
		default:
			result := c.Test(ctx, data)
			results = append(results, result)

			// 如果不是最后一次测试，等待间隔时间
			if i < count-1 {
				select {
				case <-ctx.Done():
					return results
				case <-time.After(interval):
					// 继续下一次迭代
				}
			}
		}
	}

	return results
}

// GetStats 返回当前统计信息
func (c *Client) GetStats() Stats {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	// 返回一个副本
	return Stats{
		Latencies:   append([]time.Duration{}, c.stats.Latencies...),
		Min:         c.stats.Min,
		Max:         c.stats.Max,
		Sum:         c.stats.Sum,
		Count:       c.stats.Count,
		Errors:      c.stats.Errors,
		ConnectTime: c.stats.ConnectTime,
	}
}

// GetStatsSummary 返回统计信息的摘要
func (c *Client) GetStatsSummary() string {
	return c.stats.getSummary()
}

// ResetStats 重置所有统计信息
func (c *Client) ResetStats() {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	c.stats.Latencies = c.stats.Latencies[:0]
	c.stats.Min = 0
	c.stats.Max = 0
	c.stats.Sum = 0
	c.stats.Count = 0
	c.stats.Errors = 0
	// 注意: 不重置ConnectTime
}

// 辅助函数

func (s *Stats) addLatency(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Latencies = append(s.Latencies, d)
	s.Count++
	s.Sum += d

	if s.Min == 0 || d < s.Min {
		s.Min = d
	}
	if d > s.Max {
		s.Max = d
	}
}

func (s *Stats) addError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Errors++
}

func (s *Stats) getSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Count == 0 {
		return "没有收集到任何延迟数据"
	}

	avg := s.Sum / time.Duration(s.Count)

	var median time.Duration
	if len(s.Latencies) > 0 {
		// 复制切片，避免修改原始数据
		sorted := make([]time.Duration, len(s.Latencies))
		copy(sorted, s.Latencies)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})
		median = sorted[len(sorted)/2]
	}

	var jitter time.Duration
	if len(s.Latencies) > 1 {
		var sum time.Duration
		prev := s.Latencies[0]
		for i := 1; i < len(s.Latencies); i++ {
			diff := s.Latencies[i] - prev
			if diff < 0 {
				diff = -diff
			}
			sum += diff
			prev = s.Latencies[i]
		}
		jitter = sum / time.Duration(len(s.Latencies)-1)
	}

	// 计算标准差
	var variance float64
	mean := float64(avg)
	for _, v := range s.Latencies {
		variance += (float64(v) - mean) * (float64(v) - mean)
	}
	variance /= float64(len(s.Latencies))
	stdDev := time.Duration(math.Sqrt(variance))

	return fmt.Sprintf(`
延迟统计:
  连接建立时间: %s
  发送包数: %d
  接收包数: %d
  丢包率: %.2f%%
  最小延迟: %s
  最大延迟: %s
  平均延迟: %s
  中位数延迟: %s
  抖动: %s
  标准差: %s
`,
		s.ConnectTime,
		s.Count+s.Errors,
		s.Count,
		float64(s.Errors)*100/float64(s.Count+s.Errors),
		s.Min,
		s.Max,
		avg,
		median,
		jitter,
		stdDev,
	)
}

// 比较两个数据切片是否相等
func compareData(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
