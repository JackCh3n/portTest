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
//
//	port-test -traceroute 10.0.0.1                     UDP traceroute
//	port-test -traceroute example.com -T -p 443        TCP traceroute 到 443
//	port-test -traceroute example.com -m 20 -w 2       最大 20 跳, 每跳超时 2 秒
//
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
		os.Exit(exitConnFailed)
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

// parseIcmpTypeCode 从 raw ICMP socket 读取的数据中提取 ICMP 类型和代码
// 跨平台差异: Windows raw socket 接收的数据包含 IPv4 头（数据从 IP 头开始），
// Linux/macOS 未设置 IP_HDRINCL 时数据从 ICMP 头开始。
// 通过检测首字节是否为 IPv4 version(4) 自动跳过 IP 头, 两个平台通用。
// 返回 (类型, 代码, 是否有效)
func parseIcmpTypeCode(buf []byte) (byte, byte, bool) {
	if len(buf) == 0 {
		return 0, 0, false
	}
	offset := 0
	if buf[0]>>4 == 4 {
		// IPv4 header: IHL(低4位) 表示 32 位字数, 最小 5 (20 字节)
		ihl := int(buf[0]&0x0f) * 4
		if ihl < 20 {
			return 0, 0, false
		}
		if ihl >= len(buf) {
			// 检测到 IP 头但数据不足（无 ICMP payload），丢弃
			return 0, 0, false
		}
		offset = ihl
	}
	if len(buf) <= offset+1 {
		return 0, 0, false
	}
	return buf[offset], buf[offset+1], true
}

// parseIcmpTriggerPort 从 ICMP 错误消息中提取触发包的目的端口
// 用于确认 ICMP 属于当前探测（防止无关/陈旧 ICMP 干扰路由显示）。
// 数据布局: [可选外层IP头] ICMP头(8字节) 原始IP头 原始TCP/UDP头
// TCP/UDP 头前 4 字节均为 源端口(2) + 目的端口(2), 故两种协议通用。
// 返回 (目的端口, 是否解析成功)
func parseIcmpTriggerPort(buf []byte) (int, bool) {
	off := 0
	if len(buf) > 0 && buf[0]>>4 == 4 {
		ihl := int(buf[0]&0x0f) * 4
		if ihl < 20 || len(buf) <= ihl {
			return 0, false
		}
		off = ihl
	}
	// ICMP 头 8 字节
	inner := off + 8
	if len(buf) < inner+20 || buf[inner]>>4 != 4 {
		return 0, false
	}
	ihl := int(buf[inner]&0x0f) * 4
	if ihl < 20 || len(buf) < inner+ihl+4 {
		return 0, false
	}
	tcp := inner + ihl
	return int(buf[tcp+2])<<8 | int(buf[tcp+3]), true
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

		// ICMP 类型: 11=Time Exceeded(中间跳), 3=Destination Unreachable,
		// 3且code=3=端口不可达(到达目标端口关闭), 0/8=Echo
		// 注意: Windows raw socket 数据含 IP 头, 需用 parseIcmpTypeCode 提取
		msgType, msgCode, ok := parseIcmpTypeCode(buf[:n])
		if !ok {
			continue
		}
		// 校验触发包目的端口 == 探测端口（防止无关 ICMP 干扰）
		if tp, tok := parseIcmpTriggerPort(buf[:n]); tok && tp != port {
			continue
		}
		src := cm.Src.String()

		switch {
		case msgType == 11:
			return src, false, time.Since(start)
		case msgType == 3 && msgCode == 3:
			return src, true, time.Since(start)
		case msgType == 3:
			return src, false, time.Since(start)
		case msgType == 0 || msgType == 8:
			return src, true, time.Since(start)
		}
	}
}

