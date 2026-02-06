#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
DIST_DIR="$PROJECT_ROOT/dist"

MODE="${1:-normal}"
case "$MODE" in
  normal|worker|minigame|miniprogram)
    ;;
  *)
    echo "Error: invalid MODE '$MODE'. Supported: normal, worker, minigame, miniprogram"
    exit 1
    ;;
esac

SPX_BIN="$DIST_DIR/bin/spx"
if [ "$OS" = "Windows_NT" ]; then
  SPX_BIN="$DIST_DIR/bin/spx.exe"
fi

if [ ! -x "$SPX_BIN" ]; then
  echo "Error: spx not found at $SPX_BIN"
  echo "Run 'make build' first"
  exit 1
fi

if [ ! -d "$DIST_DIR/share/engines" ] || [ -z "$(ls -A "$DIST_DIR/share/engines" 2>/dev/null)" ]; then
  echo "Error: engines not found at $DIST_DIR/share/engines"
  echo "Run 'make setup-engines' first"
  exit 1
fi

export DIST_ENGINES_DIR="$DIST_DIR/share/engines"
export DIST_TEMPLATES_DIR="$DIST_DIR/share/templates"
export SPX_BIN

mkdir -p "$DIST_TEMPLATES_DIR/$MODE"

cd "$PROJECT_ROOT/pkg/gdspx/tools"

echo "Downloading web engine template (mode: $MODE)"
export INSTALL_TEMPLATES=true
./build_engine.sh -g -p web -m "$MODE"

echo "Generating web template (mode: $MODE)"
./make_util.sh extrawebtemplate "$MODE"

echo "Web template setup completed: $DIST_TEMPLATES_DIR/$MODE"
