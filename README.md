# Port Test Tool

一个轻量级网络调试工具，支持 **Port**（HTTP 测试服务）、**TCPing**（TCP 连通性测试）、**CURL**（模拟 curl）、**BENCH**（HTTP 压测）、**UDP**（UDP 测试）、**DNS**（DNS 查询）和 **TRACEROUTE**（路由追踪）共 7 种模式。

## 功能特性

- 🛡️ **Port 模式**：启动 HTTP/HTTPS 服务器占用端口，返回自定义 JSON/HTML/状态码，内置 `/echo` 回显接口
- 🗡️ **TCPing 模式**：主动 TCP 连接测试，验证网络是否畅通
- 🌐 **CURL 模式**：模拟 curl 发送 HTTP/HTTPS 请求，支持 GET/POST、自定义头、忽略证书、跟随重定向
- ⚡ **BENCH 模式**：轻量 HTTP 压测，输出 QPS/延迟分布（P50/P90/P99），支持短连接/keep-alive
- 📦 **UDP 模式**：UDP 连通性测试
- 🌏 **DNS 模式**：DNS 记录查询（A/AAAA/MX/CNAME/TXT/NS），支持自定义服务器端口
- 🛰️ **TRACEROUTE 模式**：UDP/TCP 路由追踪（需管理员/root 权限）
- 🌐 多主机批量测试，支持 IPv4/IPv6/域名
- 🔧 丰富的位置参数写法，兼容传统 `-ip`/`-p` 写法
- 📊 脚本友好：`-json-out` 机器可读输出 + 语义化退出码
- 💻 跨平台：Windows / Linux / macOS / FreeBSD / OpenBSD / ARM / MIPS / LoongArch（17 个平台预编译二进制）

## 快速开始

### 安装

**方式一：下载预编译二进制（推荐）**

