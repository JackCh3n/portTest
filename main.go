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
	Ports     []int
	Code      int
	JSON      string
	HTML      string
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

// 打印总帮助
func printUsage() {
	printHeader()
	fmt.Println()
	fmt.Println("  用法:")
	fmt.Println("    port-test [选项] [位置参数]")
	fmt.Println()
	fmt.Println("  模式:")
	fmt.Println("    port      启动 HTTP 测试服务 (默认)")
	fmt.Println("    tcping    TCP 端口连通性测试")
	fmt.Println()
	fmt.Println("  Port 模式 (默认):")
	fmt.Println("    port-test                          默认启动 8080 端口")
	fmt.Println("    port-test -mode port 8080           指定端口")
	fmt.Println("    port-test -mode port 8080 9090      多个端口")
	fmt.Println("    port-test -p 8080,9090              用 -p 指定多个端口")
	fmt.Println("    port-test -p 8080 -code 403         指定状态码")
	fmt.Println("    port-test -p 8080 -html index.html  指定 HTML 文件")
	fmt.Println("    port-test -p 8080 -json '{\"code\":200,\"msg\":\"ok\"}'")
	fmt.Println()
	fmt.Println("  TCPing 模式:")
	fmt.Println("    port-test -mode tcping 10.0.0.1:8080               单主机单端口")
	fmt.Println("    port-test -mode tcping 10.0.0.1:8080,443           单主机多端口(逗号)")
	fmt.Println("    port-test -mode tcping 10.0.0.1:8080 443 8088      单主机多端口(空格)")
	fmt.Println("    port-test -mode tcping 10.0.0.1:8080|10.0.0.2:443  多主机用|分隔")
	fmt.Println("    port-test -mode tcping 10.0.0.1:8080,443|10.0.0.2:9999  多主机多端口")
	fmt.Println("    port-test -mode tcping -ip 10.0.0.1 -p 80,443     传统写法")
	fmt.Println("    port-test -mode tcping -ip 10.0.0.1 -p 1-1024     端口范围")
	fmt.Println()
	fmt.Println("  全局选项:")
	fmt.Println("    -mode <mode>     运行模式: port (默认) / tcping")
	fmt.Println("    -timeout <sec>   连接超时时间 (tcping, 默认: 3)")
	fmt.Println("    -count <num>     测试次数 (tcping, 默认: 1)")
	fmt.Println("    -interval <sec>  重试间隔秒 (tcping, 默认: 1)")
	fmt.Println("    -version         显示版本信息")
	fmt.Println()
}

func main() {
	// 解析命令行参数
	portStr := flag.String("p", "", "端口号，多个端口用逗号分隔")
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

	// 无参数时显示帮助
	if len(os.Args) == 1 {
		printUsage()
		os.Exit(0)
	}

	// 获取位置参数（flag 解析后剩余的非 flag 参数）
	posArgs := flag.Args()

	// 根据模式运行
	switch *mode {
	case "tcping":
		printHeader()
		runTcpingMode(posArgs, *target, *portStr, *timeout, *count, *interval)

	case "port":
		printHeader()
		runPortMode(posArgs, *portStr, *code, *jsonStr, *htmlPath, *staticDir)

	default:
		fmt.Printf("未知模式: %s\n", *mode)
		fmt.Println("可用模式: port (默认), tcping")
		os.Exit(1)
	}
}

