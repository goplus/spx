#!/usr/bin/env bash
set -euo pipefail

source "$(dirname -- "${BASH_SOURCE[0]}")/platform.sh"

assert_smoke_test_result() {
  local status="$1" log_path="$2" platform="$3"
  if ! spx_run_reached_success "$log_path"; then
    log_error "Smoke test success marker not found (exit $status)"
    return 1
  fi
  if [ "$status" -ne 0 ]; then
    if [ "$platform" != windows ]; then
      log_error "Smoke test command failed (exit $status)"
      return 1
    fi
    # Preserve the existing Windows shutdown policy after project completion.
    log_warn "Smoke test reached success marker but exited with status $status"
    log_info "Continuing because Windows gdspxrt can occasionally fail during shutdown after the project completed"
  fi
  log_success "Smoke test success marker found"
}

run_smoke_test() (
  local spx="$1" project="$2" log_path="$3" platform="$4" isolated_dir="$5" status module_cache build_cache
  # Keep compiler caches, but prevent a binary without embedded runtime assets
  # from falling back to assets installed by the preparation step in GOPATH/bin.
  module_cache="$(go env GOMODCACHE)"
  build_cache="$(go env GOCACHE)"
  export GOMODCACHE="$module_cache" GOCACHE="$build_cache"
  mkdir -p "$isolated_dir/gopath/bin" "$isolated_dir/cache"
  export GOPATH="$(normalize_windows_path "$isolated_dir/gopath")"
  export GOBIN="$(normalize_windows_path "$isolated_dir/gopath/bin")"
  export GOWORK=off
  export XDG_CACHE_HOME="$isolated_dir/cache"
  export LOCALAPPDATA="$(normalize_windows_path "$isolated_dir/cache")"
  # macOS UserCacheDir uses HOME/Library/Caches. Its embedded cache verifies the
  # binary's own payload and is never searched by a binary without that payload.
  # Leave HOME and the installed Go/XGo toolchain unchanged on every platform.
  log_section "Run isolated standalone smoke test"
  set +e
  "$spx" run --path="$project" --headless 2>&1 | tee "$log_path"
  status=${PIPESTATUS[0]}
  set -e
  assert_smoke_test_result "$status" "$log_path" "$platform"
)

main() (
  : "${PACKAGE_FILE:?}" "${PLATFORM:?}" "${PACKAGE_ARCH:?}"
  : "${RUNNER_OS_CONTEXT:?}" "${RUNNER_ARCH_CONTEXT:?}"
  local workspace target_platform target_arch runner_platform runner_arch binary_name package_path verify_dir spx project spx_version source_dir
  workspace="$(pwd)"
  target_platform="$(normalize_platform "$PLATFORM")"
  target_arch="$(normalize_arch "$PACKAGE_ARCH")"
  runner_platform="$(normalize_platform "$RUNNER_OS_CONTEXT")"
  runner_arch="$(normalize_arch "$RUNNER_ARCH_CONTEXT")"
  binary_name="$(spx_binary_name_for_platform "$target_platform")"
  package_path="$(normalize_path "$PACKAGE_FILE")"
  if [ ! -f "$package_path" ]; then
    log_error "Package file not found: $PACKAGE_FILE"
    return 1
  fi

  verify_dir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/spx-release-verify.XXXXXX")"
  trap 'rm -rf "$verify_dir"' EXIT
  log_section "Verify standalone package: $target_platform/$target_arch"
  unzip -q "$package_path" -d "$verify_dir/package"
  spx="$verify_dir/package/$binary_name"
  test -f "$spx"
  assert_binary_arch "$spx" "$target_platform" "$target_arch"
  chmod +x "$spx"
  if [ "$target_platform" != "$runner_platform" ] || [ "$target_arch" != "$runner_arch" ]; then
    log_info "Skipping smoke test on non-native runner"
    log_success "Release package verification passed"
    return
  fi

  # Copy only the executable into a fresh directory. Archive siblings and the
  # original installation must not satisfy standalone runtime lookup.
  mkdir -p "$verify_dir/standalone-bin" "$verify_dir/test"
  cp "$spx" "$verify_dir/standalone-bin/$binary_name"
  spx="$verify_dir/standalone-bin/$binary_name"
  cp -R test/CI "$verify_dir/test/"
  project="$verify_dir/test/CI"
  rm -rf "$project/.godot" "$project/go"
  rm -f "$project/go.mod" "$project/go.sum" "$project/project.godot" "$project/xgo_autogen.go"
  spx_version="$(go run ./.github/cmd/runtime-version --spx-version)"
  source_dir="$workspace"
  if command -v cygpath >/dev/null 2>&1; then
    source_dir="$(cygpath -m "$source_dir")"
  fi
  cat > "$project/go.mod" <<EOF
module github.com/goplus/spx-release-smoke-test

go 1.25.0

require github.com/goplus/spx/v4 $spx_version //xgo:class

replace github.com/goplus/spx/v4 => "$source_dir"
EOF
  run_smoke_test "$spx" "$project" "$verify_dir/smoke.log" "$runner_platform" "$verify_dir/isolated"
  log_success "Release package verification passed"
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
