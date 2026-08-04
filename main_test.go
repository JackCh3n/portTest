package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		err      bool
	}{
		{"8080", []int{8080}, false},
		{"8080,9090", []int{8080, 9090}, false},
		{"8080, 9090, 3000", []int{8080, 9090, 3000}, false},
		{"abc", nil, true},
		{"0", nil, true},
		{"70000", nil, true},
	}

	for _, tt := range tests {
		ports, err := parsePorts(tt.input)
		if tt.err && err == nil {
			t.Errorf("parsePorts(%q) expected error, got none", tt.input)
		}
		if !tt.err && err != nil {
			t.Errorf("parsePorts(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.err && len(ports) != len(tt.expected) {
			t.Errorf("parsePorts(%q) = %v, want %v", tt.input, ports, tt.expected)
		}
	}
}

func TestServer(t *testing.T) {
	// 测试默认 JSON 响应
	config = Config{Code: 200}
	jsonContent = []byte(`{"code":200,"msg":"hello"}`)
	go startServer(18080)
	time.Sleep(100 * time.Millisecond)

	// 测试根路径
	resp, err := http.Get("http://localhost:18080/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result JSONResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("JSON 解析失败: %v, body: %s", err, string(body))
	}

	if result.Code != 200 || result.Msg != "hello" {
		t.Errorf("响应 = %v, want {Code:200, Msg:hello}", result)
	}
}

func TestHealthEndpoint(t *testing.T) {
	config = Config{Code: 200}
	jsonContent = []byte(`{"code":200,"msg":"hello"}`)
	go startServer(18081)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:18081/health")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("响应 = %q, want %q", string(body), "OK")
	}
}

func TestHTMLResponse(t *testing.T) {
	// 创建临时 HTML 文件
	tmpFile, err := os.CreateTemp("", "test*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	htmlContent := `<html><body>Test</body></html>`
	tmpFile.WriteString(htmlContent)
	tmpFile.Close()

	// 设置配置
	config = Config{
		Ports: []int{18082},
		Code:  200,
		HTML:  tmpFile.Name(),
	}
	htmlContentBytes, _ := os.ReadFile(tmpFile.Name())

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write(htmlContentBytes)
	})

	go http.ListenAndServe(":18082", mux)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:18082/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Test") {
		t.Errorf("HTML 响应不包含预期内容")
	}
}

func TestGetResponseType(t *testing.T) {
	// 默认 JSON
	config = Config{}
	jsonContent = []byte(`{"code":200,"msg":"hello"}`)
	htmlContent = nil
	if got := getResponseType(); got != "Default JSON" {
		t.Errorf("getResponseType() = %q, want %q", got, "Default JSON")
	}

	// 自定义 JSON
	config = Config{JSON: "test"}
	jsonContent = []byte(`{"code":200,"msg":"hello"}`)
	htmlContent = nil
	if got := getResponseType(); got != "Custom JSON" {
		t.Errorf("getResponseType() = %q, want %q", got, "Custom JSON")
	}

	// HTML
	config = Config{HTML: "test.html"}
	htmlContent = []byte("test")
	if got := getResponseType(); got != "HTML" {
		t.Errorf("getResponseType() = %q, want %q", got, "HTML")
	}
}
