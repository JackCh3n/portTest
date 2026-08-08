package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// runDnsMode DNS 查询模式
// 用法:
//   port-test -dns example.com                    查询所有记录
//   port-test -dns example.com -type A            仅查询 A 记录
//   port-test -dns example.com -ip 8.8.8.8        指定 DNS 服务器
func runDnsMode(posArgs []string, dnsType, dnsServer string) {
	// 校验域名
	if len(posArgs) == 0 {
		fmt.Println("  错误: 请指定要查询的域名")
		fmt.Println("  用法: port-test -dns <domain> [-type <A|AAAA|MX|CNAME|TXT|NS>]")
		os.Exit(exitUsage)
	}
	domain := posArgs[0]

	// 构造 Resolver（支持自定义 DNS 服务器）
	resolver := &net.Resolver{}
	if dnsServer != "" {
		// 支持带端口写法 (8.8.8.8:5353 / [::1]:5353)，不带端口时默认 53
		dnsAddr := dnsServer
		if _, _, err := net.SplitHostPort(dnsServer); err != nil {
			dnsAddr = net.JoinHostPort(strings.Trim(dnsServer, "[]"), "53")
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "udp", dnsAddr)
			},
		}
	}

	// 确定查询类型（默认全部）
	queryAll := dnsType == ""
	if dnsType != "" {
		dnsType = strings.ToUpper(dnsType)
	}

	fmt.Printf("  域名: %s\n", domain)
	if dnsServer != "" {
		fmt.Printf("  服务器: %s\n", dnsServer)
	}
	fmt.Println("  ------------------------------------")

	ctx := context.Background()

	query := func(label string, fn func() ([]string, error)) {
		if !queryAll && dnsType != label {
			return
		}
		records, err := fn()
		if err != nil {
			fmt.Printf("  %-6s ❌ %v\n", label, err)
			return
		}
		if len(records) == 0 {
			fmt.Printf("  %-6s (无记录)\n", label)
			return
		}
		fmt.Printf("  %-6s:\n", label)
		for _, r := range records {
			fmt.Printf("    %s\n", r)
		}
	}

	// A 记录
	query("A", func() ([]string, error) {
		ips, err := resolver.LookupIPAddr(ctx, domain)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				out = append(out, ip.IP.String())
			}
		}
		return out, nil
	})

	// AAAA 记录
	query("AAAA", func() ([]string, error) {
		ips, err := resolver.LookupIPAddr(ctx, domain)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				out = append(out, ip.IP.String())
			}
		}
		return out, nil
	})

	// CNAME
	query("CNAME", func() ([]string, error) {
		cname, err := resolver.LookupCNAME(ctx, domain)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	})

	// MX 记录
	query("MX", func() ([]string, error) {
		records, err := resolver.LookupMX(ctx, domain)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, mx := range records {
			out = append(out, fmt.Sprintf("%-40s 优先级 %d", strings.TrimSuffix(mx.Host, "."), mx.Pref))
		}
		return out, nil
	})

	// TXT 记录
	query("TXT", func() ([]string, error) {
		return resolver.LookupTXT(ctx, domain)
	})

	// NS 记录
	query("NS", func() ([]string, error) {
		records, err := resolver.LookupNS(ctx, domain)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ns := range records {
			out = append(out, strings.TrimSuffix(ns.Host, "."))
		}
		return out, nil
	})
}
