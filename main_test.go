package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freePort 获取一个当前空闲的 TCP 端口, 避免固定端口被其他本地服务占用导致测试命中错误服务
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取空闲端口失败: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

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
	port := freePort(t)
	go startServer(port)
	time.Sleep(100 * time.Millisecond)

	// 测试根路径
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
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
	port := freePort(t)
	go startServer(port)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
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
		Ports: []int{freePort(t)},
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

	go http.ListenAndServe(fmt.Sprintf(":%d", config.Ports[0]), mux)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", config.Ports[0]))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Test") {
		t.Errorf("HTML 响应不包含预期内容")
	}
}

// TestServerTimeoutsForLargeDownloads 验证大文件下载场景的超时配置
// WriteTimeout 必须为 0: 它是请求头读完起算的绝对截止时间, 非零会中途掐断
// 超过时限的大文件下载(如 1-4GB 内网分发)
func TestServerTimeoutsForLargeDownloads(t *testing.T) {
	srv := newServer(freePort(t))
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, 应为 0 (非零会中断大文件下载)", srv.WriteTimeout)
	}
	if srv.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout 应保留, 用于防慢速请求占用连接")
	}
	if srv.IdleTimeout == 0 {
		t.Error("IdleTimeout 应保留, 用于回收空闲连接")
	}
}

// TestStaticFileDownload 验证 -dir 静态目录下载: 完整下载 + Range 断点请求
func TestStaticFileDownload(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("A"), 100*1024)
	if err := os.WriteFile(filepath.Join(dir, "test.bin"), content, 0644); err != nil {
		t.Fatal(err)
	}

	// 重建全局 config(清掉其他测试遗留的 HTML/JSON 字段), 测试后还原
	oldConfig, oldHTML, oldValid := config, htmlContent, staticDirValid
	config, htmlContent, staticDirValid = Config{StaticDir: dir}, nil, true
	defer func() { config, htmlContent, staticDirValid = oldConfig, oldHTML, oldValid }()

	port := freePort(t)
	go startServer(port)
	time.Sleep(100 * time.Millisecond)
	base := fmt.Sprintf("http://localhost:%d", port)

	// 完整下载, 内容必须一致
	resp, err := http.Get(base + "/static/test.bin")
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, want 200", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("下载内容不一致: got %d bytes, want %d", len(got), len(content))
	}

	// Range 断点请求: 206 + 精确分片
	req, err := http.NewRequest("GET", base+"/static/test.bin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-1023")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Range 请求失败: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 206 {
		t.Errorf("Range 状态码 = %d, want 206", resp2.StatusCode)
	}
	b2, _ := io.ReadAll(resp2.Body)
	if len(b2) != 1024 {
		t.Errorf("Range 返回 %d bytes, want 1024", len(b2))
	}
}

// TestStaticDirRootListing 验证下载站模式: 根路径直接返回文件列表,
// /static/ 与 /health 仍可用; 指定自定义 HTML 时根路径仍走 handleRoot
func TestStaticDirRootListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.bin"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	oldConfig, oldHTML, oldValid := config, htmlContent, staticDirValid
	config, htmlContent, staticDirValid = Config{StaticDir: dir}, nil, true
	defer func() { config, htmlContent, staticDirValid = oldConfig, oldHTML, oldValid }()

	port := freePort(t)
	go startServer(port)
	time.Sleep(100 * time.Millisecond)
	base := fmt.Sprintf("http://localhost:%d", port)

	// 根路径应为文件列表(包含文件名链接), 而非默认 JSON
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `"code":200`) {
		t.Error("根路径返回了默认 JSON, 应为文件列表")
	}
	if !strings.Contains(string(body), "test.bin") {
		t.Errorf("根路径列表未包含文件名 test.bin, body: %.200s", body)
	}

	// /static/ 路径仍可用
	resp2, err := http.Get(base + "/static/test.bin")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("/static/ 状态码 = %d, want 200", resp2.StatusCode)
	}

	// /health 不受影响
	resp3, err := http.Get(base + "/health")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp3.Body.Close()
	b3, _ := io.ReadAll(resp3.Body)
	if string(b3) != "OK" {
		t.Errorf("/health = %q, want OK", b3)
	}
}

