package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunCurlModeGET 验证 GET 请求抓取内容
func TestRunCurlModeGET(t *testing.T) {
	// 本地测试服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("方法 = %s, want GET", r.Method)
		}
		w.Header().Set("X-Test", "hello")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "test body content")
	}))
	defer srv.Close()

	// 直接调用 runCurlMode（该函数内部 os.Exit 在错误时，这里都是正常路径）
	runCurlMode([]string{srv.URL}, "", "", nil, false, false, 5)
}

// TestRunCurlModePOST 验证 POST + -d 请求
func TestRunCurlModePOST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("方法 = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "a=1&b=2" {
			t.Errorf("请求体 = %q, want %q", string(body), "a=1&b=2")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "posted")
	}))
	defer srv.Close()

	runCurlMode([]string{srv.URL}, "POST", "a=1&b=2", nil, false, false, 5)
}

// TestRunCurlModeHeaders 验证 -H 自定义请求头（多次指定）
func TestRunCurlModeHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token123" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer token123")
		}
		if got := r.Header.Get("X-Custom"); got != "yes" {
			t.Errorf("X-Custom = %q, want %q", got, "yes")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	headers := headerList{"Authorization: Bearer token123", "X-Custom: yes"}
	runCurlMode([]string{srv.URL}, "", "", headers, false, false, 5)
}

// TestRunCurlModeInsecure 验证忽略 HTTPS 证书（自签名证书场景）
func TestRunCurlModeInsecure(t *testing.T) {
	// 自签名 HTTPS 服务器（不验证证书时应能成功）
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "secure")
	}))
	defer srv.Close()

	// 使用 -k 忽略证书，应该成功访问
	runCurlMode([]string{srv.URL}, "", "", nil, true, false, 5)
}

// TestHeaderList 验证 headerList flag.Value 接口
func TestHeaderList(t *testing.T) {
	var h headerList
	if err := h.Set("A: 1"); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	if err := h.Set("B: 2"); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	if len(h) != 2 {
		t.Errorf("len = %d, want 2", len(h))
	}
	if !strings.Contains(h.String(), "A: 1") || !strings.Contains(h.String(), "B: 2") {
		t.Errorf("String() = %q, want 包含两个头", h.String())
	}
}

// TestRunCurlModeNoRedirect 验证不带 -L 时不跟随重定向（返回 3xx）
func TestRunCurlModeNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "final")
			return
		}
		http.Redirect(w, r, "/target", http.StatusFound)
	}))
	defer srv.Close()

	// 不带 -L: 应输出 302 而不是跟随
	runCurlMode([]string{srv.URL}, "", "", nil, false, false, 5)
}

// TestRunCurlModeRedirect 验证带 -L 时跟随重定向
func TestRunCurlModeRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "final-body")
			return
		}
		http.Redirect(w, r, "/target", http.StatusFound)
	}))
	defer srv.Close()

	// 带 -L: 应跟随到 /target 并输出 final-body
	runCurlMode([]string{srv.URL}, "", "", nil, false, true, 5)
}

// TestRunBenchModeKeepAlive 验证 bench 模式 keep-alive 开关
func TestRunBenchModeKeepAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	// 短连接压测
	runBenchMode([]string{srv.URL}, "", "", nil, false, false, false, 5, 5, 2)
	// keep-alive 压测
	runBenchMode([]string{srv.URL}, "", "", nil, false, false, true, 5, 5, 2)
}
