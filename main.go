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

const version = "1.0.0"

// 打印工具头信息
func printHeader() {
	fmt.Println("====================================")
	fmt.Printf("  Port Test Tool v%s\n", version)
	fmt.Println("====================================")
}

// 打印 Port 模式帮助
func printPortHelp() {
	fmt.Println()
	fmt.Println("  🛡️  PORT 模式 - 启动 HTTP 测试服务")
	fmt.Println("  ----------------------------------------")
	fmt.Println("  启动 HTTP 服务器占用指定端口，返回自定义内容")
	fmt.Println()
	fmt.Println("  参数说明:")
	fmt.Println("    -p <ports>      端口号，多个端口用逗号分隔 (默认: 8080)")
	fmt.Println("    -code <code>    HTTP 状态码 (默认: 200)")
	fmt.Println("    -json <json>    自定义 JSON 响应内容")
	fmt.Println("    -html <path>    返回指定的 HTML 文件")
	fmt.Println("    -dir <path>     静态文件目录 (挂载到 /static/)")
	fmt.Println()
	fmt.Println("  示例:")
	fmt.Println("    port-test -p 8080")
	fmt.Println("    port-test -p 8080,9090,3000")
	fmt.Println("    port-test -p 8080 -code 403")
	fmt.Println("    port-test -p 8080 -html ./index.html")
	fmt.Println("    port-test -p 8080 -json '{\"code\":200,\"msg\":\"ok\"}'")
	fmt.Println("    port-test -p 8080 -dir ./static")
	fmt.Println()
}

// 打印 TCPing 模式帮助
func printTcpingHelp() {
	fmt.Println()
	fmt.Println("  🗡️  TCPING 模式 - TCP 端口连通性测试")
	fmt.Println("  ----------------------------------------")
	fmt.Println("  主动连接目标 IP 的端口，测试网络是否畅通")
	fmt.Println()
	fmt.Println("  参数说明:")
	fmt.Println("    -ip <host>      目标 IP 地址或域名 (必填)")
	fmt.Println("    -p <ports>      端口号，支持逗号分隔和范围 (如: 80,443 或 1-1024)")
	fmt.Println("    -timeout <sec>  连接超时时间，秒 (默认: 3)")
	fmt.Println("    -count <num>    测试次数 (默认: 1)")
	fmt.Println("    -interval <sec> 重试间隔，秒 (默认: 1)")
	fmt.Println()
	fmt.Println("  示例:")
	fmt.Println("    port-test -mode tcping -ip 192.168.1.1 -p 80,443,3306")
	fmt.Println("    port-test -mode tcping -ip 10.0.0.1 -p 1-1024")
	fmt.Println("    port-test -mode tcping -ip 114.114.114.114 -p 53,80,443")
	fmt.Println("    port-test -mode tcping -ip 192.168.1.1 -p 22,80 -count 3 -interval 2")
	fmt.Println()
}

// 打印总帮助
func printUsage() {
	printHeader()
	fmt.Println()
	fmt.Println("  用法:")
	fmt.Println("    port-test [选项]")
	fmt.Println()
	fmt.Println("  模式:")
	fmt.Println("    port      启动 HTTP 测试服务 (默认)")
	fmt.Println("    tcping    TCP 端口连通性测试")
	fmt.Println()
	fmt.Println("  使用 'port-test -mode port' 进入 Port 模式")
	fmt.Println("  使用 'port-test -mode tcping -ip <host> -p <ports>' 进入 TCPing 模式")
	fmt.Println()
	fmt.Println("  全局选项:")
	fmt.Println("    -mode <mode>   运行模式: port (默认) / tcping")
	fmt.Println("    -help          显示帮助信息")
	fmt.Println("    -version       显示版本信息")
	fmt.Println()
}

func main() {
	// 解析命令行参数
	portStr := flag.String("p", "8080", "端口号，多个端口用逗号分隔")
	code := flag.Int("code", 200, "HTTP 状态码")
	jsonStr := flag.String("json", "", "JSON 响应内容")
	htmlPath := flag.String("html", "", "HTML 文件路径")
	staticDir := flag.String("dir", "", "静态文件目录")
	
	// 模式参数
	mode := flag.String("mode", "port", "运行模式: port (默认) / tcping")
	target := flag.String("ip", "", "目标IP地址 (tcping 模式)")
	timeout := flag.Int("timeout", 3, "连接超时时间(秒, tcping 模式)")
	count := flag.Int("count", 1, "测试次数 (tcping 模式)")
	interval := flag.Int("interval", 1, "重试间隔(秒, tcping 模式)")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本
	if *showVersion {
		printHeader()
		os.Exit(0)
	}

	// 如果没有指定模式或者没有额外参数，显示帮助
	if len(os.Args) == 1 {
		printUsage()
		os.Exit(0)
	}

	// 根据模式运行
	switch *mode {
	case "tcping":
		// TCPing 模式 - 先显示帮助，再执行
		printHeader()
		printTcpingHelp()
		ports := parsePortRange(*portStr)
		if len(ports) == 0 {
			fmt.Println("  错误: 请指定端口号 (-p)")
			fmt.Println("  提示: 使用 -p 80,443 或 -p 1-1024 指定端口")
			os.Exit(1)
		}
		TcpingMode(*target, ports, time.Duration(*timeout)*time.Second, *count, time.Duration(*interval)*time.Second)

	case "port":
		// Port 模式 - 先显示帮助，再执行
		printHeader()
		printPortHelp()
		runPortMode(*portStr, *code, *jsonStr, *htmlPath, *staticDir)

	default:
		fmt.Printf("未知模式: %s\n", *mode)
		fmt.Println("可用模式: port (默认), tcping")
		fmt.Println("使用 -help 查看帮助信息")
		os.Exit(1)
	}
}

// runPortMode 启动 Port 模式（HTTP 服务器）
func runPortMode(portStr string, code int, jsonStr, htmlPath, staticDir string) {
	// 解析端口
	ports, err := parsePorts(portStr)
	if err != nil {
		fmt.Printf("  端口解析错误: %v\n", err)
		os.Exit(1)
	}

	config = Config{
		Ports:     ports,
		Code:      code,
		JSON:      jsonStr,
		HTML:      htmlPath,
		StaticDir: staticDir,
	}

	// 加载 HTML 文件（如果指定）
	if config.HTML != "" {
		data, err := os.ReadFile(config.HTML)
		if err != nil {
			fmt.Printf("  读取 HTML 文件失败: %v\n", err)
			os.Exit(1)
		}
		htmlContent = data
		fmt.Printf("  已加载 HTML 文件: %s\n", config.HTML)
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
	fmt.Println("  🛡️  启动 Port 模式")
	fmt.Println("  ----------------------------------------")
	fmt.Printf("  端口: %v\n", config.Ports)
	fmt.Printf("  状态码: %d\n", config.Code)
	fmt.Printf("  响应类型: %s\n", getResponseType())
	fmt.Println("  ----------------------------------------")

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
			fmt.Printf("  静态文件目录: %s\n", config.StaticDir)
		}
	}

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("  服务器启动: http://localhost:%d\n", port)
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
