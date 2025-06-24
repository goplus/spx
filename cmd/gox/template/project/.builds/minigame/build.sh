#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# copy version file
cd $SCRIPT_DIR

echo "compress wasm file to .br format..."

# check brotli is installed
if ! command -v brotli &> /dev/null; then
    echo "error: brotli is not installed"
    exit 1
fi


GODOT_EDITOR_WASM="minigame/engine.wasm"
echo "compress $GODOT_EDITOR_WASM..."
brotli -f -q 11 "$GODOT_EDITOR_WASM"

GDSPX_WASM="minigame/gdspx.wasm"
echo "compress $GDSPX_WASM..."
brotli -f -q 11 "$GDSPX_WASM"


# move files to engine dir
mv -f minigame/*.zip engine/
mv -f minigame/*.br engine/

# move js files to js dir
mv -f minigame/*.js js/


# merge js files
cp -f js/header.js js/engine_new.js
cat js/engine.js >> js/engine_new.js
cat js/wasm_exec.js >> js/engine_new.js
cat js/game.js >> js/engine_new.js

rm -f js/header.js
rm -f js/engine.js
rm -f js/wasm_exec.js
rm -f js/game.js

mv -f js/engine_new.js js/engine.js


# remove minigame dir
rm -rf minigame

if [ -n "$WECHAT_DEV_TOOLS" ]; then
    $WECHAT_DEV_TOOLS/cli open --project "$SCRIPT_DIR" -y
else
    echo "WECHAT_DEV_TOOLS is not set,  please open project manually $SCRIPT_DIR"
fi