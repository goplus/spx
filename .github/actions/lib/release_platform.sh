#!/usr/bin/env bash

log_section() {
  printf '\n== %s ==\n' "$1"
}

log_info() {
  printf '[info] %s\n' "$1"
}

log_success() {
  printf '[ok] %s\n' "$1"
}

log_warn() {
  printf '[warn] %s\n' "$1"
}

log_error() {
  printf '[error] %s\n' "$1" >&2
}

normalize_platform() {
  case "$1" in
    darwin|macos|macOS)
      printf 'macos\n'
      ;;
    linux|Linux)
      printf 'linux\n'
      ;;
    windows|Windows)
      printf 'windows\n'
      ;;
    *)
      printf '%s\n' "$1" | tr '[:upper:]' '[:lower:]'
      ;;
  esac
}

normalize_arch() {
  case "$1" in
    X64|x64|amd64|x86_64)
      printf 'x64\n'
      ;;
    X86|x86|386|i386|i686)
      printf 'x86\n'
      ;;
    ARM64|arm64|aarch64)
      printf 'arm64\n'
      ;;
    *)
      printf '%s\n' "$1" | tr '[:upper:]' '[:lower:]'
      ;;
  esac
}

normalize_path() {
  local raw_path="$1"
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -u "$raw_path" 2>/dev/null || printf '%s\n' "$raw_path"
    return
  fi
  printf '%s\n' "$raw_path"
}

normalize_windows_path() {
  local raw_path="$1"
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$raw_path" 2>/dev/null || printf '%s\n' "$raw_path"
    return
  fi
  printf '%s\n' "$raw_path"
}

go_env_goexe() {
  go env GOEXE | tr -d '\r'
}

go_env_gopath_first() {
  local goexe="$1"
  local gopath_raw
  local gopath_sep
  local gopath_first

  gopath_raw="$(go env GOPATH | tr -d '\r')"
  if [ -z "$gopath_raw" ]; then
    log_error "GOPATH is empty"
    return 1
  fi

  gopath_sep=':'
  if [ "$goexe" = ".exe" ]; then
    gopath_sep=';'
  fi

  gopath_first="${gopath_raw%%${gopath_sep}*}"
  normalize_path "$gopath_first"
}

go_env_bin_dir() {
  local goexe="$1"
  local gopath_first

  gopath_first="$(go_env_gopath_first "$goexe")"
  printf '%s/bin\n' "$gopath_first"
}

go_env_spx_path() {
  local goexe="$1"
  local go_bin_dir

  go_bin_dir="$(go_env_bin_dir "$goexe")"
  printf '%s/spx%s\n' "$go_bin_dir" "$goexe"
}

spx_binary_name_for_platform() {
  case "$1" in
    windows)
      printf 'spx.exe\n'
      ;;
    macos|linux)
      printf 'spx\n'
      ;;
    *)
      log_error "unsupported platform: $1"
      return 1
      ;;
  esac
}

spx_smoke_reached_success() {
  local log_path="$1"
  grep -Eq 'Spx(CI)?RunSucc' "$log_path"
}

should_tolerate_windows_gdspxrt_shutdown_exit() {
  local runner_platform="$1"
  local log_path="$2"

  [ "$runner_platform" = "windows" ] && spx_smoke_reached_success "$log_path"
}
