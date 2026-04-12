package http_client

import (
	"io"
	"net/http"
	"sync"
	"time"
)

var directClient *http.Client
var lock sync.Mutex

func clientDirect() *http.Client {
	lock.Lock()
	defer lock.Unlock()
	if directClient != nil {
		return directClient
	}
	directClient = &http.Client{
		Timeout: time.Second * 30,
	}
	return directClient
}

func get(client *http.Client, url string) ([]byte, error) {
	// 创建一个新的 GET 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// 添加自定义请求头
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}
func GET(url string) ([]byte, error) {
	bytes, err := get(clientDirect(), url)
	return bytes, err
}
