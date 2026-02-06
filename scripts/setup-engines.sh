#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
DIST_DIR="$PROJECT_ROOT/dist"

PLATFORM="$(go env GOOS)"
case "$PLATFORM" in
  darwin)
    PLATFORM="macos"
    ;;
esac

mkdir -p "$DIST_DIR/share/engines"
mkdir -p "$DIST_DIR/share/templates"

export DIST_ENGINES_DIR="$DIST_DIR/share/engines"
export DIST_TEMPLATES_DIR="$DIST_DIR/share/templates"

cd "$PROJECT_ROOT/pkg/gdspx/tools"

echo "Downloading engines for platform: $PLATFORM"
./build_engine.sh -d -p "$PLATFORM"

echo "Engines setup completed."
