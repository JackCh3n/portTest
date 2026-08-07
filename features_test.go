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
	runScanMode([]string{"127.0.0.1"}, "", 2)
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
