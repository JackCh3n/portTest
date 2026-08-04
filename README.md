# Port Test Tool

一个轻量级端口测试工具，可以占用指定端口并启动测试 Web 页面。

## 功能特性

- 🚀 一键启动，支持同时占用多个端口
- 📝 支持自定义返回内容（JSON / HTML / 自定义状态码）
- 🌐 提供简单的 Web 管理界面（健康检查、服务信息）
- 🔧 支持命令行参数配置
- 💻 跨平台支持：Windows / Linux / macOS / FreeBSD / ARM / MIPS / LoongArch

## 快速开始

### 安装

```bash
go install github.com/yourname/port-test@latest
```

### 使用

```bash
# 启动默认端口 8080，返回 JSON
port-test -p 8080

# 启动多个端口
port-test -p 8080,9090,3000

# 指定返回自定义状态码
port-test -p 8080 -code 403

# 指定返回 HTML 文件
port-test -p 8080 -html ./index.html

# 指定返回 JSON
port-test -p 8080 -json '{"code":200,"msg":"hello"}'

# 组合使用
port-test -p 8080,9090 -code 200 -html ./index.html
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | `8080` | 端口号，多个端口用逗号分隔 |
| `-code` | `200` | HTTP 状态码 |
| `-json` | - | 返回 JSON 内容 |
| `-html` | - | 返回指定 HTML 文件 |
| `-dir` | - | 静态文件目录 |

## API 接口

### GET `/`

返回配置的响应内容（JSON 或 HTML）。

### GET `/health`

健康检查接口，返回 `OK`。

### GET `/info`

获取当前服务信息，包括状态、端口、状态码等。

## 构建

### 本地构建

```bash
go build -o port-test main.go
```

### 交叉编译

```bash
# Windows (x86)
GOOS=windows GOARCH=386 go build -o port-test-win-x86.exe main.go

# Windows (x64)
GOOS=windows GOARCH=amd64 go build -o port-test-win-amd64.exe main.go

# Windows (ARM64)
GOOS=windows GOARCH=arm64 go build -o port-test-win-arm64.exe main.go

# Linux (x64)
GOOS=linux GOARCH=amd64 go build -o port-test-linux main.go

# Linux (ARM64)
GOOS=linux GOARCH=arm64 go build -o port-test-linux-arm64 main.go

# Linux (MIPS64 - 国产芯片)
GOOS=linux GOARCH=mips64 go build -o port-test-linux-mips64 main.go

# Linux (LoongArch64 - 龙芯)
GOOS=linux GOARCH=loong64 go build -o port-test-linux-loong64 main.go

# macOS (x64)
GOOS=darwin GOARCH=amd64 go build -o port-test-mac main.go

# macOS (ARM64/M1/M2)
GOOS=darwin GOARCH=arm64 go build -o port-test-mac-arm64 main.go
```

## 支持的平台

| 操作系统 | 架构 | 说明 |
|---------|------|------|
| Windows | x86 (32位) | ✅ |
| Windows | x64 (64位) | ✅ |
| Windows | ARM64 | ✅ |
| Linux | x86 (32位) | ✅ |
| Linux | x64 (64位) | ✅ |
| Linux | ARM | ✅ |
| Linux | ARM64 | ✅ |
| Linux | MIPS64 | ✅ 国产芯片 |
| Linux | MIPS64LE | ✅ 国产芯片 |
| Linux | LoongArch64 | ✅ 龙芯 |
| macOS | x64 (Intel) | ✅ |
| macOS | ARM64 (M1/M2/M3) | ✅ |
| FreeBSD | x64 | ✅ |
| FreeBSD | ARM64 | ✅ |

## 测试

```bash
go test -v ./...
```

## GitHub Actions

项目配置了 GitHub Actions，提交代码到 main 分支自动触发构建。

构建产物标题格式：`v年_月日_时分`（例如：`v2026_0804_1810`）

## License

MIT