从 [Releases](https://github.com/JackCh3n/portTest/releases) 下载对应平台二进制，无需解压，下载即用：

```bash
# 例如 Linux x64
curl -LO https://github.com/JackCh3n/portTest/releases/latest/download/port-test-linux-amd64
chmod +x port-test-linux-amd64
```

**方式二：go install**

```bash
go install github.com/JackCh3n/portTest@latest
```

**方式三：源码构建**

详见下方「构建」章节。当前版本：`v1.2.0`（运行 `port-test -version` 查看）。

### Port 模式

```bash
# 下载站: -dir 触发, 缺省共享二进制所在目录 + 随机端口(启动后显示实际地址)
port-test -dir

# 下载站: 指定目录与端口
port-test -dir D:\share -port 8080

# 测试服务: 指定端口(默认 8080)
port-test -port 8080

# 多个端口
port-test -port 8080 9090 3000

# 自定义状态码
port-test -port 8080 -code 403

# 返回 HTML 文件
port-test -port 8080 -html ./index.html

# 返回自定义 JSON
port-test -port 8080 -json '{"code":200,"msg":"ok"}'

# 启动 HTTPS 服务（自签名证书）
port-test -port 8443 -tls-cert cert.pem -tls-key key.pem

# 传统写法（-p 指定端口）
port-test -p 8080,9090 -code 200

# 传统写法（-p 范围写法）
port-test -p 8000-8010

# 临时文件下载站（内网分发大文件）
port-test -dir D:\share -port 8080
# 其他机器浏览器打开 http://<本机IP>:8080/ 即为文件列表, 直接点击下载
# 兼容旧路径: http://<本机IP>:8080/static/ 同样可用
```

列表页为 nginx autoindex 风格的极简页面（名称带类型图标 / 大小 / 修改时间，目录优先排序，支持逐级进入子目录）。

> 按 `Ctrl+C` 优雅退出：收到中断信号会关闭所有服务器后正常退出（退出码 0）；端口被占用等启动失败则退出码 3。

> `-dir` 挂载的静态目录支持 Range 断点续传（IDM/aria2/wget -c 多线程分块均可），流式传输不占内存，GB 级大文件无传输时限，可当内网临时文件下载站使用。
> 下载站模式下根路径 `/` 直接就是文件列表（未指定 `-html`/`-json` 时），无需拼接 `/static/` 前缀；`/health`、`/info`、`/echo` 接口不受影响。
> ⚠️ 注意共享目录会暴露其中所有文件（包括隐藏文件），不要把包含私钥/敏感信息的目录（如 `/root`、用户主目录）整个共享出去。

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

# 传统写法（空格分隔端口）
port-test -tcping -ip 10.0.0.1 -p 80 443 3306

# 指定超时和重复次数
port-test -tcping -ip 10.0.0.1 -p 22,80 -timeout 5 -count 3 -interval 2

# 混合写法（-p 补充的端口会应用到所有主机）
port-test -tcping 10.0.0.1:8080 -p 9090

# JSON 输出（脚本解析）
port-test -tcping 10.0.0.1:80,443 -json-out
```

> 无效端口不会静默忽略：会打印 `警告: 忽略无效端口` 并继续；`-ip` 指定了主机但没有任何有效端口时直接报错（退出码 1）。

### CURL 模式

```bash
# GET 抓取网页内容
port-test -curl https://example.com

# 忽略 HTTPS 证书校验（自签名证书场景）
port-test -curl https://self-signed.example.com -k

# 跟随重定向（如 302/301）
port-test -curl https://example.com -L

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
> `-H 'Host: example.com'` 可覆盖请求 Host（用于调试虚拟主机/反向代理场景）。
> 所有 flag 支持双横线风格：`--timeout 10` 与 `-timeout 10` 等价；任意位置的 `-h`/`--help` 打印帮助并退出 0。

### BENCH 模式（HTTP 压测）

```bash
# 默认 100 请求 / 10 并发
port-test -bench https://example.com

# 指定请求数和并发
port-test -bench https://example.com -n 1000 -c 50

# POST 压测
port-test -bench http://api.com/login -X POST -d 'username=admin&password=123'

# 忽略证书
port-test -bench https://self-signed.com -k -timeout 10

# 复用连接 (keep-alive) 压测，模拟长连接场景
port-test -bench https://example.com -n 1000 -c 50 -ka

# 跟随重定向压测
port-test -bench https://example.com -n 100 -c 10 -L
```

输出：总请求 / 成功 / 失败 / 耗时 / QPS / 平均延迟 / P50 / P90 / P99。

> QPS 按完成的请求数（成功 + 失败）÷ 总耗时计算，与主流压测工具口径一致；服务端大量 5xx 时不会低估实际吞吐。
> 默认短连接（每次请求断开，更贴近真实场景）；加 `-ka` 复用连接性能更高。

### UDP 模式

```bash
# 测试 UDP 端口（如 DNS 53）
port-test -udp 10.0.0.1:53

# 多端口
port-test -udp 10.0.0.1 53,123
```

> UDP 无连接：收到回包 = 开放；ICMP 拒绝 = 关闭；无响应 = 无法确认（防火墙可能丢弃）。

### DNS 模式

```bash
# 查询所有记录类型
port-test -dns example.com

# 仅查 A 记录
port-test -dns example.com -type A

# 指定 DNS 服务器
port-test -dns example.com -ip 8.8.8.8

# 指定 DNS 服务器（自定义端口）
port-test -dns example.com -ip 8.8.8.8:5353
```

> 整体查询限时 10 秒，超时的记录类型按失败输出，不会长时间挂起。
> 自定义 DNS 服务器同时支持 UDP 与 TCP（响应截断时自动回退 TCP 重试）。

### TRACEROUTE 模式（路由追踪）

```bash
# UDP traceroute
port-test -traceroute 10.0.0.1

# TCP traceroute 到 443 端口
port-test -traceroute example.com -T -p 443

# 自定义最大跳数和超时
port-test -traceroute example.com -m 20 -w 2
```

> ⚠️ 需要管理员/root 权限（监听 ICMP）。Windows 需以管理员运行，Linux 需 root 或 `sudo`。

> ⚠️ **Windows 限制**：Windows Vista 及以上系统过滤 ICMP 错误消息（Time Exceeded），
> raw socket 收不到中间路由器响应，**Windows 上无法显示中间跳**（只会显示 `* * *` 直到到达目标）。
> 完整路由追踪请在 **Linux/macOS** 上运行，或 Windows 安装 [Npcap](https://npcap.com/) 后使用 tracetcp。

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

### 模式开关（互斥，七选一）

| 参数 | 说明 |
|------|------|
| `-port <ports...>` | Port 模式：启动 HTTP 测试服务 |
| `-tcping <targets...>` | TCPing 模式：TCP 端口连通性测试 |
| `-curl <url>` | CURL 模式：模拟 curl 请求 |
| `-bench <url>` | BENCH 模式：轻量 HTTP 压测 |
| `-udp <host:port>` | UDP 模式：UDP 连通性测试 |
| `-dns <domain>` | DNS 模式：DNS 记录查询 |
| `-traceroute <host>` | TRACEROUTE 模式：路由追踪 |

> 不带任何模式开关时：完全无参数显示帮助；有位置参数/`-p`/`-ip` 则默认走 Port 模式。**`-dir` 是下载站触发开关**——即使不带路径和端口也会启动（共享二进制所在目录 + 随机端口）。帮助用 `-h`/`--help` 查看。

### 全局

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-timeout` | `3` | 连接超时（秒，tcping/curl/udp；traceroute 使用 `-w`） |
| `-json-out` | `false` | JSON 输出（tcping），便于脚本解析 |
| `-h` / `--help` | - | 显示帮助（任意位置均可，退出码 0） |
| `-version` | - | 显示版本信息 |

> 所有 flag 均支持 GNU 双横线风格（`--timeout` 等价 `-timeout`），包括 URL 之后的后置参数。

### 退出码

| 退出码 | 含义 |
|--------|------|
| `0` | 成功（含 Port 模式 Ctrl+C 主动中断） |
| `1` | 参数/用法错误 |
| `2` | 连接失败（curl 请求失败、tcping/udp/traceroute 目标无法解析） |
| `3` | 被中断/内部错误（端口占用、无权限等） |

> 脚本可据此判断结果：如 `port-test -curl URL` 退出码 `2` 表示目标不可达。

### Port 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | `8080`（`-dir` 下载站模式为随机） | 端口号，支持逗号分隔与范围写法（如 `8000-8010`，传统写法） |
| `-code` | `200` | HTTP 状态码 |
| `-json` | - | 自定义 JSON 响应 |
| `-html` | - | HTML 文件路径 |
| `-dir` | 二进制所在目录 | 静态文件目录（触发下载站模式，挂载为 `/static/` 及根路径列表，支持大文件与断点续传） |
| `-tls-cert` | - | HTTPS 证书路径（与 -tls-key 同时指定） |
| `-tls-key` | - | HTTPS 私钥路径 |

### TCPing 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-ip` | - | 目标 IP/域名（传统写法） |
| `-p` | - | 端口（传统写法） |
| `-count` | `1` | 测试次数 |
| `-interval` | `1` | 重试间隔（秒） |

### CURL / BENCH 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-X` | `GET` | HTTP 方法（POST/PUT/DELETE 等） |
| `-d` | - | 请求体，指定后自动使用 POST |
| `-H` | - | 请求头，可多次指定（`-H 'Key: Value'`） |
| `-k` | `false` | 忽略 HTTPS 证书校验 |
| `-L` | `false` | 跟随重定向（默认不跟随并提示 Location） |
| `-n` | `100` | 总请求数（仅 bench） |
| `-c` | `10` | 并发数（仅 bench） |
| `-ka` | `false` | 复用连接 keep-alive（仅 bench，默认短连接） |

### UDP 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-p` | - | 端口（传统写法，与位置参数端口合并） |
| `-timeout` | `3` | 连接超时（秒） |

### DNS 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-type` | 全部 | 记录类型：A/AAAA/MX/CNAME/TXT/NS（其他值报错） |
| `-ip` | 系统默认 | DNS 服务器地址（支持 `8.8.8.8:5353` 自定义端口） |

### TRACEROUTE 模式

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-T` | `false` | TCP 模式（默认 UDP） |
| `-p` | `33434`/`443` | 目标端口（UDP 起始 / TCP 默认 443） |
| `-m` | `30` | 最大跳数 |
| `-w` | `1` | 每跳超时（秒） |

## API 接口（Port 模式）

| 路径 | 说明 |
|------|------|
| `GET /` | 返回配置的响应内容；`-dir` 下载站模式下为文件列表 |
| `GET /health` | 健康检查，返回 `OK` |
| `GET /info` | 服务信息（端口、状态码、时间戳） |
| `* /echo` | 回显请求详情（method/path/query/headers/body），配合 curl 调试 |
| `GET /static/*` | 静态文件下载（`-dir` 指定目录），支持 Range 断点续传，可分发 GB 级大文件 |

## 构建

### 构建脚本

```bash
# Linux / macOS / Git Bash
./build.sh              # 默认: win-x86 + linux-x86 + linux-arm64
./build.sh win          # 所有 Windows 平台
./build.sh linux        # 所有 Linux 平台
./build.sh all          # 全部 17 种平台
./build.sh list         # 查看支持的平台

# Windows CMD
build.bat               # 默认: win-x86 + linux-x86 + linux-arm64
build.bat all           # 全部平台
```

> Windows 构建自动带版本信息：仓库内按架构提供的 `port-test_windows_386.syso` / `port-test_windows_amd64.syso`（由 `versioninfo.rc` 经 windres 生成）会被 `go build` 自动链入对应架构的 Windows 产物，exe 文件属性显示产品名/版本/描述。win/arm64 暂无 syso（工具链限制）不带版本属性。生成命令：`windres -O coff -F pe-i386 versioninfo.rc port-test_windows_386.syso`（amd64 用 `-F pe-x86-64`）。修改版本号时同步更新 `versioninfo.rc`、`main.go` 中的 `version` 常量并重新生成对应 syso。

### CI 构建脚本

`scripts/build-all.sh` 是 CI（CNB）使用的 17 平台交叉编译脚本，也等价于 `./build.sh all`：

```bash
bash ./scripts/build-all.sh    # 编译 17 个平台到 dist/
```

> 脚本内置 `CGO_ENABLED=0`，产物命名与 CI Release 完全一致（`port-test-<系统>-<架构>[-x86][.exe]`）。
> 注意：构建命令用 `-ldflags="-s -w"` 等号写法，避免引号在变量展开后拆分出错。

### 手动交叉编译

```bash
GOOS=windows GOARCH=386 go build -o port-test-win-x86.exe .
GOOS=linux GOARCH=amd64 go build -o port-test-linux .
GOOS=darwin GOARCH=arm64 go build -o port-test-mac-arm64 .
```

## 支持的平台

支持 17 个平台的预编译二进制（CI 每次提交自动构建）：

| 操作系统 | 架构 | 说明 |
|---------|------|------|
| Windows | x86 / x64 / ARM64 | ✅ |
| Linux | x86 / x64 / ARM / ARM64 | ✅ |
| Linux | MIPS / MIPSLE / MIPS64 / MIPS64LE | ✅ 国产芯片 |
| Linux | LoongArch64 | ✅ 龙芯 |
| macOS | x64 / ARM64 | ✅ |
| FreeBSD | x64 / ARM64 | ✅ |
| OpenBSD | x64 | ✅ |

## 测试

```bash
go test -v ./...
```

## 自动构建与发布（GitHub Actions / CNB）

提交代码自动触发构建，Release 标签格式：`v年_月日`（如 `v2026_0807`）；推送 tag（如 `v1.0.0`）则使用 tag 名作为版本号正式发版，也支持手动触发。

- **GitHub Actions**（`.github/workflows/build.yml`）：push 到 main/master 或 tag → 构建 17 平台 → 直接上传二进制 → 创建 Release
- **CNB 流水线**（`.cnb.yml`）：master push / tag 推送 / 手动触发 → 与 GitHub 相同流程构建发布（二进制均约 7MB，不做打包压缩，直接上传原始文件）
- **PR 质量门禁**：go vet + go test

二进制产物命名（不带时间戳）：`port-test-<系统>-<架构>[-x86][.exe]`（发布时直接提供原始二进制，不打包）：

| 系统 | 架构 | 示例 |
| --- | --- | --- |
| Windows | x86 / x64 / ARM64 | `port-test-windows-386-x86.exe` |
| Linux | x86 / x64 / ARM / ARM64 / MIPS / MIPSLE / MIPS64 / MIPS64LE / LoongArch | `port-test-linux-amd64` |
| macOS | x64 / ARM64 | `port-test-darwin-arm64` |
| FreeBSD / OpenBSD | x64 / ARM64 | `port-test-freebsd-amd64` |

## 版本规则

版本号格式 `主版本.次版本.修订号`（如 `1.1.1`），从 `1.0.0` 起算：

| 变更类型 | 递增幅度 | 示例 |
|---------|---------|------|
| 重大版本（架构重构/重量级功能改版） | 主版本 +1，如 `1.x.x → 2.0.0` | 新增重量级模式 |
| 漏洞修复/安全修复之类 | 次版本 +0.1，如 `1.1.x → 1.2.0` | 安全加固 |
| 常规提交（功能/文档/小改动，1 次提交） | 修订号 +0.0.1，如 `1.1.0 → 1.1.1` | 日常变更 |

每次变更提交时需同步更新三处：`main.go` 的 `version` 常量、`versioninfo.rc` 的 FILEVERSION/PRODUCTVERSION、README 此处的当前版本号。

## License

MIT
