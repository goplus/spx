#!/bin/bash

_macos_go_toolchain_error() {
    echo "[error] $*" >&2
}

_macos_go_toolchain_host_goos() {
    local host_goos
    if ! host_goos="$(go env GOHOSTOS)"; then
        _macos_go_toolchain_error "go env GOHOSTOS failed while configuring the macOS toolchain"
        return 1
    fi
    printf '%s\n' "$host_goos"
}

_macos_go_toolchain_xcrun_path() {
    local description="$1"
    shift

    local resolved
    if ! resolved="$(unset SDKROOT; xcrun --sdk macosx "$@")"; then
        _macos_go_toolchain_error "xcrun could not resolve the macOS ${description}"
        return 1
    fi
    case "$resolved" in
        ''|*$'\n'*|*$'\r'*)
            _macos_go_toolchain_error "xcrun returned an invalid macOS ${description}: ${resolved:-<empty>}"
            return 1
            ;;
        /*) ;;
        *)
            _macos_go_toolchain_error "xcrun returned a non-absolute macOS ${description}: $resolved"
            return 1
            ;;
    esac
    printf '%s\n' "$resolved"
}

_macos_go_toolchain_sdk_is_usable() {
    case "$1" in
        /*) [ -d "$1" ] ;;
        *) return 1 ;;
    esac
}

_macos_go_toolchain_command_is_usable() {
    local command_value="$1"
    if [ -z "$command_value" ]; then
        return 1
    fi

    # Go accepts a compiler wrapper followed by arguments. Validate its first
    # executable without evaluating the inherited command text.
    case "$command_value" in
        *[[:space:]]*) command_value="${command_value%%[[:space:]]*}" ;;
    esac
    local resolved_command
    case "$command_value" in
        /*) [ -f "$command_value" ] && [ -x "$command_value" ] ;;
        */*) return 1 ;;
        *)
            resolved_command="$(command -v -- "$command_value" 2>/dev/null)" || return 1
            [ -f "$resolved_command" ] && [ -x "$resolved_command" ]
            ;;
    esac
}

configure_macos_go_toolchain() {
    local host_goos
    host_goos="$(_macos_go_toolchain_host_goos)" || return 1
    if [ "$host_goos" != "darwin" ]; then
        return 0
    fi

    local sdkroot="${SDKROOT:-}"
    if ! _macos_go_toolchain_sdk_is_usable "$sdkroot"; then
        sdkroot="$(_macos_go_toolchain_xcrun_path "SDK" --show-sdk-path)" || return 1
    fi
    if ! _macos_go_toolchain_sdk_is_usable "$sdkroot"; then
        _macos_go_toolchain_error "xcrun selected a missing macOS SDK: $sdkroot"
        return 1
    fi

    # Use Apple's SDK-aware compilers when inherited CC/CXX are absent or point
    # at tools removed by a Homebrew/Xcode upgrade.
    local cc="${CC:-}"
    local cxx="${CXX:-}"
    if ! _macos_go_toolchain_command_is_usable "$cc"; then
        cc="$(_macos_go_toolchain_xcrun_path "C compiler" --find clang)" || return 1
        if [ ! -f "$cc" ] || [ ! -x "$cc" ]; then
            _macos_go_toolchain_error "xcrun selected a missing macOS C compiler: $cc"
            return 1
        fi
    fi
    if ! _macos_go_toolchain_command_is_usable "$cxx"; then
        cxx="$(_macos_go_toolchain_xcrun_path "C++ compiler" --find clang++)" || return 1
        if [ ! -f "$cxx" ] || [ ! -x "$cxx" ]; then
            _macos_go_toolchain_error "xcrun selected a missing macOS C++ compiler: $cxx"
            return 1
        fi
    fi

    export SDKROOT="$sdkroot"
    export CC="$cc"
    export CXX="$cxx"
}

# Buildctl itself does not consume project-specific CGO flags. On Darwin, keep
# stale SDK references in those flags away from the bootstrap compiler without
# mutating the caller's environment. Buildctl normalizes the original flags for
# its child commands after it starts.
macos_go_toolchain_go_build() {
    local host_goos
    host_goos="$(_macos_go_toolchain_host_goos)" || return 1
    if [ "$host_goos" != "darwin" ]; then
        command go build "$@"
        return
    fi

    (
        unset CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS
        command go build "$@"
    )
}
