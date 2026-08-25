#!/usr/bin/env bash
# ============================================================
# Port Test Tool 多平台交叉编译脚本
# 用法: ./scripts/build-all.sh
# 产物输出到 dist/ 目录（命名与 GitHub Actions 矩阵完全一致）
# 注意: CGO_ENABLED=0 交叉编译，避免 cgo 需要目标平台交叉编译器
# ============================================================
set -euo pipefail

export CGO_ENABLED=0
export TZ=Asia/Shanghai

mkdir -p dist

build() {
  local os="$1" arch="$2"
  local notes="${3:-}"
  local ext="${4:-}"
  local name="port-test-${os}-${arch}${notes}${ext}"
  echo "==> 编译 $os/$arch -> dist/$name"
  GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="-s -w" -o "dist/${name}" .
}

# Windows
build windows amd64    ""    .exe
build windows 386      -x86  .exe
build windows arm64    ""    .exe

# Linux x86/x64
build linux amd64      ""    ""
build linux 386        -x86  ""

# Linux ARM
build linux arm64      ""    ""
build linux arm        ""    ""

# Linux MIPS (国产芯片支持)
build linux mips64     ""    ""
build linux mips64le   ""    ""
build linux mips       ""    ""
build linux mipsle     ""    ""

# Linux LoongArch (龙芯)
build linux loong64    ""    ""

# macOS
build darwin amd64     ""    ""
build darwin arm64     ""    ""

# FreeBSD
build freebsd amd64    ""    ""
build freebsd arm64    ""    ""

# OpenBSD
build openbsd amd64    ""    ""

echo ""
echo "==> 构建完成，产物列表："
ls -lh dist/
