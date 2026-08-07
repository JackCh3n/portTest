# Port Test Tool

一个轻量级端口测试工具，支持 **Port 模式**（HTTP 测试服务）、**TCPing 模式**（TCP 端口连通性测试）和 **CURL 模式**（模拟 curl 抓取内容）。

## 功能特性

- 🛡️ **Port 模式**：启动 HTTP 服务器占用端口，返回自定义 JSON/HTML/状态码
- 🗡️ **TCPing 模式**：主动 TCP 连接测试，验证网络是否畅通
- 🌐 **CURL 模式**：模拟 curl 发送 HTTP/HTTPS 请求，支持 GET/POST、自定义头、忽略证书
- 🌐 多主机批量测试，支持 IPv4/IPv6/域名
- 🔧 丰富的位置参数写法，兼容传统 `-ip`/`-p` 写法
- 💻 跨平台：Windows / Linux / macOS / FreeBSD / ARM / MIPS / LoongArch

## 快速开始

### 安装

```bash
go install github.com/yourname/port-test@latest
```

### Port 模式

```bash
# 指定端口（默认 8080）
port-test -port 8080

# 多个端口
port-test -port 8080 9090 3000

# 自定义状态码
port-test -port 8080 -code 403

# 返回 HTML 文件
port-test -port 8080 -html ./index.html

# 返回自定义 JSON
port-test -port 8080 -json '{"code":200,"msg":"ok"}'

# 传统写法（-p 指定端口）
port-test -p 8080,9090 -code 200
```

### TCPing 模式

```bash
# 单主机单端口
port-test -tcping 10.0.0.1:8080

# 单主机多端口（逗号）
port-test -tcping 10.0.0.1:8080,443,3306

# 单主机多端口（中文顿号）
port-test -tcping 10.0.0.1:8080、443、3306

# 单主机多端口（空格）
port-test -tcping 10.0.0.1:8080 443 3306

# host 和端口分开写
port-test -tcping 10.0.0.1 8080 443

# 端口范围
port-test -tcping 10.0.0.1 1-1024

# 多主机（用分隔符）
port-test -tcping 10.0.0.1:80#10.0.0.2:443
port-test -tcping 10.0.0.1:80!10.0.0.2:443
port-test -tcping 10.0.0.1 80,443/10.0.0.2 53,80

# IPv6
port-test -tcping ::1 80
port-test -tcping [::1]:80

# 传统写法（向后兼容）
port-test -tcping -ip 10.0.0.1 -p 80,443,3306

# 指定超时和重复次数
port-test -tcping -ip 10.0.0.1 -p 22,80 -timeout 5 -count 3 -interval 2
```

### CURL 模式

```bash
# GET 抓取网页内容
port-test -curl https://example.com

# 忽略 HTTPS 证书校验（自签名证书场景）
port-test -curl https://self-signed.example.com -k

# POST 表单请求
port-test -curl https://api.example.com/login -X POST -d 'username=admin&password=123'

# POST JSON 请求
port-test -curl http://api.example.com/json -X POST -d '{"key":"value"}' -H 'Content-Type: application/json'

# 自定义请求头（-H 可多次指定）
port-test -curl http://api.example.com -H 'Authorization: Bearer token123' -H 'X-Custom-Header: 1'

# 指定超时时间
port-test -curl https://example.com -timeout 10
```

> CURL 模式行为与 curl 一致：指定 `-d` 时自动使用 POST；不指定 `-d` 时 `-X POST` 也可强制 POST。
> 参数可以放在 URL 后面（如 `-curl https://x.com -k -X POST -d 'a=1'`），与 curl 习惯一致。

## 分隔符

### 端口分隔符

| 分隔符 | 示例 | 说明 |
|--------|------|------|
| `,` | `80,443,8080` | 英文逗号 |
| `、` | `80、443、8080` | 中文顿号 |
| ` ` | `80 443 8080` | 空格 |
| `-` | `1-1024` | 范围 |

