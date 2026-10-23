#!/usr/bin/env bash
# Run the root and base interpreter modules in local and CI runs.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.."

module_path=github.com/goplus/spx/v4

# Keep discovery separate so an error cannot silently run a partial package list.
packages="$(go list ./...)"
test_packages=()
while IFS= read -r package; do
  # A trailing slash matches both a package tree's root and its children.
  case "$package/" in
    # Exclude examples and generated bindings (covered through pkg/ispx).
    /|*/internal/webffi*|\
    "$module_path"/test/*|"$module_path"/tutorial/*|\
    "$module_path"/pkg/ispx/internal/pkg/*) continue ;;
  esac
  test_packages+=("$package")
done <<< "$packages"

if [ "${#test_packages[@]}" -eq 0 ]; then
  echo "No Go packages selected for testing" >&2
  exit 1
fi

go test "$@" "${test_packages[@]}"
cd cmd/ispx
exec go test "$@" ./...
