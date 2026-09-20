#!/usr/bin/env bash
set -euo pipefail

test_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d "${TMPDIR:-/tmp}/spx-abi.XXXXXX")"
trap 'rm -rf "$build_dir"' EXIT

cxx="${CXX:-c++}"
flags=(-std=c++17 -Wall -Wextra -Werror -I"$test_dir/../web/tests/stubs")
source_file="$test_dir/standalone/spx_abi_test.cpp"
"$cxx" "${flags[@]}" "$source_file" -o "$build_dir/abi_test"
"$build_dir/abi_test"

# Keep the portable default used by the Web ABI suite; CI can request address,undefined.
"$cxx" "${flags[@]}" -fsanitize="${SPX_ABI_SANITIZERS:-undefined}" \
    -fno-omit-frame-pointer "$source_file" -o "$build_dir/abi_test_sanitized"
ASAN_OPTIONS=detect_leaks=0 UBSAN_OPTIONS=halt_on_error=1 "$build_dir/abi_test_sanitized"
