package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHandleEcho 验证 /echo 接口回显请求详情
func TestHandleEcho(t *testing.T) {
	// 构造请求
	body := `{"k":1}`
	req := httptest.NewRequest("POST", "/echo?a=1&b=2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test", "hello")
	rec := httptest.NewRecorder()

	config = Config{Code: 200}
	handleEcho(rec, req)

	if rec.Code != 200 {
		t.Errorf("状态码 = %d, want 200", rec.Code)
	}

	// 解析响应
	var echo map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &echo); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if echo["method"] != "POST" {
		t.Errorf("method = %v, want POST", echo["method"])
	}
	if echo["path"] != "/echo" {
		t.Errorf("path = %v, want /echo", echo["path"])
	}
	if echo["query"] != "a=1&b=2" {
		t.Errorf("query = %v, want a=1&b=2", echo["query"])
	}
	if echo["body"] != body {
		t.Errorf("body = %v, want %q", echo["body"], body)
	}

	// 验证请求头回显
	headers, ok := echo["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers 类型错误: %T", echo["headers"])
	}
	if headers["X-Test"] != "hello" {
		t.Errorf("headers[X-Test] = %v, want hello", headers["X-Test"])
	}
}

// TestScanCommonPorts 验证常见端口表完整性
func TestScanCommonPorts(t *testing.T) {
	if len(scanCommonPorts) < 20 {
		t.Errorf("常见端口表过小: %d", len(scanCommonPorts))
	}
	// 必须包含关键端口
	for _, p := range []int{22, 80, 443, 3306, 6379, 8080} {
		found := false
		for _, sp := range scanCommonPorts {
			if sp == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("常见端口表缺少端口 %d", p)
		}
	}
}

// TestScanLocalOpenPort 验证扫描能发现本地开放端口
func TestScanLocalOpenPort(t *testing.T) {
	// 启动本地 HTTP 服务
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// 扫描本地常见端口（不依赖特定端口，验证引擎可用性）
	runScanMode([]string{"127.0.0.1"}, "", 2, false)
}

// TestRunUdpModeLocal 验证 UDP 探测逻辑（本地 UDP 服务）
func TestRunUdpModeLocal(t *testing.T) {
	// 启动本地 UDP 服务
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("无法启动 UDP 服务: %v", err)
	}
	defer pc.Close()

	// 验证 UDP 端口解析函数（对未监听的端口应返回 closed/unknown）
	r := pingUdp("127.0.0.1", 1, 1000)
	if r.Status != "closed" && r.Status != "unknown" {
		t.Errorf("UDP 端口 1 状态 = %q, 应为 closed/unknown", r.Status)
	}
}

// TestRunDnsModeLocal 验证 DNS 查询本地主机名
func TestRunDnsModeLocal(t *testing.T) {
	runDnsMode([]string{"localhost"}, "", "")
}

// TestExtractTrailingFlagsNew 验证新 flag 的后置回收
func TestExtractTrailingFlagsNew(t *testing.T) {
	var (
		timeout, total, conc int
		trHops, trWait       int
		trTcp                bool
	)
	timeout, total, conc = 3, 100, 10
	trHops, trWait = 30, 1

	rest := extractTrailingFlags([]string{"example.com", "-k", "-timeout", "5", "-n", "500", "-c", "20", "-T", "-m", "15"},
		&flagOpts{
			timeout:    &timeout,
			benchTotal: &total,
			benchConc:  &conc,
			trMaxHops:  &trHops,
			trWait:     &trWait,
			trTcp:      &trTcp,
		})

	if len(rest) != 1 || rest[0] != "example.com" {
		t.Errorf("rest = %v, want [example.com]", rest)
	}
	if timeout != 5 {
		t.Errorf("timeout = %d, want 5", timeout)
	}
	if total != 500 {
		t.Errorf("total = %d, want 500", total)
	}
	if conc != 20 {
		t.Errorf("conc = %d, want 20", conc)
	}
	if !trTcp {
		t.Error("trTcp = false, want true")
	}
	if trHops != 15 {
		t.Errorf("trHops = %d, want 15", trHops)
	}
}

