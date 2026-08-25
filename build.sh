#!/bin/bash
# Build script for Port Test Tool
# Default: win-x86 + linux-x86
# Usage:
#   ./build.sh              # build default: win-x86 + linux-x86
#   ./build.sh win-x86      # build win-x86 only
#   ./build.sh linux-x86    # build linux-x86 only
#   ./build.sh win          # build all windows (x86 + x64 + arm64)
#   ./build.sh linux        # build all linux (x86 + x64 + arm64 + arm)
#   ./build.sh darwin       # build all macos (x64 + arm64)
#   ./build.sh all          # build all supported platforms
#   ./build.sh list         # list supported platforms

APP_NAME="port-test"
DIST_DIR="dist"

# Supported platforms: "os arch notes"
PLATFORMS=(
  "windows 386   win-x86"
  "windows amd64 win-x64"
  "windows arm64 win-arm64"
  "linux   386   linux-x86"
  "linux   amd64 linux-x64"
  "linux   arm64 linux-arm64"
  "linux   arm   linux-arm"
  "linux   mips64      linux-mips64"
  "linux   mips64le    linux-mips64le"
  "linux   mips        linux-mips"
  "linux   mipsle      linux-mipsle"
  "linux   loong64     linux-loong64"
  "darwin  amd64 macos-x64"
  "darwin  arm64 macos-arm64"
  "freebsd amd64 freebsd-x64"
  "freebsd arm64 freebsd-arm64"
  "openbsd amd64 openbsd-x64"
)

build_one() {
  local goos=$1 goarch=$2 label=$3
  local ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  local out="${DIST_DIR}/${APP_NAME}-${label}${ext}"
  printf "  %-20s GOOS=%-8s GOARCH=%-8s -> %s\n" "$label" "$goos" "$goarch" "$out"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags="-s -w" -o "$out" . 2>/dev/null
  if [ $? -ne 0 ]; then
    echo "    ⚠️  skipped (Go may not support this target)"
    rm -f "$out"
  fi
}

build_label() {
  local target=$1
  for entry in "${PLATFORMS[@]}"; do
    read -r goos goarch label <<< "$entry"
    if [ "$label" = "$target" ]; then
      build_one "$goos" "$goarch" "$label"
      return
    fi
  done
  echo "Unknown target: $target"
  echo "Run '$0 list' to see supported targets"
}

build_group() {
  local group=$1
  case "$group" in
    win)    for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; [ "$goos" = "windows" ] && build_one "$goos" "$goarch" "$label"; done ;;
    linux)  for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; [ "$goos" = "linux" ]   && build_one "$goos" "$goarch" "$label"; done ;;
    darwin) for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; [ "$goos" = "darwin" ]  && build_one "$goos" "$goarch" "$label"; done ;;
    freebsd)for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; [ "$goos" = "freebsd" ] && build_one "$goos" "$goarch" "$label"; done ;;
    openbsd)for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; [ "$goos" = "openbsd" ] && build_one "$goos" "$goarch" "$label"; done ;;
    all)    for entry in "${PLATFORMS[@]}"; do read -r goos goarch label <<< "$entry"; build_one "$goos" "$goarch" "$label"; done ;;
    *)
      echo "Unknown group: $group"
      echo "Available groups: win linux darwin freebsd openbsd all"
      exit 1
      ;;
  esac
}

list_platforms() {
  echo "Supported platforms:"
  echo ""
  echo "  Labels (single target):"
  for entry in "${PLATFORMS[@]}"; do
    read -r goos goarch label <<< "$entry"
    printf "    %-20s (%s/%s)\n" "$label" "$goos" "$goarch"
  done
  echo ""
  echo "  Groups:"
  echo "    win          all Windows platforms (x86 x64 arm64)"
  echo "    linux        all Linux platforms (x86 x64 arm arm64 mips mipsle mips64 mips64le loong64)"
  echo "    darwin       all macOS platforms (x64 arm64)"
  echo "    freebsd      all FreeBSD platforms (x64 arm64)"
  echo "    openbsd      all OpenBSD platforms (x64)"
  echo "    all          all platforms above"
  echo ""
  echo "Usage examples:"
  echo "  ./build.sh                    # default: win-x86 + linux-x86"
  echo "  ./build.sh win-x86            # single target"
  echo "  ./build.sh win                # all windows"
  echo "  ./build.sh all                # everything"
}

# --- Main ---
echo "===================================="
echo "  Port Test Tool - Build Script"
echo "===================================="
echo ""

if [ ! -d "$DIST_DIR" ]; then
  mkdir -p "$DIST_DIR"
fi

TARGET="${1:-default}"

case "$TARGET" in
  list|-l|--list)
    list_platforms
    exit 0
    ;;
  default)
    echo "Building default targets: win-x86, linux-x86"
    echo ""
    build_label "win-x86"
    build_label "linux-x86"
    ;;
  all)
    echo "Building ALL platforms..."
    echo ""
    build_group "all"
    ;;
  win|linux|darwin|freebsd|openbsd)
    echo "Building $TARGET group..."
    echo ""
    build_group "$TARGET"
    ;;
  *)
    # Try as a single label
    build_label "$TARGET"
    ;;
esac

echo ""
echo "Output directory: $DIST_DIR/"
echo ""
ls -lh "$DIST_DIR/" 2>/dev/null | grep -v "^d" | awk '{print "  " $9, "(" $5 ")"}'
echo ""
echo "Done."