// TestDirListHandlerHTML 验证 nginx 风格目录列表页: 文件/子目录/父链接/大小,
// 文件请求透传 FileServer(保留 Range), 路径转义
func TestDirListHandlerHTML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub dir"), 0755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(makeStaticFileHandler(http.Dir(dir), ""))
	defer srv.Close()

	// 目录列表页: HTML + 文件名 + 子目录 + 大小 + 父目录链接
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, 应为 text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	for _, want := range []string{"Index of /", "hello.txt", `href="sub%20dir/"`, "2 B", "Modified"} {
		if !strings.Contains(s, want) {
			t.Errorf("列表页缺少 %q", want)
		}
	}
	// 根路径不应有父目录链接
	if strings.Contains(s, `href="../"`) {
		t.Error("根路径不应显示 ../ 链接")
	}

	// 子目录列表(含空格的目录名, 验证相对链接可访问 + 父目录链接)
	resp2, err := http.Get(srv.URL + "/sub dir/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("子目录状态码 = %d, want 200", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), `href="../"`) {
		t.Error("子目录页缺少 ../ 父目录链接")
	}

	// 文件请求透传 FileServer: 内容一致 + Range 206
	resp3, err := http.Get(srv.URL + "/hello.txt")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp3.Body.Close()
	b3, _ := io.ReadAll(resp3.Body)
	if string(b3) != "hi" {
		t.Errorf("文件内容 = %q, want hi", b3)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/hello.txt", nil)
	req.Header.Set("Range", "bytes=0-0")
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Range 请求失败: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != 206 {
		t.Errorf("Range 状态码 = %d, want 206", resp4.StatusCode)
	}
}

// TestFormatBytes 验证字节数人性化显示
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.n); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestDefaultStaticDir 验证默认共享目录为二进制所在目录且存在
func TestDefaultStaticDir(t *testing.T) {
	d := defaultStaticDir()
	if d == "" {
		t.Fatal("defaultStaticDir() 为空")
	}
	if info, err := os.Stat(d); err != nil || !info.IsDir() {
		t.Errorf("defaultStaticDir() = %q, 不是有效目录", d)
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

// TestStatusCodeValidation 验证非法状态码不会导致 WriteHeader panic
func TestStatusCodeValidation(t *testing.T) {
	// 非法状态码（<100 或 >999）应回退到 200，而不是 panic
	invalid := []int{0, 50, 99, 1000, 99999}
	for _, code := range invalid {
		config = Config{Code: code}
		jsonContent = []byte(`{}`)
		htmlContent = nil
		if got := getStatusCode(); got != 200 {
			t.Errorf("getStatusCode() with Code=%d = %d, want 200 (回退)", code, got)
		}
	}

	// 合法状态码应原样返回
	valid := []int{100, 200, 302, 404, 500, 999}
	for _, code := range valid {
		config = Config{Code: code}
		if got := getStatusCode(); got != code {
			t.Errorf("getStatusCode() with Code=%d = %d, want %d", code, got, code)
		}
	}
}

// TestHandleRootNoPanic 确保非法 Code 下 handleRoot 不 panic
func TestHandleRootNoPanic(t *testing.T) {
	config = Config{Code: 50}
	jsonContent = []byte(`{}`)
	htmlContent = nil

	// 用 httptest 真实触发 WriteHeader
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handleRoot(rec, req)

	if rec.Code != 200 {
		t.Errorf("handleRoot with Code=50 应回退为 200，实际 = %d", rec.Code)
	}
}

// TestSplitMultiHost 验证多分隔符混用
func TestSplitMultiHost(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"10.0.0.1:8080", []string{"10.0.0.1:8080"}},
		{"10.0.0.1:8080#10.0.0.2:443", []string{"10.0.0.1:8080", "10.0.0.2:443"}},
		{"10.0.0.1:8080!10.0.0.2:443", []string{"10.0.0.1:8080", "10.0.0.2:443"}},
		// 混用分隔符
		{"10.0.0.1:8080#10.0.0.2:443!10.0.0.3:80", []string{"10.0.0.1:8080", "10.0.0.2:443", "10.0.0.3:80"}},
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

// TestDedupePorts 验证端口去重
func TestDedupePorts(t *testing.T) {
	tests := []struct {
		input    []int
		expected []int
	}{
		{[]int{8080, 8080}, []int{8080}},
		{[]int{8080, 9090, 8080}, []int{8080, 9090}},
		{[]int{80, 443, 80, 443}, []int{80, 443}},
	}

	for _, tt := range tests {
		got := dedupePorts(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("dedupePorts(%v) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("dedupePorts(%v)[%d] = %d, want %d", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
