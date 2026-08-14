#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <spx-web-zip> <version> <package-dir>" >&2
  exit 1
fi

zip_path="$1"
version="${2#v}"
package_dir="$3"

case "${package_dir}" in
  "" | "." | "/" | *"/.." | *"/../"* | "../"*)
    echo "unsafe package dir: ${package_dir}" >&2
    exit 1
    ;;
esac

if [[ ! -f "${zip_path}" ]]; then
  echo "spx web zip not found: ${zip_path}" >&2
  exit 1
fi

rm -rf -- "${package_dir}"
mkdir -p -- "${package_dir}"
unzip -q "${zip_path}" -d "${package_dir}"

cat >"${package_dir}/package.json" <<EOF
{
  "name": "@xgo-pkgs/spx",
  "version": "${version}",
  "description": "SPX web runtime bundle.",
  "license": "Apache-2.0",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/goplus/spx.git"
  },
  "homepage": "https://github.com/goplus/spx#readme",
  "bugs": {
    "url": "https://github.com/goplus/spx/issues"
  },
  "files": [
    "**/*"
  ]
}
EOF

cat >"${package_dir}/README.md" <<EOF
# @xgo-pkgs/spx

SPX web runtime bundle for browser-based runners.

This package is generated from \`spx_web.zip\` in the SPX release pipeline.
EOF

npm pack "${package_dir}" --dry-run
