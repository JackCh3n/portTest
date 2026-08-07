//go:build !windows

package main

import "syscall"

// setSocketTTL 在 Unix 平台设置 IP_TTL socket 选项
// 用于 TCP traceroute: 在 Dialer.Control 回调中调用，
// 在 TCP SYN 发出之前设置 TTL，使中间路由器返回 ICMP Time Exceeded
func setSocketTTL(fd uintptr, ttl int) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
}
