package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// headerList 支持 -H 参数多次指定（自定义 flag.Value）
type headerList []string

func (h *headerList) String() string { return strings.Join(*h, "|") }

func (h *headerList) Set(val string) error {
	*h = append(*h, val)
	return nil
}

// runCurlMode 模拟 curl 发送 HTTP/HTTPS 请求并输出响应
// 支持的写法:
//   port-test -curl https://example.com                           GET 抓取
//   port-test -curl https://example.com -k                        忽略 HTTPS 证书
//   port-test -curl https://example.com -L                        跟随重定向
//   port-test -curl https://api.com/login -X POST -d 'a=1&b=2'    POST 请求
//   port-test -curl http://api.com/json -X POST -d '{"k":1}' -H 'Content-Type: application/json'
//   port-test -curl http://api.com -H 'Authorization: Bearer xxx' 自定义请求头
func runCurlMode(posArgs []string, method, data string, headers headerList, insecure, followRedirect bool, timeoutSec int) {
	// 校验 URL
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定目标 URL")
		fmt.Println("  用法: port-test -curl <url> [-X <method>] [-d <data>] [-H <header>] [-k]")
		os.Exit(exitUsage)
	}

	url := posArgs[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		fmt.Printf("  错误: 无效 URL (需以 http:// 或 https:// 开头): %s\n", url)
		os.Exit(exitUsage)
	}

	// 默认方法 GET，指定 -d 时默认 POST（与 curl 行为一致）
	if method == "" {
		if data != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	method = strings.ToUpper(method)

	// 构建请求
	var body io.Reader
	if data != "" {
		body = strings.NewReader(data)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fmt.Printf("  请求构造失败: %v\n", err)
		os.Exit(exitUsage)
	}

	// 设置请求头（-H 可多次指定）
	for _, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		idx := strings.Index(h, ":")
		if idx <= 0 {
			fmt.Printf("  警告: 忽略无效请求头: %q (需为 'Key: Value' 格式)\n", h)
			continue
		}
		key := strings.TrimSpace(h[:idx])
		val := strings.TrimSpace(h[idx+1:])
		req.Header.Set(key, val)
	}

	// POST + -d 且未显式指定 Content-Type 时，模仿 curl 自动添加
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	// 默认 User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "port-test/"+version)
	}

	// 输出请求摘要
	fmt.Printf("  请求: %s %s\n", method, url)
	fmt.Printf("  超时: %d 秒", timeoutSec)
	if insecure {
		fmt.Print(" | 已忽略 HTTPS 证书校验")
	}
	if followRedirect {
		fmt.Print(" | 跟随重定向")
	}
	fmt.Println()
	if data != "" {
		fmt.Printf("  数据: %s\n", data)
	}
	if len(headers) > 0 {
		fmt.Printf("  请求头: %s\n", strings.Join(headers, " | "))
	}
	fmt.Println("  ------------------------------------")

	// HTTP 客户端：支持忽略 HTTPS 证书、跟随重定向、读取环境代理 (HTTP_PROXY/HTTPS_PROXY/NO_PROXY)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Transport: transport,
	}
	if followRedirect {
		// 限制最大 10 次重定向，避免重定向循环
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数超过 10 次, 疑似重定向循环")
			}
			return nil
		}
	} else {
		// 默认不跟随重定向（与 curl 不带 -L 行为一致），手动打印 Location 提示
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  请求失败: %v\n", err)
		os.Exit(exitConnFailed)
	}
	defer resp.Body.Close()

	// 读取响应体（限制 10MB，防止超大响应耗尽内存）
	limitedBody := io.LimitReader(resp.Body, 10*1024*1024)
	respBody, err := io.ReadAll(limitedBody)
	if err != nil {
		fmt.Printf("  读取响应失败: %v\n", err)
		os.Exit(exitConnFailed)
	}

	elapsed := time.Since(startTime)

	// 输出响应状态
	fmt.Printf("  状态: %d %s (耗时 %v)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Round(time.Millisecond))
	if !followRedirect && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			fmt.Printf("  提示: 收到重定向 -> %s (加 -L 可自动跟随)\n", loc)
		}
	}
	fmt.Println("  响应头:")
	for k, v := range resp.Header {
		fmt.Printf("    %s: %s\n", k, strings.Join(v, ", "))
	}
	fmt.Println("  ------------------------------------")
	fmt.Println("  响应体:")
	fmt.Println(string(respBody))
}
