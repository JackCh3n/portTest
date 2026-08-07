package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	config         Config
	htmlContent    []byte
	jsonContent    []byte
	staticDirValid bool // 静态目录是否有效（在 runPortMode 中一次性校验，避免多端口重复打印）
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
	fmt.Println("    port-test -port [端口...]      启动 HTTP 测试服务")
	fmt.Println("    port-test -tcping <目标...>    TCP 端口连通性测试")
	fmt.Println("    port-test -curl <URL>          模拟 curl 抓取网页内容")
	fmt.Println("    port-test -scan <主机>         快速端口扫描")
	fmt.Println("    port-test -bench <URL>         轻量 HTTP 压测")
	fmt.Println("    port-test -udp <目标...>       UDP 连通性测试")
	fmt.Println("    port-test -dns <域名>          DNS 记录查询")
	fmt.Println("    port-test -traceroute <主机>   路由追踪")
	fmt.Println()
	fmt.Println("  Port 模式:")
	fmt.Println("    port-test -port 8080                          指定端口")
	fmt.Println("    port-test -port 8080 9090                     多个端口")
	fmt.Println("    port-test -p 8080,9090                        用 -p 指定多个端口")
	fmt.Println("    port-test -p 8080 -code 403                   指定状态码")
	fmt.Println("    port-test -p 8080 -html index.html            指定 HTML 文件")
	fmt.Println("    port-test -p 8080 -json '{\"code\":200,\"msg\":\"ok\"}'")
	fmt.Println("    /echo 接口: 回显请求的 method/header/body (配合 curl 调试)")
	fmt.Println()
	fmt.Println("  TCPing 模式:")
	fmt.Println("    port-test -tcping 10.0.0.1:8080               单主机单端口")
	fmt.Println("    port-test -tcping 10.0.0.1:8080,443           单主机多端口(逗号)")
	fmt.Println("    port-test -tcping 10.0.0.1:8080 443 8088      单主机多端口(空格)")
	fmt.Println("    port-test -tcping 10.0.0.1:8080#10.0.0.2:443  多主机用#分隔")
	fmt.Println("    port-test -tcping 10.0.0.1 80,443#10.0.0.2 9999  多主机多端口")
	fmt.Println("    port-test -tcping -ip 10.0.0.1 -p 80,443      传统写法")
	fmt.Println("    port-test -tcping -ip 10.0.0.1 -p 1-1024      端口范围")
	fmt.Println("    端口分隔符: , 、 -")
	fmt.Println("    多主机分隔符: ! # = + \\ / ? (均无需引号)")
	fmt.Println()
	fmt.Println("  CURL 模式:")
	fmt.Println("    port-test -curl https://example.com                    GET 抓取")
	fmt.Println("    port-test -curl https://example.com -k                 忽略 HTTPS 证书")
	fmt.Println("    port-test -curl https://api.com/login -X POST -d 'a=1&b=2'  POST 请求")
	fmt.Println("    port-test -curl http://api.com/json -X POST -d '{\"k\":1}' -H 'Content-Type: application/json'")
	fmt.Println("    port-test -curl http://api.com -H 'Authorization: Bearer xxx' -H 'X-Custom: 1'")
	fmt.Println()
	fmt.Println("  SCAN 模式 (端口扫描):")
	fmt.Println("    port-test -scan 10.0.0.1                 扫描常见端口")
	fmt.Println("    port-test -scan 10.0.0.1 -p 1-1024       扫描指定范围")
	fmt.Println("    port-test -scan 10.0.0.1 -p 1-65535      全端口扫描")
	fmt.Println()
	fmt.Println("  BENCH 模式 (HTTP 压测):")
	fmt.Println("    port-test -bench https://example.com                  100请求/10并发")
	fmt.Println("    port-test -bench https://example.com -n 1000 -c 50    1000请求/50并发")
	fmt.Println("    port-test -bench http://api.com/login -X POST -d 'a=1'")
	fmt.Println()
	fmt.Println("  UDP 模式:")
	fmt.Println("    port-test -udp 10.0.0.1:53            测试 UDP 端口 (DNS)")
	fmt.Println("    port-test -udp 10.0.0.1 53,123        多端口")
	fmt.Println()
	fmt.Println("  DNS 模式:")
	fmt.Println("    port-test -dns example.com            查询所有记录")
	fmt.Println("    port-test -dns example.com -type A    仅查询 A 记录")
	fmt.Println("    port-test -dns example.com -ip 8.8.8.8 指定 DNS 服务器")
	fmt.Println()
	fmt.Println("  TRACEROUTE 模式 (需管理员/root):")
	fmt.Println("    port-test -traceroute 10.0.0.1                   UDP 追踪")
	fmt.Println("    port-test -traceroute example.com -T -p 443      TCP 追踪 443 端口")
	fmt.Println("    port-test -traceroute example.com -m 20 -w 2     最大20跳/超时2秒")
	fmt.Println()
	fmt.Println("  全局选项:")
	fmt.Println("    -timeout <sec>    连接超时时间 (tcping/curl/scan/udp/traceroute, 默认: 3)")
	fmt.Println("    -count <num>      测试次数 (tcping, 默认: 1)")
	fmt.Println("    -interval <sec>   重试间隔秒 (tcping, 默认: 1)")
	fmt.Println("    -X <method>       HTTP 方法 (curl/bench, 默认: GET)")
	fmt.Println("    -d <data>         请求体 (curl/bench, POST 时自动加 Content-Type)")
	fmt.Println("    -H <header>       HTTP 请求头, 可多次指定 (curl/bench)")
	fmt.Println("    -k                忽略 HTTPS 证书校验 (curl/bench)")
	fmt.Println("    -n <num>          总请求数 (bench, 默认: 100)")
	fmt.Println("    -c <num>          并发数 (bench, 默认: 10)")
	fmt.Println("    -type <type>      DNS 记录类型 (dns, 默认: 全部)")
	fmt.Println("    -ip <host>        目标地址 (tcping) 或 DNS 服务器 (dns)")
	fmt.Println("    -T                TCP 模式 (traceroute)")
	fmt.Println("    -m <num>          最大跳数 (traceroute, 默认: 30)")
	fmt.Println("    -w <sec>          每跳超时 (traceroute, 默认: 1)")
	fmt.Println("    -version          显示版本信息")
	fmt.Println()
}

