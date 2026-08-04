# Port Test Tool

一个轻量级端口测试工具，支持 **Port 模式**（HTTP 测试服务）和 **TCPing 模式**（TCP 端口连通性测试）。

## 功能特性

- 🛡️ **Port 模式**：启动 HTTP 服务器占用端口，返回自定义 JSON/HTML/状态码
- 🗡️ **TCPing 模式**：主动 TCP 连接测试，验证网络是否畅通
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
# 默认启动 8080
port-test

# 指定端口
port-test -mode port 8080

# 多个端口
port-test -mode port 8080 9090 3000

# 自定义状态码
port-test -p 8080 -code 403

# 返回 HTML 文件
port-test -p 8080 -html ./index.html

# 返回自定义 JSON
port-test -p 8080 -json '{"code":200,"msg":"ok"}'
```

### TCPing 模式

```bash
# 单主机单端口
port-test -mode tcping 10.0.0.1:8080

# 单主机多端口（逗号）
port-test -mode tcping 10.0.0.1:8080,443,3306

# 单主机多端口（中文顿号）
port-test -mode tcping 10.0.0.1:8080、443、3306

# 单主机多端口（空格）
port-test -mode tcping 10.0.0.1:8080 443 3306

# host 和端口分开写
port-test -mode tcping 10.0.0.1 8080 443

# 端口范围
port-test -mode tcping 10.0.0.1 1-1024

# 多主机（用分隔符）
port-test -mode tcping 10.0.0.1:80#10.0.0.2:443
port-test -mode tcping 10.0.0.1:80!10.0.0.2:443
port-test -mode tcping 10.0.0.1 80,443/10.0.0.2 53,80

# IPv6
port-test -mode tcping ::1 80
port-test -mode tcping [::1]:80

# 传统写法（向后兼容）
port-test -mode tcping -ip 10.0.0.1 -p 80,443,3306

# 指定超时和重复次数
port-test -mode tcping -ip 10.0.0.1 -p 22,80 -timeout 5 -count 3 -interval 2
```

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

### 全局

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-mode` | `port` | 运行模式: `port` / `tcping` |
| `-version` | - | 显示版本信息 |

### Port 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | `8080` | 端口号，逗号分隔 |
| `-code` | `200` | HTTP 状态码 |
| `-json` | - | 自定义 JSON 响应 |
| `-html` | - | HTML 文件路径 |
| `-dir` | - | 静态文件目录 |

### TCPing 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-ip` | - | 目标 IP/域名（传统写法） |
| `-p` | - | 端口（传统写法） |
| `-timeout` | `3` | 连接超时（秒） |
| `-count` | `1` | 测试次数 |
| `-interval` | `1` | 重试间隔（秒） |

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
