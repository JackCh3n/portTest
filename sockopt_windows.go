//go:build windows

package main

import "syscall"

// Windows syscall 包未导出 IPPROTO_IP 和 IP_TTL 常量，使用 IETF 标准值
const (
	winIPProtoIP = 0
	winIPTTL     = 4
)

// setSocketTTL 在 Windows 平台设置 IP_TTL socket 选项
// 用于 TCP traceroute: 在 Dialer.Control 回调中调用，
// 在 TCP SYN 发出之前设置 TTL，使中间路由器返回 ICMP Time Exceeded
func setSocketTTL(fd uintptr, ttl int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), winIPProtoIP, winIPTTL, ttl)
}
