//go:build windows

package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/net/ipv4"
)

const (
	sioRCVALL        = 0x98000001
	rcvallIPLevel    = 3 // Vista+: 接收 IP 层数据（含 ICMP 错误消息）
	winIPProtoICMP   = 1 // IANA: ICMP 协议号 (syscall 包未导出该常量)
	winIPProtoUDP    = 17
	winSocketRaw     = 3 // SOCK_RAW (syscall 包未导出)
	winAFINET        = 2 // AF_INET (syscall 包未导出)
)

// listenIcmp 创建 ICMP 监听 socket（Windows 专用）
//
// Windows Vista 及以后, raw socket 默认只投递 ICMP Echo 报文,
// Time Exceeded / Destination Unreachable 等 ICMP 错误消息
// 被系统直接丢弃, 不会到达 raw socket。
// 要接收这些错误消息( traceroute 核心依赖), 必须:
//   1. bind 到具体接口 IP（不能是 INADDR_ANY）
//   2. 设置 SIO_RCVALL (RCVALL_IPLEVEL) 禁用包过滤
//
// 注意: 需要管理员权限运行。
func listenIcmp(dst net.IP) (*ipv4.PacketConn, error) {
	// 创建 raw ICMP socket
	fd, err := syscall.Socket(winAFINET, winSocketRaw, winIPProtoICMP)
	if err != nil {
		return nil, err
	}

	// 获取到目标的本地出口 IP（Time Exceeded 会发往该地址）
	localIP := localEgressIP(dst)
	var addr [4]byte
	if v4 := localIP.To4(); v4 != nil {
		copy(addr[:], v4)
	}

	// bind 到具体接口 IP（SIO_RCVALL 的前置要求）
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: addr}); err != nil {
		syscall.Closesocket(fd)
		return nil, fmt.Errorf("bind 到 %s 失败: %w", localIP, err)
	}

	// 设置 SIO_RCVALL (RCVALL_IPLEVEL) — 禁用包过滤, 接收 ICMP 错误消息
	var mode int32 = rcvallIPLevel
	var bytesRet uint32
	if err := syscall.WSAIoctl(fd, sioRCVALL,
		(*byte)(unsafe.Pointer(&mode)), uint32(unsafe.Sizeof(mode)),
		nil, 0, &bytesRet, nil, 0); err != nil {
		syscall.Closesocket(fd)
		return nil, fmt.Errorf("SIO_RCVALL 设置失败: %w", err)
	}

	// 包装为 net.PacketConn（fd 所有权移交 os.File）
	f := os.NewFile(uintptr(fd), "icmp-raw")
	if f == nil {
		syscall.Closesocket(fd)
		return nil, fmt.Errorf("os.NewFile 失败")
	}
	conn, err := net.FilePacketConn(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	return ipv4.NewPacketConn(conn), nil
}

// localEgressIP 获取到目标 IP 的本地出口 IP（UDP 路由探测）
func localEgressIP(dst net.IP) net.IP {
	conn, err := net.Dial("udp", net.JoinHostPort(dst.String(), "33434"))
	if err != nil {
		return net.IPv4zero
	}
	defer conn.Close()
	if la, ok := conn.LocalAddr().(*net.UDPAddr); ok && la.IP != nil {
		return la.IP
	}
	return net.IPv4zero
}
