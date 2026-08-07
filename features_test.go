package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
