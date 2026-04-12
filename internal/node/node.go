package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"playfast/internal/api"
	"playfast/internal/echo"
	"playfast/internal/httpclient"
	"time"

	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	slog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type Proxy struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"`
}

func Get() []Proxy {
	data := make([]Proxy, 0)
	all, err := httpclient.GET(fmt.Sprintf("%s/proxy.json", api.GetApiDomain()))
	if err != nil {
		return data
	}
	_ = json.Unmarshal(all, &data)
	return data
}
func GetOutbound(proxy string) (*option.Outbound, string, error) {
	data := Get()
	for i, p := range data {
		if p.Name != proxy {
			continue
		}
		var out option.Outbound
		switch p.Protocol {
		case "shadowsocks":
			out = option.Outbound{
				Type: constant.TypeShadowsocks,
				Tag:  "proxy",
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     p.Host,
						ServerPort: p.Port,
					},
					Method:   p.Method,
					Password: p.Password,
					UDPOverTCP: &option.UDPOverTCPOptions{
						Enabled: true,
						Version: 2,
					},
				},
			}
		case "vless":
			out = option.Outbound{
				Type: constant.TypeVLESS,
				Tag:  "proxy",
				Options: &option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     p.Host,
						ServerPort: p.Port,
					},
					UUID:                        p.Password,
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{},
					Multiplex: &option.OutboundMultiplexOptions{
						Enabled:        true,
						Protocol:       "h2mux",
						MaxConnections: 8,
						MinStreams:     16,
						Padding:        false,
					},
				},
			}
		case "socks":
			out = option.Outbound{
				Type: constant.TypeSOCKS,
				Tag:  "proxy",
				Options: &option.SOCKSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     p.Host,
						ServerPort: p.Port,
					},
					Version:  "5",
					Username: "yulireba",
					Password: p.Password,
					UDPOverTCP: &option.UDPOverTCPOptions{
						Enabled: true,
						Version: 2,
					},
				},
			}
		default:
			continue
		}
		registryOut := include.OutboundRegistry()
		createOutbound, err := registryOut.CreateOutbound(context.Background(), nil, slog.StdLogger(), out.Type, out.Type, out.Options)
		if err != nil {
			continue
		}
		client := echo.NewClient("1.1.1.1:80", echo.WithTimeout(3*time.Second), echo.WithDialer(createOutbound.DialContext))
		err = client.Connect(context.Background())
		if err != nil {
			continue
		}
		var ms int64
		result := client.Test(context.Background(), []byte("GET / HTTP/1.1\r\nHost: 1.1.1.1\r\nAccept: *\r\n\r\n\r\n"))
		ms = result.Latency.Milliseconds()
		log.Println(fmt.Sprintf("节点选择:ID:%d 节点:%s 延迟=%dms\n", i, p.Name, ms))
		if ms <= 0 {
			return nil, "", errors.New("节点超时")
		}
		return &out, p.Host, nil
	}
	return nil, "", errors.New("not fount Outbound")
}
