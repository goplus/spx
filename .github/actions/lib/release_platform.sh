#!/usr/bin/env bash

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