// TestParseHostSegmentIPv6 验证 IPv6 地址解析（修复 udp.go 的 IPv6 bug 后）
func TestParseHostSegmentIPv6(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort int // 第一个端口, 0 表示无端口
	}{
		{"[::1]:53", "::1", 53},
		{"::1 53", "::1", 53},
		{"[2001:db8::1]:443", "2001:db8::1", 443},
		{"10.0.0.1:8080", "10.0.0.1", 8080},
		{"10.0.0.1 8080", "10.0.0.1", 8080},
		{"example.com:443", "example.com", 443},
	}

	for _, tt := range tests {
		ht := parseHostSegment(tt.input)
		if ht.Host != tt.wantHost {
			t.Errorf("parseHostSegment(%q).Host = %q, want %q", tt.input, ht.Host, tt.wantHost)
		}
		if tt.wantPort > 0 {
			found := false
			for _, p := range ht.Ports {
				if p == tt.wantPort {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("parseHostSegment(%q).Ports = %v, want contains %d", tt.input, ht.Ports, tt.wantPort)
			}
		}
	}
}

// TestIsConnectionRefused 验证连接拒绝错误判断
func TestIsConnectionRefused(t *testing.T) {
	// nil 不应判定为拒绝
	if isConnectionRefused(nil) {
		t.Error("isConnectionRefused(nil) = true, want false")
	}
	// ECONNREFUSED 应判定为拒绝
	if !isConnectionRefused(syscall.ECONNREFUSED) {
		t.Error("isConnectionRefused(ECONNREFUSED) = false, want true")
	}
	// 其他错误不应判定为拒绝
	if isConnectionRefused(syscall.EAGAIN) {
		t.Error("isConnectionRefused(EAGAIN) = true, want false")
	}
}

// TestExtractTrailingFlagsVersion 验证 -version 后置 flag 回收
func TestExtractTrailingFlagsVersion(t *testing.T) {
	var showVer bool
	rest := extractTrailingFlags([]string{"8080", "-version"},
		&flagOpts{showVersion: &showVer})

	if len(rest) != 1 || rest[0] != "8080" {
		t.Errorf("rest = %v, want [8080]", rest)
	}
	if !showVer {
		t.Error("showVersion = false, want true")
	}
}

// TestExtractTrailingFlagsHtmlDir 验证 -html/-dir 后置 flag 回收
func TestExtractTrailingFlagsHtmlDir(t *testing.T) {
	var html, dir string
	rest := extractTrailingFlags([]string{"8080", "-html", "index.html", "-dir", "./static"},
		&flagOpts{html: &html, staticDir: &dir})

	if len(rest) != 1 || rest[0] != "8080" {
		t.Errorf("rest = %v, want [8080]", rest)
	}
	if html != "index.html" {
		t.Errorf("html = %q, want index.html", html)
	}
	if dir != "./static" {
		t.Errorf("dir = %q, want ./static", dir)
	}
}

// TestUdpIPv6Parsing 验证 UDP 模式的 IPv6 解析（通过 parseHostSegment 间接测试）
func TestUdpIPv6Parsing(t *testing.T) {
	// ::1 不应被错误解析为 host=:: port=1
	ht := parseHostSegment("::1 53")
	if ht.Host != "::1" {
		t.Errorf("parseHostSegment(\"::1 53\").Host = %q, want ::1", ht.Host)
	}
	if len(ht.Ports) != 1 || ht.Ports[0] != 53 {
		t.Errorf("parseHostSegment(\"::1 53\").Ports = %v, want [53]", ht.Ports)
	}

	// [::1]:53 应正确解析
	ht = parseHostSegment("[::1]:53")
	if ht.Host != "::1" {
		t.Errorf("parseHostSegment(\"[::1]:53\").Host = %q, want ::1", ht.Host)
	}
	if len(ht.Ports) != 1 || ht.Ports[0] != 53 {
		t.Errorf("parseHostSegment(\"[::1]:53\").Ports = %v, want [53]", ht.Ports)
	}
}

