#!/bin/bash

# merge js files
cp -f js/raw/header.js js/engine.js
cat js/raw/engine.js >> js/engine.js
cat js/raw/wasm_exec.js >> js/engine.js
cat js/raw/game.js >> js/engine.js


# 压缩wasm文件为Brotli格式的脚本
# 使用最高压缩级别(11)以获得最佳压缩比

echo "开始压缩wasm文件为.br格式..."

# 检查brotli是否已安装
if ! command -v brotli &> /dev/null; then
    echo "错误: brotli 未安装"
    echo "macOS用户请运行: brew install brotli"
    echo "Ubuntu/Debian用户请运行: sudo apt-get install brotli"
    echo "CentOS/RHEL用户请运行: sudo yum install brotli"
    exit 1
fi

# 定义文件路径
GODOT_EDITOR_WASM="engine/engine.wasm"
GDSPX_WASM="engine/gdspx.wasm"

# 检查文件是否存在
if [ ! -f "$GODOT_EDITOR_WASM" ]; then
    echo "错误: $GODOT_EDITOR_WASM 文件不存在"
    exit 1
fi

if [ ! -f "$GDSPX_WASM" ]; then
    echo "错误: $GDSPX_WASM 文件不存在"
    exit 1
fi

# 压缩engine.wasm
echo "正在压缩 $GODOT_EDITOR_WASM..."
brotli -f -q 11 "$GODOT_EDITOR_WASM"
if [ $? -eq 0 ]; then
    echo "✓ $GODOT_EDITOR_WASM 压缩完成 -> ${GODOT_EDITOR_WASM}.br"
    # 显示压缩比
    original_size=$(stat -f%z "$GODOT_EDITOR_WASM" 2>/dev/null || stat -c%s "$GODOT_EDITOR_WASM" 2>/dev/null)
    compressed_size=$(stat -f%z "${GODOT_EDITOR_WASM}.br" 2>/dev/null || stat -c%s "${GODOT_EDITOR_WASM}.br" 2>/dev/null)
    if [ -n "$original_size" ] && [ -n "$compressed_size" ]; then
        ratio=$(awk "BEGIN {printf \"%.1f\", ($compressed_size/$original_size)*100}")
        echo "  压缩比: ${ratio}% ($(numfmt --to=iec $original_size) -> $(numfmt --to=iec $compressed_size))"
    fi
else
    echo "✗ $GODOT_EDITOR_WASM 压缩失败"
fi

# 压缩gdspx.wasm
echo "正在压缩 $GDSPX_WASM..."
brotli -f -q 11 "$GDSPX_WASM"
if [ $? -eq 0 ]; then
    echo "✓ $GDSPX_WASM 压缩完成 -> ${GDSPX_WASM}.br"
    # 显示压缩比
    original_size=$(stat -f%z "$GDSPX_WASM" 2>/dev/null || stat -c%s "$GDSPX_WASM" 2>/dev/null)
    compressed_size=$(stat -f%z "${GDSPX_WASM}.br" 2>/dev/null || stat -c%s "${GDSPX_WASM}.br" 2>/dev/null)
    if [ -n "$original_size" ] && [ -n "$compressed_size" ]; then
        ratio=$(awk "BEGIN {printf \"%.1f\", ($compressed_size/$original_size)*100}")
        echo "  压缩比: ${ratio}% ($(numfmt --to=iec $original_size) -> $(numfmt --to=iec $compressed_size))"
    fi
else
    echo "✗ $GDSPX_WASM 压缩失败"
fi

echo "压缩任务完成!"

# 显示结果文件
echo ""
echo "生成的压缩文件:"
if [ -f "${GODOT_EDITOR_WASM}.br" ]; then
    echo "- ${GODOT_EDITOR_WASM}.br"
fi
if [ -f "${GDSPX_WASM}.br" ]; then
    echo "- ${GDSPX_WASM}.br"
fi 


