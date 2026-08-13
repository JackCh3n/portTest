package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// 常见端口扫描表（无 -p 参数时扫描这些端口）
var scanCommonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 135, 139, 143,
	443, 445, 465, 587, 993, 995, 1080, 1433, 1521, 3306,
	3389, 5432, 5900, 6379, 7001, 8000, 8080, 8443, 8888, 9090,
	9200, 11211, 27017,
}

// runScanMode 快速端口扫描模式
// 用法:
//   port-test -scan 10.0.0.1                扫描常见端口
//   port-test -scan 10.0.0.1 -p 1-1024      扫描指定范围
//   port-test -scan 10.0.0.1 -p 80,443,8080 扫描指定端口
//   port-test -scan 10.0.0.1 -p 1-65535     全端口扫描
//   port-test -scan 10.0.0.1 -timeout 2     自定义超时
//   port-test -scan 10.0.0.1 -json-out      JSON 输出
func runScanMode(posArgs []string, flagPorts string, timeoutSec int, jsonOut bool) {
	// 校验主机
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定要扫描的主机")
		fmt.Println("  用法: port-test -scan <host> [-p <ports/range>] [-timeout <sec>]")
		os.Exit(exitUsage)
	}
	host := strings.TrimSpace(posArgs[0])
	if host == "" {
		fmt.Println("  错误: 主机地址不能为空")
		os.Exit(exitUsage)
	}
	// 提前解析主机, 避免全端口扫描时输出大量重复的 DNS 错误
	if err := resolveHost(host); err != nil {
		fmt.Printf("  错误: 无法解析主机 %s: %v\n", host, err)
		os.Exit(exitConnFailed)
	}

	// 确定端口列表：-p 优先 + 位置参数补充
	// 支持 "port-test -scan 10.0.0.1 -p 80 443" 空格分隔端口
	var ports []int
	if flagPorts != "" {
		ports = parsePortRange(flagPorts)
		if len(ports) == 0 {
			fmt.Printf("  错误: 无效端口: %s\n", flagPorts)
			os.Exit(exitUsage)
		}
	} else {
		ports = append([]int(nil), scanCommonPorts...)
	}
	// 位置参数第 2 个起视为额外端口
	for _, p := range posArgs[1:] {
		ports = append(ports, parsePortRange(p)...)
	}
	ports = dedupePorts(ports)
	sort.Ints(ports)

	timeout := time.Duration(timeoutSec) * time.Second

	// 复用 tcping 扫描引擎
	result := scanTarget(host, ports, timeout)

	// JSON 输出模式
	if jsonOut {
		type scanJSON struct {
			Target  string       `json:"target"`
			Results []PortResult `json:"results"`
			Total   int          `json:"total"`
			Open    int          `json:"open"`
			Time    string       `json:"time"`
		}
		openCount := 0
		for _, r := range result.Results {
			if r.Status == "open" {
				openCount++
			}
		}
		data, err := json.MarshalIndent(scanJSON{
			Target:  result.Target,
			Results: result.Results,
			Total:   result.Total,
			Open:    openCount,
			Time:    result.TotalTime.Round(time.Millisecond).String(),
		}, "", "  ")
		if err != nil {
			fmt.Printf("  JSON 序列化失败: %v\n", err)
			os.Exit(exitInterrupted)
		}
		fmt.Println(string(data))
		return
	}

	// 打印扫描信息
	fmt.Printf("  目标: %s\n", host)
	fmt.Printf("  端口数: %d", len(ports))
	if flagPorts == "" {
		fmt.Print(" (常见端口表)")
	}
	fmt.Printf("\n  超时: %v\n", timeout)
	fmt.Println("  ------------------------------------")

	// 输出开放端口
	fmt.Println("  开放端口:")
	openCount := 0
	for _, r := range result.Results {
		if r.Status == "open" {
			openCount++
			fmt.Printf("    ✅ %-6d %-12s 延迟 %v\n", r.Port, getServiceName(r.Port), r.Latency.Round(time.Millisecond))
		}
	}
	if openCount == 0 {
		fmt.Println("    (未发现开放端口)")
	}
	fmt.Println("  ------------------------------------")
	fmt.Printf("  已扫描: %d | 开放: %d | 关闭: %d | 耗时: %v\n",
		result.Total, openCount, result.Total-openCount, result.TotalTime.Round(time.Millisecond))
}
