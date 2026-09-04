#!/usr/bin/env bash
set -euo pipefail

test_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d "${TMPDIR:-/tmp}/gdspx-string-abi.XXXXXX")"
trap 'rm -rf "$build_dir"' EXIT

cxx="${CXX:-c++}"
common_flags=(
    -std=c++17
    -Wall
    -Wextra
    -Werror
    -I"$test_dir/stubs"
)
source_file="$test_dir/godot_js_spx_util_string_test.cpp"

"$cxx" "${common_flags[@]}" "$source_file" -o "$build_dir/string_abi_test"
"$build_dir/string_abi_test"

# UBSan is portable across the supported local toolchains. CI can add ASan
# with GDSPX_SANITIZERS=address,undefined.
sanitizers="${GDSPX_SANITIZERS:-undefined}"
"$cxx" "${common_flags[@]}" \
    -fsanitize="$sanitizers" \
    -fno-omit-frame-pointer \
    "$source_file" -o "$build_dir/string_abi_test_sanitized"
ASAN_OPTIONS=detect_leaks=0 UBSAN_OPTIONS=halt_on_error=1 \
    "$build_dir/string_abi_test_sanitized"