### 多主机分隔符

| 分隔符 | 说明 |
|--------|------|
| `#` | 推荐，最直观 |
| `!` | |
| `=` | |
| `+` | |
| `\` | |
| `/` | |
| `?` | |

> 所有主机分隔符在 Linux/Mac/Windows 下均无需引号。

## 命令行参数

### 模式开关（互斥，三选一）

| 参数 | 说明 |
|------|------|
| `-port <ports...>` | Port 模式：启动 HTTP 测试服务 |
| `-tcping <targets...>` | TCPing 模式：TCP 端口连通性测试 |
| `-curl <url>` | CURL 模式：模拟 curl 请求 |

> 不带任何模式开关时：有位置参数/`-p`/`-ip` 则默认走 Port 模式；完全无参数则显示帮助。

### 全局

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-timeout` | `3` | 连接超时（秒，tcping/curl） |
| `-version` | - | 显示版本信息 |

### Port 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | `8080` | 端口号，逗号分隔（传统写法） |
| `-code` | `200` | HTTP 状态码 |
| `-json` | - | 自定义 JSON 响应 |
| `-html` | - | HTML 文件路径 |
| `-dir` | - | 静态文件目录 |

### TCPing 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-ip` | - | 目标 IP/域名（传统写法） |
| `-p` | - | 端口（传统写法） |
| `-count` | `1` | 测试次数 |
| `-interval` | `1` | 重试间隔（秒） |

### CURL 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-X` | `GET` | HTTP 方法（POST/PUT/DELETE 等） |
| `-d` | - | 请求体，指定后自动使用 POST |
| `-H` | - | 请求头，可多次指定（`-H 'Key: Value'`） |
| `-k` | `false` | 忽略 HTTPS 证书校验 |

## API 接口（Port 模式）

| 路径 | 说明 |
|------|------|
| `GET /` | 返回配置的响应内容 |
| `GET /health` | 健康检查，返回 `OK` |
| `GET /info` | 服务信息（端口、状态码、时间戳） |

## 构建

### 构建脚本

```bash
# Linux / macOS / Git Bash
./build.sh              # 默认: win-x86 + linux-x86
./build.sh win          # 所有 Windows 平台
./build.sh linux        # 所有 Linux 平台
./build.sh all          # 全部 15 种平台
./build.sh list         # 查看支持的平台

# Windows CMD
build.bat               # 默认: win-x86 + linux-x86
build.bat all           # 全部平台
```

### 手动交叉编译

```bash
GOOS=windows GOARCH=386 go build -o port-test-win-x86.exe .
GOOS=linux GOARCH=amd64 go build -o port-test-linux .
GOOS=darwin GOARCH=arm64 go build -o port-test-mac-arm64 .
```

## 支持的平台

| 操作系统 | 架构 | 说明 |
|---------|------|------|
| Windows | x86 / x64 / ARM64 | ✅ |
| Linux | x86 / x64 / ARM / ARM64 | ✅ |
| Linux | MIPS64 / MIPS64LE | ✅ 国产芯片 |
| Linux | LoongArch64 | ✅ 龙芯 |
| macOS | x64 / ARM64 | ✅ |
| FreeBSD | x64 / ARM64 | ✅ |

## 测试

```bash
go test -v ./...
```

## GitHub Actions

提交代码自动触发构建，Release 标题/标签格式：`v年_月日_时分`

二进制产物命名（不带时间戳）：`port-test-<系统>-<架构>[.exe]`

| 系统 | 架构 | 示例 |
| --- | --- | --- |
| Windows | x86 / x64 / ARM64 | `port-test-windows-386.exe` |
| Linux | x86 / x64 / ARM / ARM64 / MIPS / LoongArch | `port-test-linux-amd64` |
| macOS | x64 / ARM64 | `port-test-darwin-arm64` |
| FreeBSD / OpenBSD | x64 / ARM64 | `port-test-freebsd-amd64` |

## License

MIT