// sendTcpProbe 发送一个 TCP SYN 探测包（带指定 TTL）并等待响应
// TCP traceroute 原理: 通过 Dialer.Control 在 SYN 发出前设置 IP_TTL，
// 中间路由器 TTL 过期时返回 ICMP Time Exceeded（由共享 ICMP 监听器捕获），
// 目标收到 SYN 后返回 SYN-ACK（连接成功）或 RST（连接拒绝），均表示到达目标。
//
// 关键点（Linux 行为）: 内核收到 Time Exceeded 后, 除了投递给 raw socket,
// 还会把 EHOSTUNREACH 注入到进行中的 connect()。因此 connect 可能快速返回
// 该错误, 此时不能立即判定"超时"——必须继续等待 raw socket 的 ICMP 直到
// deadline, 否则中间路由器的 Time Exceeded 会被竞态丢掉, 显示为无响应。
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

	// 异步发起 TCP 连接（connect 返回前内核可能已把 ICMP 错误注入 socket）
	tcpCh := make(chan error, 1)
	go func() {
		conn, err := dialer.Dial("tcp", addr)
		if conn != nil {
			conn.Close()
		}
		tcpCh <- err
	}()

	// ICMP 消息
	type icmpMsg struct {
		ip      string
		msgType byte
		msgCode byte
		ok      bool
	}

	// 启动 ICMP reader: 读取一个包后退出。
	// 注意: 截止时间在启动 reader 前设置, 避免 reader 内部的
	// SetReadDeadline 与 stopReader 的唤醒互相覆盖造成竞态。
	startReader := func() (<-chan icmpMsg, <-chan struct{}) {
		ch := make(chan icmpMsg, 1)
		done := make(chan struct{})
		pc.SetReadDeadline(deadline)
		go func() {
			defer close(done)
			buf := make([]byte, 1500)
			n, cm, _, err := pc.ReadFrom(buf)
			if err != nil || n == 0 || cm == nil || cm.Src == nil {
				ch <- icmpMsg{}
				return
			}
			mt, mc, ok := parseIcmpTypeCode(buf[:n])
			if ok {
				// 校验触发包目的端口 == 探测端口（防止无关 ICMP 干扰）
				if tp, tok := parseIcmpTriggerPort(buf[:n]); tok && tp != port {
					ok = false
				}
			}
			ch <- icmpMsg{cm.Src.String(), mt, mc, ok}
		}()
		return ch, done
	}

	var (
		icmpCh     <-chan icmpMsg
		readerDone <-chan struct{}
	)
	icmpCh, readerDone = startReader()

	// 唤醒 ICMP reader 并等待退出（保证探测间串行, 不抢占下个探测的包）
	stopReader := func() {
		pc.SetReadDeadline(time.Now())
		<-readerDone
	}

	tcpDone := false // connect 已返回（成功/拒绝/超时/错误）
	for {
		var tcpSel <-chan error
		if !tcpDone {
			tcpSel = tcpCh
		}

		select {
		case msg := <-icmpCh:
			if msg.ok {
				switch {
				case msg.msgType == 11: // Time Exceeded — 中间路由器
					return msg.ip, false, time.Since(start)
				case msg.msgType == 3 && msg.msgCode == 3: // 端口不可达 — 到达目标
					return msg.ip, true, time.Since(start)
				case msg.msgType == 3: // 网络/主机不可达 — 视为中间跳
					return msg.ip, false, time.Since(start)
				case msg.msgType == 0 || msg.msgType == 8: // Echo
					return msg.ip, true, time.Since(start)
				}
			}
			// 无关 ICMP / 未知类型 / 读取失败: 重启 reader 继续等待
			if time.Now().After(deadline) {
				return "", false, time.Since(start)
			}
			icmpCh, readerDone = startReader()

		case err := <-tcpSel:
			tcpDone = true
			if err == nil {
				// 连接成功 — 到达目标，端口开放
				stopReader()
				return dst.String(), true, time.Since(start)
			}
			if isConnectionRefused(err) {
				// 连接被拒绝（RST）— 到达目标，端口关闭
				stopReader()
				return dst.String(), true, time.Since(start)
			}
			// 其他错误(EHOSTUNREACH/ETIMEDOUT 等, 常见于 TTL 过期):
			// 不立即返回, 继续等待 ICMP Time Exceeded 直到 deadline

		case <-time.After(time.Until(deadline)):
			stopReader()
			return "", false, time.Since(start)
		}
	}
}

// isConnectionRefused 判断错误是否为连接被拒绝（TCP RST）
// 跨平台:
//   - Linux: ECONNREFUSED=111
//   - macOS: ECONNREFUSED=61
//   - Windows: WSAECONNREFUSED=10061
//
// 注意: 不能只用 errors.Is(err, syscall.ECONNREFUSED)——Go 的 syscall 包在
// Windows 上把该常量定义为 POSIX 语义别名(数值 ≠ 10061), errors.Is 会失败。
// 需额外用 errors.As 提取 Errno 数值按平台匹配。
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if runtime.GOOS == "windows" && uintptr(errno) == 10061 {
			return true // WSAECONNREFUSED
		}
	}
	return false
}
