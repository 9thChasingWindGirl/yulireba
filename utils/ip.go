package utils

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/r10v/gowindows"
)

// NetworkInfo 包含网卡的网络信息
type NetworkInfo struct {
	InterfaceName string // 网卡名称
	Subnet        string // IP段/子网掩码
	IfIndex       int
	Gateway       string
	Metric        int
}

// GetDefaultNetworkInfo 获取默认网卡的IP段和网关信息
func GetDefaultNetworkInfo() (*NetworkInfo, error) {
	iface, err := GetDefaultInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to get default interface: %v", err)
	}
	inter, err := net.InterfaceByName(iface.Name)
	if err != nil {
		log.Printf("无法获取信息: %v", err)
	}
	addrs, err := inter.Addrs()
	if err != nil {
		log.Println(err)
	}
	// 获取IP地址，子网掩码
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				gateway, metric, err2 := getDefaultGateway(iface.Index)
				if err2 != nil {
					return nil, err2
				}
				return &NetworkInfo{
					InterfaceName: iface.Name,
					Subnet:        ip.String(),
					IfIndex:       iface.Index,
					Gateway:       gateway,
					Metric:        metric,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("no default interface found")
}
func GetDefaultInterface() (*net.Interface, error) {
	table, err := gowindows.GetIpForwardTable()
	if err != nil {
		return nil, err
	}
	minM := 0
	index := 0
	for _, row := range table {
		if row.ForwardDest[0] == 0 && row.ForwardDest[1] == 0 && row.ForwardDest[2] == 0 && row.ForwardDest[3] == 0 && row.ForwardMask[0] == 0 && row.ForwardMask[1] == 0 && row.ForwardMask[2] == 0 && row.ForwardMask[3] == 0 {
			if int(row.ForwardMetric1) < minM || minM == 0 {
				minM = int(row.ForwardMetric1)
				index = int(row.ForwardIfIndex)
			}
		}
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		if iface.Index == index {
			return &iface, nil
		}
	}
	return nil, fmt.Errorf("no default interface found")
}

func getDefaultGateway(ifIndex int) (string, int, error) {
	table, err := gowindows.GetIpForwardTable()
	if err != nil {
		return "", 0, err
	}
	for _, row := range table {
		if row.ForwardIfIndex == gowindows.DWord(ifIndex) && row.ForwardDest[0] == 0 && row.ForwardDest[1] == 0 && row.ForwardDest[2] == 0 && row.ForwardDest[3] == 0 && row.ForwardMask[0] == 0 && row.ForwardMask[1] == 0 && row.ForwardMask[2] == 0 && row.ForwardMask[3] == 0 {
			return net.IP(row.ForwardNextHop[:]).String(), int(row.ForwardMetric1), nil
		}
	}
	return "", 0, fmt.Errorf("no default gateway found")
}
func RandIP() (string, string, string, error) {
	info, err := GetDefaultNetworkInfo()
	if err != nil {
		return "", "", "", err
	}
	ip, err := scanCIDR(info.Subnet)
	if err != nil {
		return "", "", "", err
	}
	ip2, ipNet, err := net.ParseCIDR(info.Subnet)
	if err != nil {
		return "", "", "", err
	}
	return ip.String(), ip2.String(), net.IP(ipNet.Mask).String(), nil
}

const (
	timeout        = 500 * time.Millisecond // 每个IP的超时时间
	maxWorkers     = 50                     // 并发数
	maxRetries     = 2                      // 每个IP的重试次数
	minSuccessRate = 0.5                    // 成功率阈值
)

type PingResult struct {
	IP        net.IP
	Reachable bool
	Latency   time.Duration
}

func pingIP(ip net.IP) (bool, time.Duration, error) {
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		return false, 0, err
	}

	pinger.SetPrivileged(true) // 需要root权限
	pinger.Timeout = timeout
	pinger.Count = 3 // 每个IP发送3个包
	pinger.Interval = 50 * time.Millisecond

	err = pinger.Run()
	if err != nil {
		return false, 0, err
	}

	stats := pinger.Statistics()
	successRate := float64(stats.PacketsRecv) / float64(stats.PacketsSent)

	return successRate >= minSuccessRate, stats.AvgRtt, nil
}

func generateIPsFromCIDR(cidr string) ([]net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		// 跳过网络地址和广播地址
		if !ip.Equal(ipnet.IP) && !isBroadcast(ip, ipnet.Mask) {
			ips = append(ips, net.ParseIP(ip.String()))
		}
	}
	return ips, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func isBroadcast(ip net.IP, mask net.IPMask) bool {
	for i := range ip {
		if ip[i]|^mask[i] != 255 {
			return false
		}
	}
	return true
}

func scanCIDR(cidr string) (net.IP, error) {
	ips, err := generateIPsFromCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %v", err)
	}

	results := make(chan PingResult, maxWorkers)
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	// 从后往前扫描
	for i := len(ips) - 1; i >= 0; i-- {
		ip := ips[i]
		sem <- struct{}{}
		wg.Add(1)

		go func(ip net.IP) {
			defer func() {
				<-sem
				wg.Done()
			}()

			for retry := 0; retry < maxRetries; retry++ {
				reachable, latency, err := pingIP(ip)
				if err != nil {
					continue
				}

				if reachable {
					results <- PingResult{
						IP:        ip,
						Reachable: true,
						Latency:   latency,
					}
					fmt.Printf("IP %s is reachable (latency: %v)\n", ip, latency)
					return
				}
			}

			results <- PingResult{
				IP:        ip,
				Reachable: false,
			}
		}(ip)

		// 检查是否有不可达的结果
		select {
		case result := <-results:
			if !result.Reachable {
				return result.IP, nil
			}
		default:
		}
	}

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 处理剩余结果
	for result := range results {
		if !result.Reachable {
			return result.IP, nil
		}
	}

	return nil, fmt.Errorf("all IPs in the CIDR range are reachable")
}
