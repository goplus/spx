#!/usr/bin/env bash
# Generate and check committed bindings before tests create unrelated outputs.
set -euo pipefail

check_export_types=true
if [ "$#" -ne 0 ]; then
  if [ "$#" -ne 2 ] || [ "$1" != --check-export-types ]; then
    echo "usage: $0 [--check-export-types true|false]" >&2
    exit 2
  fi
  check_export_types="$2"
fi
case "$check_export_types" in
  true|false) ;;
  *) echo "check-export-types must be true or false" >&2; exit 2 ;;
esac

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.."
repo_root="$PWD"
type_snapshot=pkg/ispx/internal/pkg/github.com/goplus/spx/v4/export.types
# Regular CI restores this file after generation because private implementation
# changes affect it too. Never discard a local edit or a staged snapshot.
if ! git diff --quiet -- "$type_snapshot" || ! git diff --cached --quiet -- "$type_snapshot"; then
  echo "Commit or stash changes to $type_snapshot before checking generated files" >&2
  exit 1
fi

(
  cd internal/cmd/codegen
  SPX_MODULE_SRC="$repo_root/godot_modules/spx" go run .
)
go generate ./cmd/spx/internal/command/...
go generate ./pkg/ispx/...
if [ "$check_export_types" = false ]; then
  git restore -- "$type_snapshot"
fi

git diff --exit-code

# Include untracked outputs: git diff alone misses newly generated files.
# Keep these roots aligned with codegen and the runtime go:generate commands.
untracked=$(git ls-files --others --exclude-standard -- \
  internal/gdengine internal/enginewrap pkg/spx/pkg/engine \
  godot_modules/spx cmd/spx/template/platform/webworker \
  pkg/ispx/internal/pkg cmd/spx/internal/command/builderai)
if [ -n "$untracked" ]; then
  printf 'Generated files are not tracked:\n%s\n' "$untracked" >&2
  exit 1
fi
