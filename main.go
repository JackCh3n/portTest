package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config 应用配置
type Config struct {
	Ports    []int
	Code     int
	JSON     string
	HTML     string
	StaticDir string
}

var (
	config      Config
	htmlContent []byte
	jsonContent []byte
)

// JSONResponse 默认 JSON 响应
type JSONResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func main() {
	// 解析命令行参数
	portStr := flag.String("p", "8080", "端口号，多个端口用逗号分隔")
	code := flag.Int("code", 200, "HTTP 状态码")
	jsonStr := flag.String("json", "", "JSON 响应内容")
	htmlPath := flag.String("html", "", "HTML 文件路径")
	staticDir := flag.String("dir", "", "静态文件目录")
	flag.Parse()

	// 解析端口
	ports, err := parsePorts(*portStr)
	if err != nil {
		fmt.Printf("端口解析错误: %v\n", err)
		os.Exit(1)
	}

	config = Config{
		Ports:     ports,
		Code:      *code,
		JSON:      *jsonStr,
		HTML:      *htmlPath,
		StaticDir: *staticDir,
	}

	// 加载 HTML 文件（如果指定）
	if config.HTML != "" {
		data, err := os.ReadFile(config.HTML)
		if err != nil {
			fmt.Printf("读取 HTML 文件失败: %v\n", err)
			os.Exit(1)
		}
		htmlContent = data
		fmt.Printf("已加载 HTML 文件: %s\n", config.HTML)
	}

	// 准备 JSON 响应
	if config.JSON != "" {
		jsonContent = []byte(config.JSON)
	} else {
		resp := JSONResponse{
			Code: 200,
			Msg:  "hello",
		}
		data, _ := json.Marshal(resp)
		jsonContent = data
	}

	// 打印启动信息
	fmt.Println("====================================")
	fmt.Println("  Port Test Tool v1.0.0")
	fmt.Println("====================================")
	fmt.Printf("端口: %v\n", config.Ports)
	fmt.Printf("状态码: %d\n", config.Code)
	fmt.Printf("响应类型: %s\n", getResponseType())
	fmt.Println("====================================")

	// 启动 HTTP 服务器
	var wg sync.WaitGroup
	errCh := make(chan error, len(config.Ports))

	for _, port := range config.Ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if err := startServer(p); err != nil {
				errCh <- fmt.Errorf("端口 %d 启动失败: %v", p, err)
			}
		}(port)
	}

	// 等待所有服务器启动或出错
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// 检查启动错误
	for err := range errCh {
		fmt.Println(err)
		os.Exit(1)
	}

	// 保持运行
	select {}
}

// parsePorts 解析端口字符串
func parsePorts(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	ports := make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("无效端口号: %s", p)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("端口号超出范围: %d", port)
		}
		ports = append(ports, port)
	}

	return ports, nil
}

// startServer 启动单个 HTTP 服务器
func startServer(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/info", handleInfo)

	// 静态文件服务（如果指定目录）
	if config.StaticDir != "" {
		if _, err := os.Stat(config.StaticDir); err == nil {
			mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(config.StaticDir))))
			fmt.Printf("静态文件目录: %s\n", config.StaticDir)
		}
	}

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("服务器启动: http://localhost:%d\n", port)
	return server.ListenAndServe()
}

// handleRoot 根路径处理器
func handleRoot(w http.ResponseWriter, r *http.Request) {
	// 如果指定了 HTML 文件，返回 HTML
	if htmlContent != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(getStatusCode())
		w.Write(htmlContent)
		return
	}

	// 返回 JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(getStatusCode())
	if len(jsonContent) > 0 {
		w.Write(jsonContent)
	} else {
		w.Write([]byte(`{"code":200,"msg":"hello"}`))
	}
}

// getStatusCode 获取有效的 HTTP 状态码
func getStatusCode() int {
	if config.Code > 0 {
		return config.Code
	}
	return 200
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleInfo 服务信息
func handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now().Format(time.RFC3339),
		"ports":     config.Ports,
		"code":      config.Code,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(info)
}

// getResponseType 获取响应类型描述
func getResponseType() string {
	if htmlContent != nil {
		return "HTML"
	}
	if config.JSON != "" {
		return "Custom JSON"
	}
	return "Default JSON"
}
