package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// runBenchMode 轻量 HTTP 压测模式
// 用法:
//   port-test -bench https://example.com                 100 请求, 10 并发
//   port-test -bench https://example.com -n 1000 -c 50   1000 请求, 50 并发
//   port-test -bench http://api.com/login -X POST -d 'a=1'   POST 压测
//   port-test -bench https://x.com -k -timeout 10        忽略证书 + 超时
//   port-test -bench https://x.com -L                    跟随重定向
//   port-test -bench https://x.com -ka                   复用连接 (keep-alive)
func runBenchMode(posArgs []string, method, data string, headers headerList, insecure, followRedirect, keepAlive bool, timeoutSec, total, concurrency int) {
	// 校验 URL
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定压测 URL")
		fmt.Println("  用法: port-test -bench <url> [-n <请求数>] [-c <并发>] [-X <method>] [-d <data>]")
		os.Exit(exitUsage)
	}
	url := posArgs[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		fmt.Printf("  错误: 无效 URL (需以 http:// 或 https:// 开头): %s\n", url)
		os.Exit(exitUsage)
	}

	// 参数校验
	if total <= 0 {
		fmt.Println("  警告: 无效请求数, 使用默认 100")
		total = 100
	}
	if concurrency <= 0 {
		fmt.Println("  警告: 无效并发数, 使用默认 10")
		concurrency = 10
	}
	if concurrency > total {
		fmt.Printf("  提示: 并发数 %d 超过请求数, 调整为 %d\n", concurrency, total)
		concurrency = total
	}

	// 默认方法 GET，指定 -d 时默认 POST
	if method == "" {
		if data != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	method = strings.ToUpper(method)

	// 提前校验请求模板（URL/方法非法时在并发启动前报错, 避免 worker goroutine 内 os.Exit）
	if _, err := http.NewRequest(method, url, nil); err != nil {
		fmt.Printf("  请求构造失败: %v\n", err)
		os.Exit(exitUsage)
	}

	// 构建请求模板
	newReq := func() *http.Request {
		var body io.Reader
		if data != "" {
			body = strings.NewReader(data)
		}
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			// 模板已在启动前校验通过, 此处不应失败; 返回 nil 由调用方跳过
			return nil
		}
		for _, h := range headers {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			idx := strings.Index(h, ":")
			if idx <= 0 {
				continue
			}
			req.Header.Set(strings.TrimSpace(h[:idx]), strings.TrimSpace(h[idx+1:]))
		}
		if body != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "port-test/"+version)
		}
		return req
	}

	// HTTP 客户端（支持忽略证书、跟随重定向、keep-alive 开关、读取环境代理）
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if !keepAlive {
		// 关闭连接复用: 每次请求后断开, 模拟真实场景的短连接压测
		transport.DisableKeepAlives = true
	}
	client := &http.Client{
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Transport: transport,
	}
	if !followRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// 打印压测信息
	fmt.Printf("  目标: %s %s\n", method, url)
	fmt.Printf("  请求数: %d | 并发: %d", total, concurrency)
	if data != "" {
		fmt.Printf(" | 数据: %s", data)
	}
	if insecure {
		fmt.Print(" | 忽略证书")
	}
	if followRedirect {
		fmt.Print(" | 跟随重定向")
	}
	if keepAlive {
		fmt.Print(" | keep-alive")
	} else {
		fmt.Print(" | 短连接")
	}
	fmt.Println()
	fmt.Println("  ------------------------------------")

	// 并发压测
	var (
		wg        sync.WaitGroup
		success   int64
		failed    int64
		mu        sync.Mutex
		latencies []time.Duration
	)
	startTime := time.Now()

	// 任务队列
	jobs := make(chan int, total)
	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				req := newReq()
				if req == nil {
					atomic.AddInt64(&failed, 1)
					continue
				}
				reqStart := time.Now()
				resp, err := client.Do(req)
				latency := time.Since(reqStart)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					atomic.AddInt64(&success, 1)
				} else {
					atomic.AddInt64(&failed, 1)
				}
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(startTime)

	// 统计结果
	successN := int(atomic.LoadInt64(&success))
	failedN := int(atomic.LoadInt64(&failed))

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	avgLatency := time.Duration(0)
	if len(latencies) > 0 {
		avgLatency = totalLatency / time.Duration(len(latencies))
	}

	percentile := func(p float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)-1) * p / 100.0)
		return latencies[idx]
	}

	qps := 0.0
	if elapsedSec := elapsed.Seconds(); elapsedSec > 0 {
		qps = float64(successN) / elapsedSec
	}

	fmt.Println("  压测结果:")
	fmt.Printf("    总请求: %d\n", total)
	fmt.Printf("    成功:   %d\n", successN)
	fmt.Printf("    失败:   %d\n", failedN)
	fmt.Printf("    耗时:   %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("    QPS:    %.1f\n", qps)
	if len(latencies) > 0 {
		fmt.Printf("    平均延迟: %v\n", avgLatency.Round(time.Microsecond))
		fmt.Printf("    P50:      %v\n", percentile(50).Round(time.Microsecond))
		fmt.Printf("    P90:      %v\n", percentile(90).Round(time.Microsecond))
		fmt.Printf("    P99:      %v\n", percentile(99).Round(time.Microsecond))
	}
}
