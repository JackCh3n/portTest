package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
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
		os.Exit(exitUsage)
	}
	target := posArgs[0]

	// 解析目标 IP: 优先选择 IPv4 地址（当前仅支持 IPv4 traceroute）
	ips, err := net.LookupIP(target)
	if err != nil || len(ips) == 0 {
		fmt.Printf("  错误: 无法解析目标: %s\n", target)
		os.Exit(exitUsage)
	}
	var dst net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			dst = ip
			break
		}
	}
	if dst == nil {
		fmt.Println("  错误: 暂不支持 IPv6 traceroute (目标仅有 AAAA 记录)")
		os.Exit(exitUsage)
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
	// Windows 需设置 SIO_RCVALL + bind 出口 IP 才能收到 ICMP 错误消息
	icmpPC, err := listenIcmp(dst)
	if err != nil {
		fmt.Printf("  错误: 无法监听 ICMP (需要管理员/root 权限): %v\n", err)
		fmt.Println("  提示: Windows 请以管理员运行; 若仍失败请检查防火墙是否放行 ICMP 入站")
		os.Exit(exitInterrupted)
	}
	defer icmpPC.Close()

	// 逐跳探测
	timeout := time.Duration(timeoutSec) * time.Second
	startTime := time.Now()
	noResponseHops := 0 // 无响应跳数(用于 Windows 限制提示)

	for ttl := 1; ttl <= maxHops; ttl++ {
		// 本跳探测 3 次
		var (
			hopIP   string
			times   []time.Duration
			reached bool
		)

		for attempt := 0; attempt < 3; attempt++ {
			var ip string
			var done bool
			var rtt time.Duration

			if tcpMode {
				ip, done, rtt = sendTcpProbe(icmpPC, dst, ttl, port, timeout)
			} else {
				// 经典 traceroute 行为: 每个 TTL 使用递增探测端口
				// 便于区分 ICMP 响应来源（UDP 模式默认起始 33434）
				probePort := port + (ttl-1)*3 + attempt
				if probePort > 65535 {
					probePort = 33434 + (probePort % 32000)
				}
				ip, done, rtt = sendProbe(icmpPC, dst, ttl, probePort, timeout)
			}

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
			noResponseHops++
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

	// Windows 限制提示: raw socket 收不到 ICMP 错误消息, 中间跳无法显示
	if runtime.GOOS == "windows" && noResponseHops > 0 {
		fmt.Println()
		fmt.Println("  提示: Windows 系统过滤了 ICMP Time Exceeded 消息, 中间路由器无法显示。")
		fmt.Println("  完整路由建议: ① 在 Linux/macOS 上运行本工具 ② 或 Windows 安装 Npcap 后使用 tracetcp")
	}
}

// parseIcmpType 从 raw ICMP socket 读取的数据中提取 ICMP 类型
// 跨平台差异: Windows raw socket 接收的数据包含 IPv4 头（数据从 IP 头开始），
// Linux/macOS 未设置 IP_HDRINCL 时数据从 ICMP 头开始。
// 通过检测首字节是否为 IPv4 version(4) 自动跳过 IP 头, 两个平台通用。
func parseIcmpType(buf []byte) (byte, bool) {
	if len(buf) == 0 {
		return 0, false
	}
	offset := 0
	if buf[0]>>4 == 4 {
		// IPv4 header: IHL(低4位) 表示 32 位字数, 最小 5 (20 字节)
		ihl := int(buf[0]&0x0f) * 4
		if ihl >= 20 && ihl < len(buf) {
			offset = ihl
		} else if ihl >= 20 {
			// 检测到 IP 头但数据不足（无 ICMP payload），丢弃
			return 0, false
		}
	}
	return buf[offset], true
}

// sendProbe 发送一个 UDP TTL 探测包并等待 ICMP 响应
// 返回: 响应来源 IP, 是否到达目标, 往返时间
func sendProbe(pc *ipv4.PacketConn, dst net.IP, ttl int, port int, timeout time.Duration) (string, bool, time.Duration) {
	start := time.Now()

	// 通过 UDP socket 发送探测包（触发 ICMP Time Exceeded / Port Unreachable）
	conn, err := net.DialTimeout("udp", net.JoinHostPort(dst.String(), strconv.Itoa(port)), timeout)
	if err != nil {
		return "", false, time.Since(start)
	}
	defer conn.Close()

	// 使用 x/net/ipv4 的跨平台封装设置 IP TTL
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
		// 注意: Windows raw socket 数据含 IP 头, 需用 parseIcmpType 提取
		msgType, ok := parseIcmpType(buf)
		if !ok {
			continue
		}
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

// sendTcpProbe 发送一个 TCP SYN 探测包（带指定 TTL）并等待响应
// TCP traceroute 原理: 通过 Dialer.Control 在 SYN 发出前设置 IP_TTL，
// 中间路由器 TTL 过期时返回 ICMP Time Exceeded（由共享 ICMP 监听器捕获），
// 目标收到 SYN 后返回 SYN-ACK（连接成功）或 RST（连接拒绝），均表示到达目标。
// 返回: 响应来源 IP, 是否到达目标, 往返时间
func sendTcpProbe(pc *ipv4.PacketConn, dst net.IP, ttl, port int, timeout time.Duration) (string, bool, time.Duration) {
	start := time.Now()
	deadline := start.Add(timeout)
	addr := net.JoinHostPort(dst.String(), strconv.Itoa(port))

	// TCP dialer: 在 Control 回调中设置 TTL（在 SYN 发出前生效）
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(network, address string, c syscall.RawConn) error {
			// 尽力设置 TTL，失败时警告（TTL 不生效则中间跳永远不会显示）
			var serr error
			if err := c.Control(func(fd uintptr) {
				serr = setSocketTTL(fd, ttl)
			}); err != nil {
				return err
			}
			if serr != nil {
				fmt.Printf("  警告: TTL 设置失败 (ttl=%d): %v\n", ttl, serr)
			}
			return nil
		},
	}

	// 异步发起 TCP 连接
	tcpCh := make(chan error, 1)
	go func() {
		conn, err := dialer.Dial("tcp", addr)
		if conn != nil {
			conn.Close()
		}
		tcpCh <- err
	}()

	// ICMP 消息结构
	type icmpMsg struct {
		ip      string
		msgType byte
		ok      bool
	}

	for {
		// 单次 ICMP 读取: 每次探测只启动一个 reader,
		// 结束前必须唤醒并等待其退出, 避免旧 reader 抢走下次探测的 ICMP 响应
		icmpCh := make(chan icmpMsg, 1)
		readerDone := make(chan struct{})
		go func() {
			defer close(readerDone)
			buf := make([]byte, 1500)
			pc.SetReadDeadline(deadline)
			n, cm, _, err := pc.ReadFrom(buf)
			if err != nil || n == 0 || cm == nil || cm.Src == nil {
				icmpCh <- icmpMsg{}
				return
			}
			mt, ok := parseIcmpType(buf)
			icmpCh <- icmpMsg{cm.Src.String(), mt, ok}
		}()

		// 唤醒 ICMP reader 并等待退出（保证探测间串行，不抢占下个探测的包）
		stopReader := func() {
			pc.SetReadDeadline(time.Now())
			<-readerDone
		}

		select {
		case msg := <-icmpCh:
			if msg.ok {
				switch msg.msgType {
				case 11: // Time Exceeded — 中间路由器
					stopReader()
					return msg.ip, false, time.Since(start)
				case 3: // Destination Unreachable — 到达目标（或不可达）
					stopReader()
					return msg.ip, true, time.Since(start)
				}
			}
			// 无关 ICMP 包或读取失败: 唤醒 reader 后重新读取
			stopReader()

		case err := <-tcpCh:
			// TCP 结果已到, 唤醒 ICMP reader 后返回
			stopReader()
			if err == nil {
				// 连接成功 — 到达目标，端口开放
				return dst.String(), true, time.Since(start)
			}
			if isConnectionRefused(err) {
				// 连接被拒绝（RST）— 到达目标，端口关闭
				return dst.String(), true, time.Since(start)
			}
			// 超时或其他错误 — 无法确认
			return "", false, time.Since(start)

		case <-time.After(time.Until(deadline)):
			stopReader()
			return "", false, time.Since(start)
		}
	}
}

// isConnectionRefused 判断错误是否为连接被拒绝（TCP RST）
// 跨平台: Linux ECONNREFUSED=111, macOS=61, Windows WSAECONNREFUSED=1226
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED)
}
