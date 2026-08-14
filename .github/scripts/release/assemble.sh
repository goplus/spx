#!/usr/bin/env bash

set -euo pipefail

mkdir -p artifacts dist/runtime dist/product
declare -A files=()
while IFS= read -r -d '' path; do
  name="$(basename "$path")"
  if [ -n "${files[$name]:-}" ]; then
    echo "[error] Duplicate asset name $name: ${files[$name]} and $path" >&2
    exit 1
  fi
  if [ ! -s "$path" ]; then
    echo "[error] Empty asset: $path" >&2
    exit 1
  fi
  files[$name]="$path"
done < <(find artifacts -type f -print0 | sort -z)

if [ "${#files[@]}" -eq 0 ] && [ "$RUNTIME_STATE" = missing ]; then
  echo "[error] No build artifacts were downloaded" >&2
  exit 1
fi

mapfile -t required_assets < <(jq -r '.required_assets[]' internal/release/runtime.lock.json)
if [ "$RUNTIME_STATE" = ready ]; then
  gh release download "$RUNTIME_TAG" --repo "$REPOSITORY" --dir dist/runtime
  (cd dist/runtime && sha256sum -c SHA256SUMS)
  go run ./.github/scripts/runtime/manifest.go \
    --lock internal/release/runtime.lock.json \
    --verify-manifest "dist/runtime/$RUNTIME_MANIFEST" \
    --asset-directory dist/runtime
  for name in "${required_assets[@]}"; do
    if [ ! -s "dist/runtime/$name" ]; then
      echo "[error] Published $RUNTIME_TAG is missing locked asset: $name" >&2
      exit 1
    fi
  done
else
  for name in "${required_assets[@]}"; do
    path="${files[$name]:-}"
    if [ -z "$path" ]; then
      echo "[error] Missing locked runtime asset: $name" >&2
      exit 1
    fi
    cp "$path" "dist/runtime/$name"
    unset 'files[$name]'
  done
fi

expected_products=()
if [ "$RUN_WEB" = true ]; then
  expected_products+=(spx_web.zip spx_web_worker.zip spx_web_minigame.zip spx_web_miniprogram.zip)
fi
if [ "$RUN_MACOS" = true ]; then
  expected_products+=(spx-standalone-darwin-x64.zip spx-standalone-darwin-arm64.zip)
fi
if [ "$RUN_WINDOWS" = true ]; then
  expected_products+=(spx-standalone-windows-x64.zip)
fi
if [ "$RUN_LINUX" = true ]; then
  expected_products+=(spx-standalone-linux-x64.zip)
fi
for name in "${expected_products[@]}"; do
  path="${files[$name]:-}"
  if [ -z "$path" ]; then
    echo "[error] Missing selected SPX product asset: $name" >&2
    exit 1
  fi
  cp "$path" "dist/product/$name"
  unset 'files[$name]'
done
if [ "${#files[@]}" -ne 0 ]; then
  printf '[error] Unexpected build artifact: %s\n' "${!files[@]}" >&2
  exit 1
fi
product_count="${#expected_products[@]}"
echo "product_count=$product_count" >> "$GITHUB_OUTPUT"

if [ "$RUNTIME_STATE" = missing ]; then
  spx_commit="$(git rev-parse HEAD)"
  if [ -z "$MODULE_PATH" ]; then
    echo "[error] Locked SPX module path must not be empty" >&2
    exit 1
  fi
  module_tree="$(git rev-parse "HEAD:${MODULE_PATH}")"
  runtime_pack_source_sha256="$(go run ./.github/scripts/runtime/digest.go pack-source HEAD)"
  build_recipe_sha256="$(go run ./.github/scripts/runtime/digest.go build-recipe HEAD)"

  manifest_args=()
  for name in "${required_assets[@]}"; do
    manifest_args+=(--asset "$name=dist/runtime/$name")
  done
  go run ./.github/scripts/runtime/manifest.go \
    --lock internal/release/runtime.lock.json \
    --output "dist/runtime/$RUNTIME_MANIFEST" \
    --checksums dist/runtime/SHA256SUMS \
    --spx-commit "$spx_commit" \
    --module-tree "$module_tree" \
    --runtime-pack-source-sha256 "$runtime_pack_source_sha256" \
    --build-recipe-sha256 "$build_recipe_sha256" \
    "${manifest_args[@]}"

  (
    cd dist/runtime
    sha256sum "$RUNTIME_MANIFEST" >> SHA256SUMS
    sort -k2 -o SHA256SUMS SHA256SUMS
  )
  go run ./.github/scripts/runtime/manifest.go \
    --lock internal/release/runtime.lock.json \
    --verify-manifest "dist/runtime/$RUNTIME_MANIFEST" \
    --asset-directory dist/runtime
fi

declare -A expected_runtime_files=()
expected_runtime_files["$RUNTIME_MANIFEST"]=1
expected_runtime_files[SHA256SUMS]=1
for name in "${required_assets[@]}"; do
  expected_runtime_files["$name"]=1
done
while IFS= read -r -d '' path; do
  name="$(basename "$path")"
  if [ -z "${expected_runtime_files[$name]:-}" ]; then
    echo "[error] Unexpected runtime release asset: $name" >&2
    exit 1
  fi
  unset 'expected_runtime_files[$name]'
done < <(find dist/runtime -mindepth 1 -maxdepth 1 -type f -print0 | sort -z)
if [ "${#expected_runtime_files[@]}" -ne 0 ]; then
  printf '[error] Runtime release asset is missing: %s\n' "${!expected_runtime_files[@]}" >&2
  exit 1
fi

if [ "$product_count" -gt 0 ]; then
  (
    cd dist/product
    find . -maxdepth 1 -type f ! -name SHA256SUMS -print0 |
      sort -z |
      xargs -0 sha256sum |
      sed 's#  \./#  #' > SHA256SUMS
  )
fi

echo "[info] Runtime release assets:"
find dist/runtime -maxdepth 1 -type f -printf '%f\n' | sort
echo "[info] SPX product assets:"
find dist/product -maxdepth 1 -type f -printf '%f\n' | sort
