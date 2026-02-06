#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJ_DIR="$SCRIPT_DIR/../../.."
DIST_DIR="$PROJ_DIR/dist"
DIST_ENGINES_DIR="${DIST_ENGINES_DIR:-$DIST_DIR/share/engines}"
DIST_TEMPLATES_DIR="${DIST_TEMPLATES_DIR:-$DIST_DIR/share/templates}"

mkdir -p "$DIST_TEMPLATES_DIR"

ensure_spx_bin() {
    if [ -z "$SPX_BIN" ]; then
        echo "Error: SPX_BIN is not set"
        echo "Run this script via 'make setup-web' or set SPX_BIN manually"
        exit 1
    fi
    if [ ! -x "$SPX_BIN" ]; then
        echo "Error: SPX_BIN is not executable: $SPX_BIN"
        exit 1
    fi
}

# Create a temporary project for export commands
prepare_export_env() {
    rm -rf "$PROJ_DIR/.tmp/web"
    mkdir -p "$PROJ_DIR/.tmp/web/assets"
    echo '{"map":{"width":480,"height":360}}' > "$PROJ_DIR/.tmp/web/assets/index.json"
    echo "" > "$PROJ_DIR/.tmp/web/main.spx"
    rm -rf "$PROJ_DIR/.tmp/web/project/.builds"/*web

    PCK_VERSION=$(cat "$PROJ_DIR/cmd/gox/template/pck_version")
}

do_exportweb() {
    ensure_spx_bin
    local mode="${1:-normal}"

    if [ "$mode" != "normal" ] && [ "$mode" != "worker" ] && [ "$mode" != "minigame" ] && [ "$mode" != "miniprogram" ]; then
        echo "Error: Invalid mode '$mode'. Supported modes: normal, worker, minigame, miniprogram"
        return 1
    fi

    local spx_cmd="exportweb"
    local output_zip="spx-web.zip"
    case "$mode" in
        normal)
            spx_cmd="exportweb"
            output_zip="spx-web.zip"
            ;;
        worker)
            spx_cmd="exportwebworker"
            output_zip="spx-web-worker.zip"
            ;;
        minigame)
            spx_cmd="exportminigame"
            output_zip="spx-web-minigame.zip"
            ;;
        miniprogram)
            spx_cmd="exportminiprogram"
            output_zip="spx-web-miniprogram.zip"
            ;;
    esac

    prepare_export_env

    (cd "$PROJ_DIR/.tmp/web"
     "$SPX_BIN" $spx_cmd
     cd ./project/.builds/web
     rm -f game.zip
     zip -r "$PROJ_DIR/$output_zip" *
     echo "$PROJ_DIR/$output_zip has been created") || {
        echo "Error: Failed to create web export (mode: $mode)"
        return 1
    }

    rm -rf "$PROJ_DIR/.tmp"
    echo "exportweb (mode: $mode) completed successfully"
    return 0
}

do_extra_webtemplate() {
    ensure_spx_bin
    local mode="${1:-normal}"

    prepare_export_env

    mkdir -p "$DIST_TEMPLATES_DIR/$mode"
    local dstdir="$DIST_TEMPLATES_DIR/$mode"

    echo "exporting web runtime... $mode"
    (cd "$PROJ_DIR/.tmp/web" && "$SPX_BIN" exporttemplateweb)

    rm -rf "$dstdir"
    cp -rf "$PROJ_DIR/.tmp/web/project/.builds/webi" "$dstdir"
    mv "$dstdir/engine.pck" "$dstdir/engine.zip"

    engine_mode_define="var EnginePackMode = '$mode';"
    temp_file=$(mktemp)
    echo "$engine_mode_define" > "$temp_file"
    cat "$dstdir/engine.js" >> "$temp_file"
    mv "$temp_file" "$dstdir/engine.js"

    echo "exporting web runtime done: $dstdir (mode: $mode)"
    rm -rf "$PROJ_DIR/.tmp"
    return 0
}

do_exportpack() {
    ensure_spx_bin
    prepare_export_env

    echo "Starting exportpack..."
    echo "exporting pck..."
    (cd "$PROJ_DIR/.tmp/web" && "$SPX_BIN" export)

    mkdir -p "$DIST_ENGINES_DIR"
    OUTPUT_PCK="$DIST_ENGINES_DIR/gdspxrt$PCK_VERSION.pck"
    RUNTIME_GDEXT="$DIST_DIR/share/runtime.gdextension"

    if [ -f "$PROJ_DIR/.tmp/web/project/.builds/pc/gdexport.pck" ]; then
        cp "$PROJ_DIR/.tmp/web/project/.builds/pc/gdexport.pck" "$OUTPUT_PCK"
    fi

    if [ -d "$PROJ_DIR/.tmp/web/project/.builds/pc/gdexport.app/Contents/Resources" ] && \
       [ "$(ls -A "$PROJ_DIR/.tmp/web/project/.builds/pc/gdexport.app/Contents/Resources"/*.pck 2>/dev/null)" ]; then
        cp "$PROJ_DIR/.tmp/web/project/.builds/pc/gdexport.app/Contents/Resources"/*.pck "$OUTPUT_PCK"
    fi

    if [ ! -f "$OUTPUT_PCK" ]; then
        echo "Error: $OUTPUT_PCK does not exist"
        return 1
    fi
    if [ ! -f "$RUNTIME_GDEXT" ]; then
        echo "Error: $RUNTIME_GDEXT does not exist"
        return 1
    fi

    TEMP_PCK="$DIST_ENGINES_DIR/gdspxrt.pck"
    DST_ZIP="$DIST_ENGINES_DIR/gdspxrt.pck.$PCK_VERSION.zip"

    cp "$OUTPUT_PCK" "$TEMP_PCK"
    rm -f "$DST_ZIP"
    zip -j "$DST_ZIP" "$TEMP_PCK" "$RUNTIME_GDEXT"
    rm -f "$TEMP_PCK"
    rm -rf "$PROJ_DIR/.tmp"
    return 0
}

main() {
    if [ $# -eq 0 ]; then
        echo "Usage: $0 [command] [options]"
        echo "Commands:"
        echo "  exportweb [mode] - Create a web release package (mode: normal|worker|minigame|miniprogram, default: normal)"
        echo "  exportpack - Set up and package the application"
        echo "  extrawebtemplate [mode] - Export web runtime template (mode: worker|minigame|miniprogram|normal)"
        return 1
    fi

    command="$1"
    shift

    case "$command" in
        exportweb)
            mode="$1"
            do_exportweb "$mode"
            ;;
        exportpack)
            do_exportpack
            ;;
        extrawebtemplate)
            mode="$1"
            do_extra_webtemplate "$mode"
            ;;
        *)
            echo "Unknown command: $command"
            echo "Available commands: exportweb [mode], exportpack, extrawebtemplate"
            return 1
            ;;
    esac
}

main "$@"