func main() {
	// 解析命令行参数
	portStr := flag.String("p", "", "端口号，多个端口用逗号分隔")
	code := flag.Int("code", 200, "HTTP 状态码")
	jsonStr := flag.String("json", "", "JSON 响应内容")
	htmlPath := flag.String("html", "", "HTML 文件路径")
	staticDir := flag.String("dir", "", "静态文件目录")

	// 模式开关（替代原 -mode 参数）
	usePort := flag.Bool("port", false, "Port 模式: 启动 HTTP 测试服务")
	useTcping := flag.Bool("tcping", false, "TCPing 模式: TCP 端口连通性测试")
	useCurl := flag.Bool("curl", false, "CURL 模式: 模拟 curl 抓取内容")
	useScan := flag.Bool("scan", false, "SCAN 模式: 快速端口扫描")
	useBench := flag.Bool("bench", false, "BENCH 模式: 轻量 HTTP 压测")
	useUdp := flag.Bool("udp", false, "UDP 模式: UDP 连通性测试")
	useDns := flag.Bool("dns", false, "DNS 模式: DNS 记录查询")
	useTraceroute := flag.Bool("traceroute", false, "TRACEROUTE 模式: 路由追踪")

	// TCPing 参数
	target := flag.String("ip", "", "目标IP地址 (tcping 模式) 或 DNS 服务器 (dns 模式)")
	timeout := flag.Int("timeout", 3, "连接超时时间(秒, tcping/curl/scan/udp/traceroute 模式)")
	count := flag.Int("count", 1, "测试次数 (tcping 模式)")
	interval := flag.Int("interval", 1, "重试间隔(秒, tcping 模式)")

	// CURL/BENCH 参数
	method := flag.String("X", "", "HTTP 方法 (curl/bench 模式, 默认 GET)")
	data := flag.String("d", "", "请求体 (curl/bench 模式)")
	var headers headerList
	flag.Var(&headers, "H", "HTTP 请求头, 可多次指定 (curl/bench 模式)")
	insecure := flag.Bool("k", false, "忽略 HTTPS 证书校验 (curl/bench 模式)")

	// BENCH 参数
	benchTotal := flag.Int("n", 100, "总请求数 (bench 模式, 默认: 100)")
	benchConcurrency := flag.Int("c", 10, "并发数 (bench 模式, 默认: 10)")

	// DNS 参数
	dnsType := flag.String("type", "", "DNS 记录类型 (dns 模式, 默认: 全部)")

	// TRACEROUTE 参数
	trTcp := flag.Bool("T", false, "TCP 模式 (traceroute)")
	trMaxHops := flag.Int("m", 30, "最大跳数 (traceroute, 默认: 30)")
	trWait := flag.Int("w", 1, "每跳超时秒 (traceroute, 默认: 1)")

	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 获取位置参数（flag 解析后剩余的非 flag 参数）
	posArgs := flag.Args()

	// 回收位置参数中后置的 flag（兼容 "port-test -curl URL -k -X POST" 这类 curl 习惯写法）
	// Go flag 包在遇到第一个非 flag 参数后停止解析，URL 后面的 flag 会残留在位置参数中
	posArgs = extractTrailingFlags(posArgs,
		&flagOpts{
			timeout:       timeout,
			count:         count,
			interval:      interval,
			ip:            target,
			portStr:       portStr,
			method:        method,
			data:          data,
			headers:       &headers,
			insecure:      insecure,
			code:          code,
			usePort:       usePort,
			useTcping:     useTcping,
			useCurl:       useCurl,
			useScan:       useScan,
			useBench:      useBench,
			useUdp:        useUdp,
			useDns:        useDns,
			useTraceroute: useTraceroute,
			benchTotal:    benchTotal,
			benchConc:     benchConcurrency,
			dnsType:       dnsType,
			trTcp:         trTcp,
			trMaxHops:     trMaxHops,
			trWait:        trWait,
			showVersion:   showVersion,
			html:          htmlPath,
			jsonStr:       jsonStr,
			staticDir:     staticDir,
		})

	// 显示版本（可能在 flag.Parse 或 extractTrailingFlags 中被设置）
	if *showVersion {
		printHeader()
		os.Exit(0)
	}

	// 无任何模式开关且无位置参数时显示帮助
	// 兼容旧用法: 只带 -p/-code 等参数时默认走 port 模式
	if !*usePort && !*useTcping && !*useCurl && !*useScan && !*useBench && !*useUdp && !*useDns && !*useTraceroute {
		if len(posArgs) == 0 && *portStr == "" && *target == "" {
			printUsage()
			os.Exit(0)
		}
		// 旧用法默认 port 模式
		*usePort = true
	}

	// 同时指定多个模式时报错
	modes := 0
	for _, m := range []bool{*usePort, *useTcping, *useCurl, *useScan, *useBench, *useUdp, *useDns, *useTraceroute} {
		if m {
			modes++
		}
	}
	if modes > 1 {
		fmt.Println("  错误: 只能指定一种模式 (-port / -tcping / -curl / -scan / -bench / -udp / -dns / -traceroute 互斥)")
		os.Exit(1)
	}

	// 根据模式运行
	switch {
	case *useCurl:
		printHeader()
		runCurlMode(posArgs, *method, *data, headers, *insecure, *timeout)

	case *useTcping:
		printHeader()
		runTcpingMode(posArgs, *target, *portStr, *timeout, *count, *interval)

	case *useScan:
		printHeader()
		runScanMode(posArgs, *portStr, *timeout)

	case *useBench:
		printHeader()
		runBenchMode(posArgs, *method, *data, headers, *insecure, *timeout, *benchTotal, *benchConcurrency)

	case *useUdp:
		printHeader()
		runUdpMode(posArgs, *portStr, *timeout)

	case *useDns:
		printHeader()
		runDnsMode(posArgs, *dnsType, *target)

	case *useTraceroute:
		printHeader()
		// traceroute 端口从 -p 解析（与 scan/tcping 共用）
		trPort := 0
		if p, err := strconv.Atoi(strings.TrimSpace(*portStr)); err == nil {
			trPort = p
		}
		runTracerouteMode(posArgs, *trTcp, trPort, *trMaxHops, *trWait)

	default: // *usePort
		printHeader()
		runPortMode(posArgs, *portStr, *code, *jsonStr, *htmlPath, *staticDir)
	}
}

