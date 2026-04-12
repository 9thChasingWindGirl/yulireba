package node

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"playfast/internal/httpclient"
	"strings"

	"gopkg.in/yaml.v3"
)



type ClashProxy struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	UUID     string `yaml:"uuid"`
	Cipher   string `yaml:"cipher"`
	Method   string `yaml:"method"`
	Network  string `yaml:"network"`
}

type ClashConfig struct {
	Proxies []ClashProxy `yaml:"proxies"`
}

func GetFromSubscription(url string) ([]Proxy, error) {
	all, err := httpclient.GET(url)
	if err != nil {
		return nil, err
	}

	var proxies []Proxy

	decoded := tryDecodeBase64(string(all))

	if strings.Contains(decoded, "proxies:") || strings.Contains(decoded, "Proxy:") {
		proxies, err = parseClashYAML(decoded)
		if err == nil && len(proxies) > 0 {
			return proxies, nil
		}
	}

	if strings.Contains(decoded, "{") || strings.Contains(decoded, "[") {
		proxies, err = parseJSONProxies(decoded)
		if err == nil && len(proxies) > 0 {
			return proxies, nil
		}
	}

	lines := strings.Split(decoded, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ss://") {
			proxy, err := parseSSLink(line)
			if err == nil {
				proxies = append(proxies, proxy)
			}
		} else if strings.HasPrefix(line, "vmess://") {
			proxy, err := parseVMessLink(line)
			if err == nil {
				proxies = append(proxies, proxy)
			}
		} else if strings.HasPrefix(line, "vless://") {
			proxy, err := parseVLESSLink(line)
			if err == nil {
				proxies = append(proxies, proxy)
			}
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("无法解析订阅内容")
	}

	return proxies, nil
}

func tryDecodeBase64(content string) string {
	reader := bytes.NewReader([]byte(content))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "ss://") ||
			strings.HasPrefix(line, "vmess://") ||
			strings.HasPrefix(line, "vless://") ||
			strings.HasPrefix(line, "trojan://") {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(line)
		if err == nil {
			return string(decoded)
		}
		decoded, err = base64.URLEncoding.DecodeString(line)
		if err == nil {
			return string(decoded)
		}
		decoded, err = base64.RawStdEncoding.DecodeString(line)
		if err == nil {
			return string(decoded)
		}
	}
	return content
}

