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
      log_error "Unsupported platform: $1"
      return 1
      ;;
  esac
}

assert_macos_binary_arch() {
  local binary_path="$1"
  local expected_arch="$2"
  local actual_archs

  if ! command -v lipo >/dev/null 2>&1; then
    log_error "Lipo command is required to verify macOS binaries"
    exit 1
  fi

  actual_archs="$(lipo -archs "$binary_path" 2>/dev/null | tr -s '[:space:]' ' ' | sed -e 's/^ //' -e 's/ $//')"
  if [ -z "$actual_archs" ]; then
    log_error "Failed to inspect macOS binary architecture: $binary_path"
    exit 1
  fi

  case " $actual_archs " in
    *" $expected_arch "*)
      log_success "Binary architecture matches macOS target: $actual_archs"
      ;;
    *)
      log_error "Binary architecture mismatch for macOS (expected $expected_arch, got $actual_archs)"
      exit 1
      ;;
  esac
}

assert_binary_arch() {
  local binary_path="$1"
  local platform="$2"
  local arch="$3"
  local description

  if ! command -v file >/dev/null 2>&1; then
    log_error "File command is required to verify binary architecture"
    exit 1
  fi

  description="$(file "$binary_path")"
  log_info "Binary architecture: $description"

  case "$platform/$arch" in
    linux/x64)
      case "$description" in
        *"ELF 64-bit"*"x86-64"*) return ;;
      esac
      ;;
    linux/x86)
      case "$description" in
        *"ELF 32-bit"*"Intel 80386"*) return ;;
      esac
      ;;
    linux/arm64)
      case "$description" in
        *"ELF 64-bit"*"ARM aarch64"*) return ;;
      esac
      ;;
    windows/x64)
      case "$description" in
        *"PE32+ executable"*"x86-64"*) return ;;
      esac
      ;;
    windows/x86)
      case "$description" in
        *"PE32 executable"*"Intel 80386"*) return ;;
      esac
      ;;
    macos/x64)
      assert_macos_binary_arch "$binary_path" "x86_64"
      return
      ;;
    macos/arm64)
      assert_macos_binary_arch "$binary_path" "arm64"
      return
      ;;
    *)
      log_error "Unsupported binary architecture target: $platform/$arch"
      exit 1
      ;;
  esac

  log_error "Binary architecture mismatch for $platform/$arch"
  exit 1
}

spx_run_reached_success() {
  local log_path="$1"
  grep -Fq 'SPX_CI_TEST_OK' "$log_path"
}

should_tolerate_windows_gdspxrt_shutdown_exit() {
  local runner_platform="$1"
  local log_path="$2"

  [ "$runner_platform" = "windows" ] && spx_run_reached_success "$log_path"
}
