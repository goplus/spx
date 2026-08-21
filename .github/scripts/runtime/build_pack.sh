#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: build_pack.sh <engine-asset-directory> <output-zip>" >&2
  exit 2
fi

engine_asset_directory="$1"
output_zip="$2"

if [ -z "${BUILDCTL:-}" ] || [ ! -x "$BUILDCTL" ]; then
  echo "[error] BUILDCTL must name an executable buildctl binary" >&2
  exit 1
fi
if [ ! -d "$engine_asset_directory" ]; then
  echo "[error] Engine asset directory not found: $engine_asset_directory" >&2
  exit 1
fi

"$BUILDCTL" runtime export-pack \
  --engine-asset-dir "$engine_asset_directory"

gopath="$(go env GOPATH | tr -d '\r\n')"
gopath="${gopath%%:*}"
runtime_pack="$gopath/bin/spx-runtime-assets.zip"
if [ ! -s "$runtime_pack" ]; then
  echo "[error] Runtime asset bundle not found: $runtime_pack" >&2
  exit 1
fi

mkdir -p "$(dirname "$output_zip")"
cp "$runtime_pack" "$output_zip"
unzip -t "$output_zip"
echo "[info] Built runtime asset bundle: $output_zip"
