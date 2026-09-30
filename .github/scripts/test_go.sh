#!/usr/bin/env bash
# Run both Go modules with the same package-selection policy in local and CI runs.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.."

# Keep discovery separate so an error cannot silently run a partial package list.
packages="$(go list ./...)"
test_packages=()
while IFS= read -r package; do
  case "$package" in
    ""|*/internal/webffi*) continue ;;
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
