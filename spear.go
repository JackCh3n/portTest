package main

import (
	"context"
	"fmt"
	"net"
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

// SpearMode 矛模式 - TCPing 端口测试
func SpearMode(target string, ports []int, timeout time.Duration, count int, interval time.Duration) {
	if target == "" {
		fmt.Println("错误: 请使用 -ip 指定目标IP地址")
		fmt.Println("示例: port-test -mode spear -ip 192.168.1.1 -p 80,443,8080")
		return
	}

	fmt.Println("🗡️  矛模式 - TCP 端口连通性测试")
	fmt.Println("====================================")
	fmt.Printf("目标: %s\n", target)
	fmt.Printf("端口: %v\n", ports)
	fmt.Printf("超时: %v\n", timeout)
	if count > 1 {
		fmt.Printf("次数: %d (间隔 %v)\n", count, interval)
	}
	fmt.Println("====================================")

	if count <= 1 {
		// 单次扫描
		result := scanTarget(target, ports, timeout)
		printResult(result)
	} else {
		// 重复扫描
		continuousScan(target, ports, timeout, count, interval)
	}
}

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

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
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

	result.TotalTime = time.Since(startTime)
	return result
}

// pingPort 测试单个端口
func pingPort(target string, port int, timeout time.Duration) PortResult {
	addr := fmt.Sprintf("%s:%d", target, port)
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
		fmt.Printf("\n--- 第 %d/%d 次扫描 ---\n", i, count)
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
			fmt.Printf("  ✅ 端口 %-6d  [开放]  服务: %-8s  延迟: %v\n", r.Port, service, r.Latency)
		} else {
			fmt.Printf("  ❌ 端口 %-6d  [关闭]  %s\n", r.Port, r.Error)
		}
	}
	fmt.Println("------------------------------------")
	fmt.Printf("总计: %d | 开放: %d | 关闭: %d | 耗时: %v\n",
		result.Total, result.Success, result.Failed, result.TotalTime)
}

// QuickPing 快速检测（只输出关键信息，适合脚本调用）
func QuickPing(target string, ports []int, timeout time.Duration) (int, []PortResult) {
	results := make([]PortResult, 0, len(ports))
	var successCount int32

	var wg sync.WaitGroup
	resultCh := make(chan PortResult, len(ports))

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
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
		if err1 == nil && err2 == nil && start > 0 && end > start && end <= 65535 {
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
