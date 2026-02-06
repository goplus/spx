#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
DIST_DIR="$PROJECT_ROOT/dist"

BUILD_WEB=false
BUILD_NATIVE=false

case "${1:-}" in
  "")
    ;;
  --web)
    BUILD_WEB=true
    ;;
  --native)
    BUILD_NATIVE=true
    ;;
  --all)
    BUILD_WEB=true
    BUILD_NATIVE=true
    ;;
  *)
    echo "Error: unsupported option '$1'"
    echo "Usage: $0 [--web|--native|--all]"
    exit 1
    ;;
esac

# Pin Go toolchain version for consistent wasm_exec.js and ispx.wasm
export GOTOOLCHAIN=go1.25.7

mkdir -p "$DIST_DIR/bin"
mkdir -p "$DIST_DIR/share/ispx"
mkdir -p "$DIST_DIR/share/engines"
mkdir -p "$DIST_DIR/share/templates"
mkdir -p "$DIST_DIR/lib"

# Download font if missing
FONT_DIR="$PROJECT_ROOT/cmd/gox/template/project/engine/fonts"
FONT_PATH="$FONT_DIR/CnFont.ttf"
if [ ! -f "$FONT_PATH" ]; then
  echo "Downloading CnFont.ttf..."
  mkdir -p "$FONT_DIR"
  curl -L https://github.com/goplus/godot/releases/download/spx2.0.14/CnFont.ttf -o "$FONT_PATH"
fi

if [ ! -f "$FONT_PATH" ]; then
  echo "Error: CnFont.ttf not found at $FONT_PATH"
  exit 1
fi

# Build spx
echo "Building spx..."
cd "$PROJECT_ROOT/cmd/gox"
if [ "$OS" = "Windows_NT" ]; then
  go build -ldflags="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition" -o "$DIST_DIR/bin/spx.exe"
else
  go build -ldflags="-checklinkname=0" -o "$DIST_DIR/bin/spx"
fi

# Build spxrun
echo "Building spxrun..."
cd "$PROJECT_ROOT/cmd/spxrun"
if [ "$OS" = "Windows_NT" ]; then
  go build -o "$DIST_DIR/bin/spxrun.exe"
else
  go build -o "$DIST_DIR/bin/spxrun"
fi

# Copy runtime.gdextension
cp "$PROJECT_ROOT/cmd/gox/template/project/runtime.gdextension.txt" "$DIST_DIR/share/runtime.gdextension"

if $BUILD_WEB; then
  echo "Building ispx.wasm..."
  cd "$PROJECT_ROOT/cmd/ispx"
  ./build.sh

  echo "Copying ispx web runtime..."
  cp -r "$PROJECT_ROOT/cmd/ispx/web/"* "$DIST_DIR/share/ispx/"

  if [ -f "$PROJECT_ROOT/cmd/ispx/ispx.wasm" ]; then
    mv "$PROJECT_ROOT/cmd/ispx/ispx.wasm" "$DIST_DIR/share/ispx/"
  fi

  echo "Copying wasm_exec.js..."
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$DIST_DIR/share/"
fi

if $BUILD_NATIVE; then
  echo "Building ispxnative..."
  cd "$PROJECT_ROOT/cmd/ispxnative"
  ./build.sh

  shopt -s nullglob
  for file in "$PROJECT_ROOT/cmd/ispxnative"/gdspx-*; do
    mv "$file" "$DIST_DIR/lib/"
  done
  shopt -u nullglob
fi

echo "Build completed. Output: $DIST_DIR"
