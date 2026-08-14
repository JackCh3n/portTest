package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PortResult 端口扫描结果
type PortResult struct {
	Port    int           `json:"port"`
	Status  string        `json:"status"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
}

// ScanResult 扫描结果汇总
type ScanResult struct {
	Target    string        `json:"target"`
	Results   []PortResult  `json:"results"`
	Total     int           `json:"total"`
	Success   int           `json:"success"`
	Failed    int           `json:"failed"`
	TotalTime time.Duration `json:"total_time"`
}

// HostTarget 多主机目标
type HostTarget struct {
	Host  string
	Ports []int
}

// MultiTcpingMode 多主机 TCPing 测试
func MultiTcpingMode(targets []HostTarget, timeout time.Duration, count int, interval time.Duration, jsonOut bool) {
	if jsonOut {
		// JSON 输出模式为单次快照, 不重复 -count/-interval, 避免静默忽略造成误解
		if count > 1 {
			fmt.Println("  警告: JSON 输出模式为单次探测, -count/-interval 不生效")
		}
		// JSON 输出模式：收集所有主机结果一次性输出
		type targetResult struct {
			Target  string       `json:"target"`
			Results []PortResult `json:"results"`
			Total   int          `json:"total"`
			Success int          `json:"success"`
			Failed  int          `json:"failed"`
			Time    string       `json:"time"`
		}
		all := make([]targetResult, 0, len(targets))
		for _, t := range targets {
			result := scanTarget(t.Host, t.Ports, timeout)
			all = append(all, targetResult{
				Target:  result.Target,
				Results: result.Results,
				Total:   result.Total,
				Success: result.Success,
				Failed:  result.Failed,
				Time:    result.TotalTime.Round(time.Millisecond).String(),
			})
		}
		data, err := json.MarshalIndent(all, "", "  ")
		if err != nil {
			fmt.Printf("  JSON 序列化失败: %v\n", err)
			os.Exit(exitInterrupted)
		}
		fmt.Println(string(data))
		return
	}

	for i, t := range targets {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("  🗡️  目标 %d: %s\n", i+1, t.Host)
		fmt.Printf("  端口: %v\n", t.Ports)
		fmt.Printf("  超时: %v\n", timeout)
		if count > 1 {
			fmt.Printf("  次数: %d (间隔 %v)\n", count, interval)
		}
		fmt.Println("  ------------------------------------")

		if count <= 1 {
			result := scanTarget(t.Host, t.Ports, timeout)
			printResult(result)
		} else {
			continuousScan(t.Host, t.Ports, timeout, count, interval)
		}
	}
}

// TcpingMode TCPing 模式 - TCP 端口连通性测试 (单主机, 向后兼容)
func TcpingMode(target string, ports []int, timeout time.Duration, count int, interval time.Duration) {
	MultiTcpingMode([]HostTarget{{Host: target, Ports: ports}}, timeout, count, interval, false)
}

// maxScanConcurrency 扫描最大并发数，防止全端口扫描时一次性开 65535 个
// goroutine 导致文件描述符耗尽（每并发一个 socket）
const maxScanConcurrency = 1024

// scanTarget 对目标执行单次端口扫描
func scanTarget(target string, ports []int, timeout time.Duration) ScanResult {
	result := ScanResult{
		Target:  target,
		Results: make([]PortResult, 0, len(ports)),
		Total:   len(ports),
	}

	startTime := time.Now()
	var wg sync.WaitGroup
	resultCh := make(chan PortResult, len(ports))

	// 信号量限制并发: 最多同时 maxScanConcurrency 个探测
	sem := make(chan struct{}, maxScanConcurrency)

	for _, port := range ports {
		wg.Add(1)
		sem <- struct{}{} // 获取令牌（满时阻塞）
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌
			resultCh <- pingPort(target, p, timeout)
		}(port)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	for r := range resultCh {
		result.Results = append(result.Results, r)
		if r.Status == "open" {
			result.Success++
		} else {
			result.Failed++
		}
	}

	// 按端口排序, 保证文本/JSON 输出顺序稳定（并发探测的完成顺序是不确定的）
	sort.Slice(result.Results, func(i, j int) bool {
		return result.Results[i].Port < result.Results[j].Port
	})

	result.TotalTime = time.Since(startTime)
	return result
}

// resolveHost 校验目标主机可解析: IP 直接通过, 域名做一次 DNS 查询。
// 用于扫描前快速失败, 避免全端口范围扫描时输出大量重复的 DNS 错误。
func resolveHost(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}
	_, err := net.LookupIP(host)
	return err
}

// pingPort 测试单个端口
func pingPort(target string, port int, timeout time.Duration) PortResult {
	// IPv6 地址需要用 [] 包裹
	addr := net.JoinHostPort(target, strconv.Itoa(port))
	result := PortResult{
		Port: port,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	latency := time.Since(startTime)

	if err != nil {
		result.Status = "closed"
		result.Error = err.Error()
		result.Latency = latency
		return result
	}

	result.Status = "open"
	result.Latency = latency
	conn.Close()
	return result
}

// 常见服务端口映射
var commonServices = map[int]string{
	21:   "FTP",
	22:   "SSH",
	23:   "Telnet",
	25:   "SMTP",
	53:   "DNS",
	80:   "HTTP",
	110:  "POP3",
	143:  "IMAP",
	443:  "HTTPS",
	465:  "SMTPS",
	587:  "SMTP",
	993:  "IMAPS",
	995:  "POP3S",
	1080: "SOCKS",
	1433: "MSSQL",
	1521: "Oracle",
	3306: "MySQL",
	3389: "RDP",
	5432: "PostgreSQL",
	5900: "VNC",
	6379: "Redis",
	8080: "HTTP-Alt",
	8443: "HTTPS-Alt",
	9090: "HTTP-Alt",
	9200: "Elasticsearch",
	11211: "Memcached",
	27017: "MongoDB",
}

// getServiceName 获取端口对应的服务名称
func getServiceName(port int) string {
	if name, ok := commonServices[port]; ok {
		return name
	}
	return "Unknown"
}

// continuousScan 持续扫描
func continuousScan(target string, ports []int, timeout time.Duration, count int, interval time.Duration) {
	for i := 1; i <= count; i++ {
		fmt.Printf("\n  --- 第 %d/%d 次扫描 ---\n", i, count)
		result := scanTarget(target, ports, timeout)
		printResult(result)

		if i < count {
			time.Sleep(interval)
		}
	}
}

// printResult 打印扫描结果
func printResult(result ScanResult) {
	for _, r := range result.Results {
		service := getServiceName(r.Port)
		if r.Status == "open" {
			fmt.Printf("    ✅ 端口 %-6d  [开放]  服务: %-8s  延迟: %v\n", r.Port, service, r.Latency)
		} else {
			fmt.Printf("    ❌ 端口 %-6d  [关闭]  %s\n", r.Port, r.Error)
		}
	}
	fmt.Println("  ------------------------------------")
	fmt.Printf("  总计: %d | 开放: %d | 关闭: %d | 耗时: %v\n",
		result.Total, result.Success, result.Failed, result.TotalTime)
}

// QuickPing 快速检测（只输出关键信息，适合脚本调用）
func QuickPing(target string, ports []int, timeout time.Duration) (int, []PortResult) {
	results := make([]PortResult, 0, len(ports))
	var successCount int32

	var wg sync.WaitGroup
	resultCh := make(chan PortResult, len(ports))

	// 信号量限制并发（同 scanTarget）
	sem := make(chan struct{}, maxScanConcurrency)

	for _, port := range ports {
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()
			r := pingPort(target, p, timeout)
			resultCh <- r
		}(port)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for r := range resultCh {
		results = append(results, r)
		if r.Status == "open" {
			atomic.AddInt32(&successCount, 1)
		}
	}

	// 按端口排序, 保证输出顺序稳定
	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	return int(atomic.LoadInt32(&successCount)), results
}

// 解析端口范围，支持 "80,443" 和 "1-1024" 格式
func parsePortRange(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// 检查是否为范围格式
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		// 完整校验: 起止端口均在 1-65535 内，且 start <= end
		if err1 == nil && err2 == nil && start >= 1 && start <= 65535 && end >= start && end <= 65535 {
			ports := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				ports = append(ports, i)
			}
			return ports
		}
	}

	// 解析逗号分隔的端口列表
	ports, err := parsePorts(s)
	if err != nil {
		return nil
	}
	return ports
}