// runTcpingMode 解析位置参数并启动 tcping
// 支持的写法:
//   port-test -mode tcping 10.0.0.1:8080                    单主机单端口
//   port-test -mode tcping 10.0.0.1:8080,443               单主机多端口(逗号)
//   port-test -mode tcping 10.0.0.1:8080 443 8088          单主机多端口(空格)
//   port-test -mode tcping 10.0.0.1 8080 443               host和端口分开
//   port-test -mode tcping 10.0.0.1:8080|10.0.0.2:443      多主机用|分隔
//   port-test -mode tcping 10.0.0.1:8080,443|10.0.0.2:9999 多主机多端口
//   port-test -mode tcping -ip 10.0.0.1 -p 80,443           传统写法
func runTcpingMode(posArgs []string, flagIP, flagPorts string, timeoutSec, count, intervalSec int) {
	var targets []HostTarget

	if len(posArgs) > 0 {
		// 合并所有位置参数为一个字符串
		joined := strings.Join(posArgs, " ")

		// 按 | 分割多主机
		hostSegs := strings.Split(joined, "|")
		for i, seg := range hostSegs {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			ht := parseHostSegment(seg)
			if i == len(hostSegs)-1 && len(hostSegs) > 1 {
				// 最后一段，如果有裸端口，附加到最后一个主机
				// parseHostSegment 已经处理了
			}
			if ht.Host != "" {
				targets = append(targets, ht)
			}
		}
	}

	// 回退到传统 -ip -p 写法
	if len(targets) == 0 && flagIP != "" {
		ht := HostTarget{Host: flagIP}
		if flagPorts != "" {
			ht.Ports = parsePortRange(flagPorts)
		}
		targets = append(targets, ht)
	}

	// 校验
	if len(targets) == 0 {
		fmt.Println("  错误: 请指定目标地址")
		fmt.Println("  用法: port-test -mode tcping <host:port> [port2 ...] [| host2:port ...]")
		fmt.Println("  或:   port-test -mode tcping -ip <host> -p <ports>")
		os.Exit(1)
	}
	for i := range targets {
		if len(targets[i].Ports) == 0 {
			fmt.Printf("  错误: 主机 %s 未指定端口\n", targets[i].Host)
			os.Exit(1)
		}
	}

	MultiTcpingMode(targets, time.Duration(timeoutSec)*time.Second, count, time.Duration(intervalSec)*time.Second)
}

// parseHostSegment 解析单个主机段
// 支持: "10.0.0.1:8080,443", "10.0.0.1:8080 443", "10.0.0.1 8080 443"
func parseHostSegment(seg string) HostTarget {
	ht := HostTarget{}
	parts := strings.Fields(seg)
	if len(parts) == 0 {
		return ht
	}

	first := parts[0]
	if strings.Contains(first, ":") {
		// host:port 格式
		colonParts := strings.SplitN(first, ":", 2)
		ht.Host = strings.TrimSpace(colonParts[0])
		portStr := strings.TrimSpace(colonParts[1])
		// 逗号分隔的端口
		ht.Ports = append(ht.Ports, parsePortRange(portStr)...)
	} else {
		// 纯 host
		ht.Host = first
	}

	// 后续部分都是端口
	for _, p := range parts[1:] {
		ht.Ports = append(ht.Ports, parsePortRange(p)...)
	}

	return ht
}

// runPortMode 解析位置参数并启动 port 模式
// 支持的写法:
//   port-test -mode port 8080
//   port-test -mode port 8080 9090 3000
//   port-test -p 8080,9090 (传统写法)
func runPortMode(posArgs []string, flagPorts string, code int, jsonStr, htmlPath, staticDir string) {
	var ports []int

	// 优先使用位置参数
	for _, arg := range posArgs {
		arg = strings.TrimSpace(arg)
		rangedPorts := parsePortRange(arg)
		if len(rangedPorts) > 0 {
			ports = append(ports, rangedPorts...)
		}
	}

	// 回退到 -p flag
	if len(ports) == 0 && flagPorts != "" {
		parsed, err := parsePorts(flagPorts)
		if err != nil {
			fmt.Printf("  端口解析错误: %v\n", err)
			os.Exit(1)
		}
		ports = parsed
	}

	// 默认 8080
	if len(ports) == 0 {
		ports = []int{8080}
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
		resp := JSONResponse{Code: 200, Msg: "hello"}
		data, _ := json.Marshal(resp)
		jsonContent = data
	}

	// 打印启动信息
	fmt.Printf("  端口: %v\n", config.Ports)
	fmt.Printf("  状态码: %d\n", config.Code)
	fmt.Printf("  响应类型: %s\n", getResponseType())
	fmt.Println("  ------------------------------------")

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

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for err := range errCh {
		fmt.Println(err)
		os.Exit(1)
	}

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
	if htmlContent != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(getStatusCode())
		w.Write(htmlContent)
		return
	}

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