func parseClashYAML(content string) ([]Proxy, error) {
	var config ClashConfig
	err := yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, err
	}

	proxies := make([]Proxy, 0)
	for _, p := range config.Proxies {
		proxy := Proxy{
			Name: p.Name,
			Host: p.Server,
			Port: uint16(p.Port),
		}

		switch strings.ToLower(p.Type) {
		case "shadowsocks":
			proxy.Protocol = "shadowsocks"
			proxy.Password = p.Password
			proxy.Method = p.Cipher
			if proxy.Method == "" {
				proxy.Method = "aes-256-gcm"
			}
		case "vmess":
			proxy.Protocol = "vmess"
			proxy.Password = p.UUID
		case "vless":
			proxy.Protocol = "vless"
			proxy.Password = p.UUID
		case "trojan":
			proxy.Protocol = "trojan"
			proxy.Password = p.Password
		case "socks5", "socks":
			proxy.Protocol = "socks"
			proxy.Password = p.Password
		default:
			continue
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

func parseJSONProxies(content string) ([]Proxy, error) {
	var proxies []Proxy

	if strings.HasPrefix(strings.TrimSpace(content), "[") {
		err := json.Unmarshal([]byte(content), &proxies)
		if err != nil {
			return nil, err
		}
	} else {
		var config map[string]interface{}
		err := json.Unmarshal([]byte(content), &config)
		if err != nil {
			return nil, err
		}

		if outbound, ok := config["outbounds"].([]interface{}); ok {
			for _, ob := range outbound {
				obMap, ok := ob.(map[string]interface{})
				if !ok {
					continue
				}
				if obMap["type"] == "shadowsocks" || obMap["type"] == "vless" || obMap["type"] == "vmess" {
					proxy := Proxy{
						Name:     getString(obMap, "tag"),
						Protocol: getString(obMap, "type"),
					}

					if server, ok := obMap["server"].(string); ok {
						proxy.Host = server
					}
					if serverPort, ok := obMap["server_port"].(float64); ok {
						proxy.Port = uint16(serverPort)
					}

					if ssOpts, ok := obMap["shadowsocks"].(map[string]interface{}); ok {
						proxy.Password = getString(ssOpts, "password")
						proxy.Method = getString(ssOpts, "method")
					} else if vlessOpts, ok := obMap["vless"].(map[string]interface{}); ok {
						proxy.Password = getString(vlessOpts, "uuid")
					} else if vmessOpts, ok := obMap["vmess"].(map[string]interface{}); ok {
						proxy.Password = getString(vmessOpts, "uuid")
					}

					proxies = append(proxies, proxy)
				}
			}
		}
	}

	return proxies, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func parseSSLink(link string) (Proxy, error) {
	proxy := Proxy{Protocol: "shadowsocks"}

	withoutScheme := strings.TrimPrefix(link, "ss://")
	parts := strings.SplitN(withoutScheme, "#", 2)
	if len(parts) > 1 {
		proxy.Name, _ = urlDecode(parts[1])
	}

	userInfo := parts[0]
	atIndex := strings.Index(userInfo, "@")
	if atIndex == -1 {
		return proxy, fmt.Errorf("invalid ss link")
	}

	userPart := userInfo[:atIndex]
	serverPart := userInfo[atIndex+1:]

	decodedUser, err := base64.StdEncoding.DecodeString(userPart)
	if err != nil {
		decodedUser, _ = base64.URLEncoding.DecodeString(userPart)
	}
	decodedStr := string(decodedUser)

	colonIndex := strings.LastIndex(decodedStr, ":")
	if colonIndex == -1 {
		return proxy, fmt.Errorf("invalid user info")
	}

	proxy.Method = decodedStr[:colonIndex]
	proxy.Password = decodedStr[colonIndex+1:]

	serverColonIndex := strings.LastIndex(serverPart, ":")
	if serverColonIndex == -1 {
		return proxy, fmt.Errorf("invalid server info")
	}

	proxy.Host = serverPart[:serverColonIndex]
	var portStr string
	remaining := serverPart[serverColonIndex+1:]
	if idx := strings.Index(remaining, "?"); idx != -1 {
		portStr = remaining[:idx]
	} else {
		portStr = remaining
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)
	proxy.Port = uint16(port)

	if proxy.Name == "" {
		proxy.Name = fmt.Sprintf("SS-%s:%d", proxy.Host, proxy.Port)
	}

	return proxy, nil
}

func parseVMessLink(link string) (Proxy, error) {
	proxy := Proxy{Protocol: "vmess"}

	withoutScheme := strings.TrimPrefix(link, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(withoutScheme)
	if err != nil {
		decoded, _ = base64.URLEncoding.DecodeString(withoutScheme)
	}

	var vmess struct {
		Ver  string `json:"ver"`
		Name string `json:"ps"`
		Host string `json:"add"`
		Port string `json:"port"`
		UUID string `json:"id"`
	}

	json.Unmarshal(decoded, &vmess)

	proxy.Name = vmess.Name
	proxy.Host = vmess.Host
	proxy.Password = vmess.UUID

	var port int
	fmt.Sscanf(vmess.Port, "%d", &port)
	proxy.Port = uint16(port)

	if proxy.Name == "" {
		proxy.Name = fmt.Sprintf("VMess-%s:%d", proxy.Host, proxy.Port)
	}

	return proxy, nil
}

func parseVLESSLink(link string) (Proxy, error) {
	proxy := Proxy{Protocol: "vless"}

	withoutScheme := strings.TrimPrefix(link, "vless://")
	
	atIndex := strings.Index(withoutScheme, "@")
	if atIndex == -1 {
		return proxy, fmt.Errorf("invalid vless link")
	}

	proxy.Password = withoutScheme[:atIndex]
	serverPart := withoutScheme[atIndex+1:]

	serverColonIndex := strings.LastIndex(serverPart, ":")
	if serverColonIndex == -1 {
		return proxy, fmt.Errorf("invalid server info")
	}

	proxy.Host = serverPart[:serverColonIndex]
	remaining := serverPart[serverColonIndex+1:]

	queryIndex := strings.Index(remaining, "?")
	var portStr string
	if queryIndex != -1 {
		portStr = remaining[:queryIndex]
	} else {
		portStr = remaining
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)
	proxy.Port = uint16(port)

	proxy.Name = fmt.Sprintf("VLESS-%s:%d", proxy.Host, proxy.Port)

	return proxy, nil
}

func urlDecode(s string) (string, error) {
	s = strings.ReplaceAll(s, "%20", " ")
	s = strings.ReplaceAll(s, "%21", "!")
	s = strings.ReplaceAll(s, "%23", "#")
	s = strings.ReplaceAll(s, "%24", "$")
	s = strings.ReplaceAll(s, "%26", "&")
	s = strings.ReplaceAll(s, "%27", "'")
	s = strings.ReplaceAll(s, "%28", "(")
	s = strings.ReplaceAll(s, "%29", ")")
	s = strings.ReplaceAll(s, "%2B", "+")
	s = strings.ReplaceAll(s, "%2C", ",")
	s = strings.ReplaceAll(s, "%2F", "/")
	s = strings.ReplaceAll(s, "%3A", ":")
	s = strings.ReplaceAll(s, "%3B", ";")
	s = strings.ReplaceAll(s, "%3D", "=")
	s = strings.ReplaceAll(s, "%3F", "?")
	s = strings.ReplaceAll(s, "%40", "@")
	return s, nil
}

func GetFromLocalFile(content string) ([]Proxy, error) {
	var proxies []Proxy

	err := json.Unmarshal([]byte(content), &proxies)
	if err == nil && len(proxies) > 0 {
		return proxies, nil
	}

	proxies, err = parseClashYAML(content)
	if err == nil && len(proxies) > 0 {
		return proxies, nil
	}

	proxies, err = parseJSONProxies(content)
	if err == nil && len(proxies) > 0 {
		return proxies, nil
	}

	return nil, fmt.Errorf("无法解析文件内容")
}
