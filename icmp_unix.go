//go:build !windows

package main

import (
	"net"

	"golang.org/x/net/ipv4"
)

// listenIcmp 创建 ICMP 监听 socket（Unix 平台）
//
// Linux/macOS 的 raw socket 默认就能接收 ICMP 错误消息
// (Time Exceeded / Destination Unreachable), 无需额外设置。
func listenIcmp(dst net.IP) (*ipv4.PacketConn, error) {
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	return ipv4.NewPacketConn(conn), nil
}