// runTcpingMode 解析位置参数并启动 tcping
// 支持的多主机分隔符: ! # = + \ / ?
// 支持的写法:
//   port-test -tcping 10.0.0.1:8080                    单主机单端口
//   port-test -tcping 10.0.0.1:8080,443               单主机多端口(逗号)
//   port-test -tcping 10.0.0.1:8080 443 8088          单主机多端口(空格)
//   port-test -tcping 10.0.0.1 8080 443               host和端口分开
//   port-test -tcping 10.0.0.1:8080#10.0.0.2:443      多主机用#分隔
//   port-test -tcping 10.0.0.1:8080,443#10.0.0.2:9999 多主机多端口
//   port-test -tcping -ip 10.0.0.1 -p 80,443           传统写法
func runTcpingMode(posArgs []string, flagIP, flagPorts string, timeoutSec, count, intervalSec int) {
	var targets []HostTarget

	if len(posArgs) > 0 {
		// 合并所有位置参数为一个字符串
		joined := strings.Join(posArgs, " ")

		// 按 ! # = + \ / ? 分割多主机
		hostSegs := splitMultiHost(joined)
		for _, seg := range hostSegs {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			ht := parseHostSegment(seg)
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
		fmt.Println("  用法: port-test -tcping <host:port> [port2 ...] [# host2:port ...]")
		fmt.Println("  或:   port-test -tcping -ip <host> -p <ports>")
		os.Exit(1)
	}
	for i := range targets {
		if len(targets[i].Ports) == 0 {
			fmt.Printf("  错误: 主机 %s 未指定端口\n", targets[i].Host)
			os.Exit(1)
		}
		// 去除重复端口
		targets[i].Ports = dedupePorts(targets[i].Ports)
	}

	MultiTcpingMode(targets, time.Duration(timeoutSec)*time.Second, count, time.Duration(intervalSec)*time.Second)
}

// dedupePorts 去除重复端口
func dedupePorts(ports []int) []int {
	seen := make(map[int]bool)
	result := make([]int, 0, len(ports))
	for _, p := range ports {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// parseHostSegment 解析单个主机段
// 支持: "10.0.0.1:8080,443", "[::1]:8080", "::1 8080", "10.0.0.1:8080 443", "10.0.0.1 8080 443"
func parseHostSegment(seg string) HostTarget {
	ht := HostTarget{}
	parts := strings.Fields(seg)
	if len(parts) == 0 {
		return ht
	}

	first := parts[0]

	// IPv6 带括号: [::1]:8080
	if strings.HasPrefix(first, "[") {
		end := strings.Index(first, "]")
		if end > 0 {
			ht.Host = first[1:end]
			rest := first[end+1:]
			if strings.HasPrefix(rest, ":") {
				// [::1]:8080,443
				portStr := strings.TrimPrefix(rest, ":")
				ht.Ports = append(ht.Ports, parsePortRange(portStr)...)
			}
		} else {
			ht.Host = first
		}
	} else if strings.Count(first, ":") >= 2 {
		// 纯 IPv6 地址（至少 2 个冒号），如 ::1 或 2001:db8::1
		ht.Host = first
	} else if strings.Contains(first, ":") {
		// IPv4:host:port 格式
		idx := strings.LastIndex(first, ":")
		ht.Host = strings.TrimSpace(first[:idx])
		portStr := strings.TrimSpace(first[idx+1:])
		ht.Ports = append(ht.Ports, parsePortRange(portStr)...)
	} else {
		// 纯 IPv4 或 hostname
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
//   port-test -port 8080
//   port-test -port 8080 9090 3000
//   port-test -p 8080,9090 (传统写法)
func runPortMode(posArgs []string, flagPorts string, code int, jsonStr, htmlPath, staticDir string) {
	var ports []int

	// 优先使用位置参数
	for _, arg := range posArgs {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		rangedPorts := parsePortRange(arg)
		if len(rangedPorts) > 0 {
			ports = append(ports, rangedPorts...)
		} else {
			fmt.Printf("  警告: 忽略无效端口: %q\n", arg)
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

	// 校验 HTTP 状态码（Go 要求 100-999，否则 WriteHeader panic）
	if code < 100 || code > 999 {
		fmt.Printf("  错误: 无效 HTTP 状态码: %d (有效范围 100-999)\n", code)
		os.Exit(1)
	}

	// 去重端口，避免同一端口重复启动导致监听冲突
	ports = dedupePorts(ports)

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

	// 校验静态目录（一次性，避免多端口重复打印）
	if config.StaticDir != "" {
		if info, err := os.Stat(config.StaticDir); err == nil && info.IsDir() {
			staticDirValid = true
			fmt.Printf("  静态文件目录: %s\n", config.StaticDir)
		} else {
			fmt.Printf("  警告: 静态目录不存在或不是目录: %s\n", config.StaticDir)
		}
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
// 支持: 逗号(,) 和 中文顿号(、) 作为端口分隔符
func parsePorts(s string) ([]int, error) {
	// 统一替换中文顿号为逗号
	s = strings.ReplaceAll(s, "、", ",")
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
	mux.HandleFunc("/echo", handleEcho)

	if config.StaticDir != "" && staticDirValid {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(config.StaticDir))))
	}

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
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
// Go 的 WriteHeader 要求状态码在 100-999 之间，否则会 panic
func getStatusCode() int {
	if config.Code >= 100 && config.Code <= 999 {
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
		"code":      getStatusCode(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(info)
}

// handleEcho 请求回显接口：返回请求的 method/path/query/headers/body
// 配合 curl 模式使用：port-test -curl http://localhost:8080/echo?a=1 -X POST -d '{"k":1}'
func handleEcho(w http.ResponseWriter, r *http.Request) {
	// 读取请求体（限制 1MB，防止超大请求体耗尽内存）
	limitedBody := io.LimitReader(r.Body, 1*1024*1024)
	body, _ := io.ReadAll(limitedBody)
	defer r.Body.Close()

	// 收集请求头
	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ", ")
	}

	echo := map[string]interface{}{
		"method":   r.Method,
		"path":     r.URL.Path,
		"query":    r.URL.RawQuery,
		"proto":    r.Proto,
		"remote":   r.RemoteAddr,
		"headers":  headers,
		"body":     string(body),
		"body_len": len(body),
		"time":     time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(getStatusCode())
	json.NewEncoder(w).Encode(echo)
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

// splitMultiHost 按多主机分隔符切分
// 支持: ! # = + \ / ? (均为非 shell 保留字符, 不需要引号)
// 多个分隔符可以混用，统一替换后按单个分隔符切分
func splitMultiHost(s string) []string {
	hasSep := false
	for _, sep := range []string{"!", "#", "=", "+", "\\", "/", "?"} {
		if strings.Contains(s, sep) {
			hasSep = true
			// 统一替换为空格，这样所有分隔符可以混用
			s = strings.ReplaceAll(s, sep, " ")
		}
	}
	if hasSep {
		return strings.Fields(s)
	}
	return []string{s}
}

// flagOpts 后置 flag 回收目标（指针指向 main 中的 flag 变量）
type flagOpts struct {
	timeout, count, interval *int
	ip, portStr              *string
	method, data             *string
	headers                  *headerList
	insecure                 *bool
	code                     *int
	usePort, useTcping       *bool
	useCurl, useScan         *bool
	useBench, useUdp         *bool
	useDns, useTraceroute    *bool
	benchTotal, benchConc    *int
	dnsType                  *string
	trTcp                    *bool
	trMaxHops, trWait        *int
	showVersion              *bool
	html, jsonStr, staticDir *string
}

// extractTrailingFlags 从位置参数中回收后置的 flag
// Go flag 包在遇到第一个非 flag 参数时停止解析，因此
// "port-test -curl https://x -k -timeout 10" 中 URL 后面的
// -k / -timeout 会残留在位置参数中。此函数把它们提取出来
// 应用到对应 flag 变量，并返回剩余的真实位置参数。
func extractTrailingFlags(args []string, opts *flagOpts) []string {
	valueFlags := map[string]func(string){
		"-timeout":  func(v string) { if opts.timeout != nil { *opts.timeout = atoiSafe(v, *opts.timeout) } },
		"-count":    func(v string) { if opts.count != nil { *opts.count = atoiSafe(v, *opts.count) } },
		"-interval": func(v string) { if opts.interval != nil { *opts.interval = atoiSafe(v, *opts.interval) } },
		"-ip":       func(v string) { if opts.ip != nil { *opts.ip = v } },
		"-p":        func(v string) { if opts.portStr != nil { *opts.portStr = v } },
		"-X":        func(v string) { if opts.method != nil { *opts.method = v } },
		"-d":        func(v string) { if opts.data != nil { *opts.data = v } },
		"-H":        func(v string) { if opts.headers != nil { *opts.headers = append(*opts.headers, v) } },
		"-code":     func(v string) { if opts.code != nil { *opts.code = atoiSafe(v, *opts.code) } },
		"-n":        func(v string) { if opts.benchTotal != nil { *opts.benchTotal = atoiSafe(v, *opts.benchTotal) } },
		"-c":        func(v string) { if opts.benchConc != nil { *opts.benchConc = atoiSafe(v, *opts.benchConc) } },
		"-type":     func(v string) { if opts.dnsType != nil { *opts.dnsType = v } },
		"-m":        func(v string) { if opts.trMaxHops != nil { *opts.trMaxHops = atoiSafe(v, *opts.trMaxHops) } },
		"-w":        func(v string) { if opts.trWait != nil { *opts.trWait = atoiSafe(v, *opts.trWait) } },
		"-html":     func(v string) { if opts.html != nil { *opts.html = v } },
		"-json":     func(v string) { if opts.jsonStr != nil { *opts.jsonStr = v } },
		"-dir":      func(v string) { if opts.staticDir != nil { *opts.staticDir = v } },
	}

	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// 布尔 flag
		boolHandled := true
		switch arg {
		case "-k":
			if opts.insecure != nil {
				*opts.insecure = true
			}
		case "-port":
			if opts.usePort != nil {
				*opts.usePort = true
			}
		case "-tcping":
			if opts.useTcping != nil {
				*opts.useTcping = true
			}
		case "-curl":
			if opts.useCurl != nil {
				*opts.useCurl = true
			}
		case "-scan":
			if opts.useScan != nil {
				*opts.useScan = true
			}
		case "-bench":
			if opts.useBench != nil {
				*opts.useBench = true
			}
		case "-udp":
			if opts.useUdp != nil {
				*opts.useUdp = true
			}
		case "-dns":
			if opts.useDns != nil {
				*opts.useDns = true
			}
		case "-traceroute":
			if opts.useTraceroute != nil {
				*opts.useTraceroute = true
			}
		case "-T":
			if opts.trTcp != nil {
				*opts.trTcp = true
			}
		case "-version":
			if opts.showVersion != nil {
				*opts.showVersion = true
			}
		default:
			boolHandled = false
		}
		if boolHandled {
			continue
		}

		// 值 flag: 需要下一个参数作为值
		if fn, ok := valueFlags[arg]; ok {
			if i+1 < len(args) {
				fn(args[i+1])
				i++
			} else {
				fmt.Printf("  警告: flag %s 缺少值, 已忽略\n", arg)
			}
			continue
		}

		// 普通位置参数
		rest = append(rest, arg)
	}
	return rest
}

// atoiSafe 安全解析整数，失败时返回默认值
func atoiSafe(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	fmt.Printf("  警告: 无效数值 %q, 使用默认值 %d\n", s, def)
	return def
}
