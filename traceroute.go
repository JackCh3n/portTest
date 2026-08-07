package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/ipv4"
)

// runTracerouteMode 路由追踪模式
// 用法:
//   port-test -traceroute 10.0.0.1                     UDP traceroute
//   port-test -traceroute example.com -T -p 443        TCP traceroute 到 443
//   port-test -traceroute example.com -m 20 -w 2       最大 20 跳, 每跳超时 2 秒
// 注意: 需要管理员/root 权限（原始 socket 监听 ICMP），Windows 需以管理员运行
func runTracerouteMode(posArgs []string, tcpMode bool, port, maxHops, timeoutSec int) {
	// 校验目标
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定目标主机")
		fmt.Println("  用法: port-test -traceroute <host> [-T] [-p <port>] [-m <hops>] [-w <sec>]")
		os.Exit(1)
	}
	target := posArgs[0]

	// 解析目标 IP
	ips, err := net.LookupIP(target)
	if err != nil || len(ips) == 0 {
		fmt.Printf("  错误: 无法解析目标: %s\n", target)
		os.Exit(1)
	}
	dst := ips[0]
	if dst.To4() == nil {
		fmt.Println("  错误: 暂不支持 IPv6 traceroute")
		os.Exit(1)
	}

	// 参数默认值
	if maxHops <= 0 {
		maxHops = 30
	}
	if timeoutSec <= 0 {
		timeoutSec = 1
	}
	if port <= 0 {
		if tcpMode {
			port = 443 // TCP 模式默认 443
		} else {
			port = 33434 // UDP 模式起始探测端口（与 traceroute 默认一致）
		}
	}

	// 打印信息
	mode := "UDP"
	if tcpMode {
		mode = "TCP"
	}
	fmt.Printf("  目标: %s (%s)\n", target, dst)
	fmt.Printf("  模式: %s | 端口: %d | 最大跳数: %d | 超时: %ds\n", mode, port, maxHops, timeoutSec)
	fmt.Println("  ------------------------------------")

	// 监听 ICMP 响应（Time Exceeded / Port Unreachable）
	icmpConn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Printf("  错误: 无法监听 ICMP (需要管理员/root 权限): %v\n", err)
		os.Exit(1)
	}
	defer icmpConn.Close()
	icmpPC := ipv4.NewPacketConn(icmpConn)

	// 逐跳探测
	timeout := time.Duration(timeoutSec) * time.Second
	startTime := time.Now()

	for ttl := 1; ttl <= maxHops; ttl++ {
		// 本跳探测 3 次
		var (
			hopIP   string
			times   []time.Duration
			reached bool
		)

		for attempt := 0; attempt < 3; attempt++ {
			ip, done, rtt := sendProbe(icmpPC, dst, ttl, port, timeout)
			times = append(times, rtt)
			if ip != "" {
				hopIP = ip
			}
			if done {
				reached = true
				break
			}
		}

		// 输出本跳
		if hopIP == "" {
			fmt.Printf("  %2d  * * * (无响应)\n", ttl)
		} else {
			var rtts string
			for _, t := range times {
				rtts += fmt.Sprintf(" %v", t.Round(time.Millisecond))
			}
			arrow := ""
			if reached {
				arrow = " ← 到达目标"
			}
			fmt.Printf("  %2d  %-15s [%s]%s\n", ttl, hopIP, rtts, arrow)
		}

		if reached {
			break
		}
	}

	fmt.Println("  ------------------------------------")
	fmt.Printf("  总耗时: %v\n", time.Since(startTime).Round(time.Millisecond))
}

// sendProbe 发送一个 TTL 探测包并等待 ICMP 响应
// 返回: 响应来源 IP, 是否到达目标, 往返时间
func sendProbe(pc *ipv4.PacketConn, dst net.IP, ttl int, port int, timeout time.Duration) (string, bool, time.Duration) {
	start := time.Now()

	// UDP 探测端口：经典 traceroute 用递增端口，TCP 模式固定目标端口
	probePort := port
	if probePort < 33434 {
		probePort += rand.Intn(100)
	}

	// 通过 UDP socket 发送探测包（触发 ICMP Time Exceeded / Port Unreachable）
	conn, err := net.DialTimeout("udp", net.JoinHostPort(dst.String(), strconv.Itoa(probePort)), timeout)
	if err != nil {
		return "", false, time.Since(start)
	}
	defer conn.Close()

	// 使用 x/net/ipv4 的跨平台封装设置 IP TTL（避免 syscall.Handle 等平台差异）
	udpConn, ok := conn.(*net.UDPConn)
	if ok {
		ipv4Conn := ipv4.NewConn(udpConn)
		if err := ipv4Conn.SetTTL(ttl); err != nil {
			// TTL 设置失败不致命，继续发送
			_ = err
		}
	}

	// 发送探测数据
	probe := make([]byte, 40)
	if _, err := conn.Write(probe); err != nil {
		return "", false, time.Since(start)
	}

	// 等待 ICMP 响应
	buf := make([]byte, 1500)
	pc.SetReadDeadline(time.Now().Add(timeout))

	for {
		n, cm, _, err := pc.ReadFrom(buf)
		if err != nil {
			return "", false, time.Since(start)
		}
		if n == 0 || cm == nil || cm.Src == nil {
			continue
		}

		// ICMP 类型: 11=Time Exceeded(中间跳), 3=Destination Unreachable(到达目标), 0/8=Echo
		msgType := buf[0] & 0xff
		src := cm.Src.String()

		switch msgType {
		case 11:
			return src, false, time.Since(start)
		case 3:
			return src, true, time.Since(start)
		case 0, 8:
			return src, true, time.Since(start)
		}
	}
}

// tcpProbe TCP 探测：TCP 模式下补充验证目标端口是否真实开放
func tcpProbe(target string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
