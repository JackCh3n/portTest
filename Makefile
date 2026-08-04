.PHONY: build clean test run

# 默认构建当前平台
build:
	go build -o port-test main.go

# 运行
run:
	go run main.go -p 8080

# 测试
test:
	go test -v ./...

# 清理
clean:
	rm -f port-test port-test-* *.exe

# 构建所有平台
build-all:
	# Windows
	GOOS=windows GOARCH=386 go build -o port-test-win-x86.exe main.go
	GOOS=windows GOARCH=amd64 go build -o port-test-win-amd64.exe main.go
	GOOS=windows GOARCH=arm64 go build -o port-test-win-arm64.exe main.go
	# Linux
	GOOS=linux GOARCH=386 go build -o port-test-linux-x86 main.go
	GOOS=linux GOARCH=amd64 go build -o port-test-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -o port-test-linux-arm64 main.go
	GOOS=linux GOARCH=arm go build -o port-test-linux-arm main.go
	GOOS=linux GOARCH=mips64 go build -o port-test-linux-mips64 main.go
	GOOS=linux GOARCH=mips64le go build -o port-test-linux-mips64le main.go
	GOOS=linux GOARCH=loong64 go build -o port-test-linux-loong64 main.go
	# macOS
	GOOS=darwin GOARCH=amd64 go build -o port-test-mac-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o port-test-mac-arm64 main.go
	# FreeBSD
	GOOS=freebsd GOARCH=amd64 go build -o port-test-freebsd-amd64 main.go
	GOOS=freebsd GOARCH=arm64 go build -o port-test-freebsd-arm64 main.go

# 默认本地构建（Windows x86）
local:
	GOOS=windows GOARCH=386 go build -o port-test-win-x86.exe main.go

# TCPing 模式测试
tcping-test:
	go run main.go -mode tcping -ip 127.0.0.1 -p 80,443,3306
