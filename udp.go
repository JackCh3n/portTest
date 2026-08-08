package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// runUdpMode UDP 连通性测试模式
// UDP 无连接，检测原理：向目标发送 UDP 包并等待回包。
// 若收到 ICMP Port Unreachable -> 端口关闭；收到回包 -> 端口开放；超时 -> 无法确认。
// 用法:
//   port-test -udp 10.0.0.1:53         测试 UDP 53 (DNS)
//   port-test -udp 10.0.0.1 53,123     多端口
//   port-test -udp 10.0.0.1:53 -timeout 5
func runUdpMode(posArgs []string, flagPorts string, timeoutSec int) {
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定目标地址")
		fmt.Println("  用法: port-test -udp <host:port> [-timeout <sec>]")
		os.Exit(exitUsage)
	}

	// 复用 parseHostSegment 统一解析逻辑（正确处理 IPv6/IPv4/hostname + 端口）
	var host string
	var ports []int

	if len(posArgs) > 0 {
		// 合并位置参数为一段，用 parseHostSegment 解析
		joined := strings.Join(posArgs, " ")
		ht := parseHostSegment(joined)
		host = ht.Host
		ports = ht.Ports
	}

	// -p flag 补充（总是合并，支持 "port-test -udp 10.0.0.1 -p 53 123" 空格分隔）
	if flagPorts != "" {
		ports = append(ports, parsePortRange(flagPorts)...)
	}
	ports = dedupePorts(ports)

	if len(ports) == 0 {
		fmt.Printf("  错误: 主机 %s 未指定端口\n", host)
		fmt.Println("  用法: port-test -udp <host:port> [port2 ...]")
		os.Exit(exitUsage)
	}

	timeout := time.Duration(timeoutSec) * time.Second
	fmt.Printf("  目标: %s\n", host)
	fmt.Printf("  端口: %v\n", ports)
	fmt.Printf("  超时: %v\n", timeout)
	fmt.Println("  ------------------------------------")

	// 逐端口测试
	for _, port := range ports {
		result := pingUdp(host, port, timeout)
		fmt.Printf("    %s 端口 %-6d [%s]  %s\n",
			statusIcon(result), port, result.Status, result.Error)
	}
}

type udpResult struct {
	Status string
	Error  string
}

func statusIcon(r udpResult) string {
	switch r.Status {
	case "open":
		return "✅"
	case "closed":
		return "❌"
	default:
		return "⚠️"
	}
}

// pingUdp 向目标发送 UDP 包并等待响应
func pingUdp(target string, port int, timeout time.Duration) udpResult {
	addr := net.JoinHostPort(target, strconv.Itoa(port))

	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return udpResult{Status: "closed", Error: err.Error()}
	}
	defer conn.Close()

	// 发送探测包（DNS 查询格式，可被 DNS 服务识别响应）
	// 其他 UDP 服务会忽略或返回 Port Unreachable
	probe := []byte{0xAB, 0xCD, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x07, 'v', 'e', 'r', 's', 'i', 'o', 'n', 0x04, 'b', 'i', 'n', 'd', 0x00, 0x00, 0x10, 0x00, 0x03}
	if _, err := conn.Write(probe); err != nil {
		return udpResult{Status: "closed", Error: err.Error()}
	}

	// 等待回包
	buf := make([]byte, 512)
	conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := conn.Read(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return udpResult{Status: "unknown", Error: "无响应 (UDP 无连接, 可能被防火墙丢弃或服务不回包)"}
		}
		// ICMP Port Unreachable 会反映为 read error
		return udpResult{Status: "closed", Error: "连接被拒绝 (ICMP Port Unreachable)"}
	}
	_ = n
	return udpResult{Status: "open", Error: "收到回包"}
}
