#!/usr/bin/env bash
# Run both modules; --cover instruments the selected packages across their tests.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.."

module_path=github.com/goplus/spx/v4
cover_packages=false
if [ "${1:-}" = --cover ]; then
  cover_packages=true
  shift
fi

test_module() {
  local packages package coverpkg
  local test_packages=()
  # Keep discovery separate so an error cannot silently run a partial package list.
  packages="$(go list ./...)"
  while IFS= read -r package; do
    # A trailing slash matches both a package tree's root and its children.
    case "$package/" in
      # Exclude examples and generated bindings (covered through pkg/ispx).
      /|\
      "$module_path"/test/*|"$module_path"/tutorial/*|\
      "$module_path"/pkg/ispx/internal/pkg/*) continue ;;
    esac
    test_packages+=("$package")
  done <<< "$packages"

  if [ "${#test_packages[@]}" -eq 0 ]; then
    echo "No Go packages selected for testing" >&2
    exit 1
  fi

  if [ "$cover_packages" = true ]; then
    # Reuse the test selection for instrumentation; dependencies still execute.
    coverpkg=$(IFS=,; printf '%s' "${test_packages[*]}")
    set -- "-coverpkg=$coverpkg" "$@"
  fi
  go test "$@" "${test_packages[@]}"
}

test_module "$@"
cd cmd/ispx
test_module "$@"
