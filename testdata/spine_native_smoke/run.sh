#!/usr/bin/env bash
# Linux amd64 smoke: build a real Go extension and run it in a matching engine.
set -euo pipefail
fixture=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$fixture/../.." && pwd)
engine=${1:?usage: run.sh /absolute/path/to/godot [enabled|disabled] [project|export]}
mode=${2:-enabled}
layout=${3:-project}
case "$mode:$layout" in enabled:project|disabled:project|enabled:export) ;; *) echo "unsupported mode/layout: $mode/$layout" >&2; exit 2 ;; esac
work=$(mktemp -d "${TMPDIR:-/tmp}/spx-spine-smoke.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/lib" "$work/.godot" "$work/cache" "$work/data" "$work/config"
export XDG_CACHE_HOME="$work/cache" XDG_DATA_HOME="$work/data" XDG_CONFIG_HOME="$work/config"
cp -R "$fixture/assets" "$work/"
cp -R "$repo/cmd/spx/template/project/engine" "$work/"
cp "$repo/cmd/spx/template/project/"{project.godot,main.tscn,export_presets.cfg} "$work/"
# Import before installing the extension, so editor startup cannot start the game.
if [[ "$mode" == enabled ]]; then
 if ! timeout 90 "$engine" --headless --path "$work" --editor --import >"$work/import.log" 2>&1; then
  cat "$work/import.log"; exit 1
 fi
fi
cp "$repo/cmd/spx/template/project/gdspx.gdextension" "$work/"
printf 'res://gdspx.gdextension\n' > "$work/.godot/extension_list.cfg"
(cd "$repo" && CGO_ENABLED=1 go build -tags=packmode -buildmode=c-shared -o "$work/lib/gdspx-linux-amd64.so" ./testdata/spine_native_smoke)
run_dir="$work"
engine_args=(--headless --path "$work")
if [[ "$layout" == export ]]; then
 run_dir="$work/exported"
 mkdir -p "$run_dir/lib"
 if ! timeout 90 "$engine" --headless --quit --path "$work" --export-pack Linux "$run_dir/game.pck" >"$work/export.log" 2>&1; then
  cat "$work/export.log"; exit 1
 fi
 # Desktop payloads retain external configuration and the platform shared library.
 cp -R "$fixture/assets" "$run_dir/"
 # Require native resources to come from the exported pack, not raw sidecars.
 rm "$run_dir/assets/sprites/Quad/quad.atlas" "$run_dir/assets/sprites/Quad/quad.json" "$run_dir/assets/sprites/Quad/quad.png"
 cp "$work/lib/gdspx-linux-amd64.so" "$run_dir/lib/"
 engine_args=(--headless --path "$run_dir" --main-pack "$run_dir/game.pck")
fi
set +e
(cd "$run_dir" && timeout 30 "$engine" "${engine_args[@]}") >"$work/run.log" 2>&1
status=$?
set -e
cat "$work/run.log"
if [[ "$mode" == enabled ]]; then
 [[ $status == 0 ]] && grep -q '^SPX_SPINE_NATIVE_OK$' "$work/run.log"
else
 [[ $status != 0 && $status != 124 ]] && grep -q 'Spine support is disabled' "$work/run.log" && ! grep -q '^SPX_SPINE_NATIVE_OK$' "$work/run.log"
fi