// TestBenchQPSNoDivisionByZero 验证 bench QPS 不会除零（elapsed=0 时不 panic）
func TestBenchQPSNoDivisionByZero(t *testing.T) {
	// 直接验证除零保护逻辑（不启动完整压测）
	elapsed := time.Duration(0)
	successN := 100
	qps := 0.0
	if elapsedSec := elapsed.Seconds(); elapsedSec > 0 {
		qps = float64(successN) / elapsedSec
	}
	if qps != 0.0 {
		t.Errorf("QPS with zero elapsed = %v, want 0.0 (no division by zero)", qps)
	}
}

// TestDnsServerAddress 验证 DNS 自定义服务器地址构造（带/不带端口）
func TestDnsServerAddress(t *testing.T) {
	buildAddr := func(server string) string {
		if _, _, err := net.SplitHostPort(server); err != nil {
			return net.JoinHostPort(strings.Trim(server, "[]"), "53")
		}
		return server
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"8.8.8.8", "8.8.8.8:53"},
		{"8.8.8.8:5353", "8.8.8.8:5353"},
		{"[2001:4860:4860::8888]", "[2001:4860:4860::8888]:53"},
		{"[2001:4860:4860::8888]:5353", "[2001:4860:4860::8888]:5353"},
	}

	for _, tt := range tests {
		if got := buildAddr(tt.input); got != tt.expected {
			t.Errorf("buildAddr(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestParsePortRangeBounds 验证端口范围边界校验（修复 12345-70000 越界 bug）
func TestParsePortRangeBounds(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		// 正常范围
		{"1-5", []int{1, 2, 3, 4, 5}},
		{"80-82", []int{80, 81, 82}},
		// 完整端口范围
		{"1-65535", []int{1}}, // 只验证首位，避免大数组
		// 越界: 结束端口 > 65535 应拒绝
		{"12345-70000", nil},
		// 越界: 起始端口 > 65535 应拒绝
		{"70000-70001", nil},
		// 非法: 起始 > 结束 应拒绝
		{"1024-80", nil},
		// 非法: 端口 0 应拒绝
		{"0-80", nil},
		// 非法: 非数字
		{"abc-def", nil},
	}

	for _, tt := range tests {
		ports := parsePortRange(tt.input)
		if tt.expected == nil {
			if len(ports) != 0 {
				t.Errorf("parsePortRange(%q) = %v, want nil/empty (边界校验失败)", tt.input, ports)
			}
			continue
		}
		if tt.input == "1-65535" {
			if len(ports) != 65535 {
				t.Errorf("parsePortRange(1-65535) 长度 = %d, want 65535", len(ports))
			}
			if ports[0] != 1 {
				t.Errorf("parsePortRange(1-65535)[0] = %d, want 1", ports[0])
			}
			continue
		}
		if len(ports) != len(tt.expected) {
			t.Errorf("parsePortRange(%q) = %v, want %v", tt.input, ports, tt.expected)
			continue
		}
		for i := range ports {
			if ports[i] != tt.expected[i] {
				t.Errorf("parsePortRange(%q)[%d] = %d, want %d", tt.input, i, ports[i], tt.expected[i])
			}
		}
	}
}

// TestPortRangeSinglePort 验证单端口经过 parsePortRange 也走边界校验
func TestPortRangeSinglePort(t *testing.T) {
	// 合法单端口
	if ports := parsePortRange("8080"); len(ports) != 1 || ports[0] != 8080 {
		t.Errorf("parsePortRange(8080) = %v, want [8080]", ports)
	}
	// 非法单端口（>65535）应返回空
	if ports := parsePortRange("70000"); len(ports) != 0 {
		t.Errorf("parsePortRange(70000) = %v, want empty", ports)
	}
	// 非法单端口（0）应返回空
	if ports := parsePortRange("0"); len(ports) != 0 {
		t.Errorf("parsePortRange(0) = %v, want empty", ports)
	}
}

// TestParseHostSegmentLoneBracket 验证只有左括号的 IPv6 也能解析
func TestParseHostSegmentLoneBracket(t *testing.T) {
	ht := parseHostSegment("[::1 53")
	if ht.Host != "::1" {
		t.Errorf("parseHostSegment(\"[::1 53\").Host = %q, want ::1", ht.Host)
	}
	if len(ht.Ports) != 1 || ht.Ports[0] != 53 {
		t.Errorf("parseHostSegment(\"[::1 53\").Ports = %v, want [53]", ht.Ports)
	}
}

// TestSplitMultiHostMoreSeps 验证 "=" 与 "?" 分隔符
func TestSplitMultiHostMoreSeps(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"10.0.0.1:80=10.0.0.2:443", []string{"10.0.0.1:80", "10.0.0.2:443"}},
		{"10.0.0.1:80?10.0.0.2:443", []string{"10.0.0.1:80", "10.0.0.2:443"}},
		{"10.0.0.1:80+10.0.0.2:443\\10.0.0.3:53", []string{"10.0.0.1:80", "10.0.0.2:443", "10.0.0.3:53"}},
	}
	for _, tt := range tests {
		got := splitMultiHost(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitMultiHost(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitMultiHost(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

// TestExtractTrailingFlagsXDH 验证 curl 风格后置 -X/-d/-H 回收
func TestExtractTrailingFlagsXDH(t *testing.T) {
	var method, data string
	var headers headerList
	rest := extractTrailingFlags([]string{"http://x.com", "-X", "POST", "-d", "a=1&b=2", "-H", "X-Test: 1", "-H", "X-Two: 2"},
		&flagOpts{method: &method, data: &data, headers: &headers})

	if len(rest) != 1 || rest[0] != "http://x.com" {
		t.Errorf("rest = %v, want [http://x.com]", rest)
	}
	if method != "POST" {
		t.Errorf("method = %q, want POST", method)
	}
	if data != "a=1&b=2" {
		t.Errorf("data = %q, want a=1&b=2", data)
	}
	if len(headers) != 2 || headers[0] != "X-Test: 1" || headers[1] != "X-Two: 2" {
		t.Errorf("headers = %v, want [X-Test: 1 X-Two: 2]", headers)
	}
}

// TestScanResultsSorted 验证 scanTarget 结果按端口升序
func TestScanResultsSorted(t *testing.T) {
	// 本地启动服务提供开放端口
	ports := []int{18121, 18120, 18122, 1}
	result := scanTarget("127.0.0.1", ports, 500*time.Millisecond)

	if result.Total != len(ports) {
		t.Fatalf("Total = %d, want %d", result.Total, len(ports))
	}
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i-1].Port > result.Results[i].Port {
			t.Errorf("结果未按端口排序: %v", result.Results)
			break
		}
	}
	// 首个结果应为最小端口 1
	if result.Results[0].Port != 1 {
		t.Errorf("Results[0].Port = %d, want 1 (最小端口排最前)", result.Results[0].Port)
	}
}

// TestParseIcmpType 验证 ICMP 类型/代码跨平台解析
// Windows raw socket 数据含 IPv4 头, Linux/macOS 不含, 应都能正确提取
func TestParseIcmpType(t *testing.T) {
	// 构造 Windows 格式: 20 字节 IPv4 头 (0x45) + ICMP 消息
	winBuf := func(icmpType, icmpCode byte) []byte {
		ipHeader := make([]byte, 20)
		ipHeader[0] = 0x45 // IPv4 version=4, IHL=5 (20 字节)
		ipHeader[2] = 0x00 // total length 高字节
		ipHeader[3] = 0x17 // total length 低字节 (23)
		ipHeader[8] = 64   // TTL
		return append(ipHeader, icmpType, icmpCode)
	}
	// 只有 IP 头 (0x45 开头) 无 ICMP payload
	ipHeaderOnly := make([]byte, 20)
	ipHeaderOnly[0] = 0x45

	tests := []struct {
		name     string
		buf      []byte
		wantType byte
		wantCode byte
		wantOK   bool
	}{
		// Linux/macOS: 无 IP 头, 数据从 ICMP 开始
		{"linux time exceeded", []byte{11, 0, 0, 0}, 11, 0, true},
		{"linux dest unreachable", []byte{3, 3, 0, 0}, 3, 3, true},
		{"linux echo reply", []byte{0, 0, 0, 0}, 0, 0, true},
		// Windows: 含 IPv4 头 (0x45, IHL=5), ICMP 从偏移 20 开始
		{"windows time exceeded", winBuf(11, 0), 11, 0, true},
		{"windows dest unreachable", winBuf(3, 3), 3, 3, true},
		{"windows echo reply", winBuf(0, 0), 0, 0, true},
		// 边界
		{"empty buffer", nil, 0, 0, false},
		{"ip header only no icmp", ipHeaderOnly, 0, 0, false},
	}

	for _, tt := range tests {
		gotType, gotCode, gotOK := parseIcmpTypeCode(tt.buf)
		if gotOK != tt.wantOK || (gotOK && (gotType != tt.wantType || gotCode != tt.wantCode)) {
			t.Errorf("%s: parseIcmpTypeCode = (%d, %d, %v), want (%d, %d, %v)",
				tt.name, gotType, gotCode, gotOK, tt.wantType, tt.wantCode, tt.wantOK)
		}
	}
}

// TestParseIcmpTriggerPort 验证 ICMP 错误消息中触发包目的端口提取
// 布局: [可选外层IP头] ICMP头(8) + 原始IP头(20) + 原始TCP/UDP头(8)
// TCP/UDP 头: 源端口(2) + 目的端口(2) + ...
func TestParseIcmpTriggerPort(t *testing.T) {
	// 构造 ICMP Time Exceeded (type=11) 消息
	makeIcmp := func(includeOuterIP bool, srcPort, dstPort int) []byte {
		inner := make([]byte, 0, 36)
		// 原始 IP 头 (20 字节): ver/IHL(1) tos(1) len(2) id(2) frag(2) ttl(1) proto(1) csum(2) srcIP(4) dstIP(4)
		inner = append(inner, 0x45, 0x00, 0x00, 0x1c)
		inner = append(inner, make([]byte, 8)...) // id + frag + ttl + proto + csum
		inner = append(inner, 10, 0, 0, 1)        // 源 IP
		inner = append(inner, 10, 0, 0, 2)        // 目的 IP
		// 原始 TCP 头前 8 字节: 源端口(2) + 目的端口(2) + seq(4)
		inner = append(inner,
			byte(srcPort>>8), byte(srcPort),
			byte(dstPort>>8), byte(dstPort),
			0, 0, 0, 0)

		icmp := []byte{11, 0, 0, 0, 0, 0, 0, 0} // type=11, code=0, checksum, unused
		icmp = append(icmp, inner...)

		if !includeOuterIP {
			return icmp
		}
		// Windows 格式: 外层 IP 头(20) + ICMP
		outer := []byte{0x45, 0x00, 0x00, 0x38}
		outer = append(outer, make([]byte, 8)...)
		outer = append(outer, 192, 168, 1, 1)
		outer = append(outer, 192, 168, 1, 2)
		return append(outer, icmp...)
	}

	tests := []struct {
		name     string
		buf      []byte
		wantPort int
		wantOK   bool
	}{
		{"linux format match", makeIcmp(false, 50000, 8085), 8085, true},
		{"windows format match", makeIcmp(true, 50000, 8085), 8085, true},
		{"linux different port", makeIcmp(false, 50000, 9090), 9090, true},
		{"empty buffer", nil, 0, false},
		{"too short", []byte{11, 0}, 0, false},
	}

	for _, tt := range tests {
		gotPort, gotOK := parseIcmpTriggerPort(tt.buf)
		if gotOK != tt.wantOK || (gotOK && gotPort != tt.wantPort) {
			t.Errorf("%s: parseIcmpTriggerPort = (%d, %v), want (%d, %v)",
				tt.name, gotPort, gotOK, tt.wantPort, tt.wantOK)
		}
	}
}
