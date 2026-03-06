#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# Default project directory
DEFAULT_PROJ_DIR=./tutorial/01_aircraft/

# Parse command line arguments
while getopts "p:" opt; do
    case $opt in
        p) PROJ_DIR="$OPTARG";;
        \?) echo "Invalid option: -$OPTARG" >&2; exit 1;;
    esac
done

# Use default if not specified
PROJ_DIR=${PROJ_DIR:-$DEFAULT_PROJ_DIR}

# Convert PROJ_DIR to absolute path
PROJ_DIR="$(cd "$PROJ_DIR" && pwd)"

# Check if ANDROID_NDK_ROOT is set
if [ -z "$ANDROID_NDK_ROOT" ]; then
    echo "Error: ANDROID_NDK_ROOT environment variable is not set"
    echo "Please set it to your Android NDK installation path"
    exit 1
fi

LIB_DIR=$PROJ_DIR/lib
GO_DIR=$PROJ_DIR/go
cd $GO_DIR

# Set common variables
# Detect system architecture and OS
OS_NAME="$(uname -s)"
ARCH="$(uname -m)"

if [[ "$OS_NAME" =~ "MINGW"|"MSYS"|"CYGWIN" ]]; then
    # Windows environment (MSYS2/MinGW/Cygwin)
    case "$ARCH" in
        x86_64|amd64) HOST_TAG="windows-x86_64";;
        *)            echo "Unsupported Windows architecture: $ARCH"; exit 1;;
    esac
elif [[ "$OS_NAME" == "Linux" ]]; then
    case "$ARCH" in
        x86_64)  HOST_TAG="linux-x86_64";;
        aarch64) HOST_TAG="linux-aarch64";;
        *)       echo "Unsupported Linux architecture: $ARCH"; exit 1;;
    esac
elif [[ "$OS_NAME" == "Darwin" ]]; then
    case "$ARCH" in
        x86_64)  HOST_TAG="darwin-x86_64";;
        arm64)   HOST_TAG="darwin-aarch64";;
        *)       echo "Unsupported macOS architecture: $ARCH"; exit 1;;
    esac
else
    echo "Unsupported operating system: $OS_NAME"
    exit 1
fi

NDK_TOOLCHAIN="$ANDROID_NDK_ROOT/toolchains/llvm/prebuilt/$HOST_TAG/bin"
MIN_SDK=21

resolve_godot_binary() {
    if [ -n "$GODOT_BIN" ] && [ -f "$GODOT_BIN" ]; then
        echo "$GODOT_BIN"
        return 0
    fi

    local repo_dir
    repo_dir="$(pwd)"
    local godot_src="${GODOT_SRC:-$repo_dir/godot}"
    if command -v realpath >/dev/null 2>&1; then
        godot_src="$(realpath "$godot_src" 2>/dev/null || echo "$godot_src")"
    fi

    local version=""
    if [ -f "$repo_dir/cmd/gox/template/version" ]; then
        version="$(cat "$repo_dir/cmd/gox/template/version")"
    fi

    local gopath_bin=""
    if command -v go >/dev/null 2>&1; then
        gopath_bin="$(go env GOPATH 2>/dev/null)/bin"
    fi

    local candidates=()

    case "$OS_NAME" in
        Darwin)
            if [ "$ARCH" = "arm64" ]; then
                candidates+=("$godot_src/bin/godot.macos.editor.dev.arm64")
            fi
            candidates+=(
                "$godot_src/bin/godot.macos.editor.dev.x86_64"
                "$godot_src/bin/godot.macos.editor.dev"
            )
            [ -n "$version" ] && candidates+=("$gopath_bin/gdspx$version")
            ;;
        Linux)
            candidates+=("$godot_src/bin/godot.linuxbsd.editor.dev.$ARCH")
            [ -n "$version" ] && candidates+=("$gopath_bin/gdspx$version")
            ;;
        *)
            candidates+=("$godot_src/bin/godot.windows.editor.dev.x86_64.exe")
            [ -n "$version" ] && candidates+=("$gopath_bin/gdspx$version.exe")
            ;;
    esac

    for candidate in "${candidates[@]}"; do
        if [ -f "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

echo "Building for arm64-v8a..."
CGO_ENABLED=1 \
GOOS=android \
GOARCH=arm64 \
CC="$NDK_TOOLCHAIN/aarch64-linux-android$MIN_SDK-clang" \
go build -buildmode=c-shared -o $LIB_DIR/libgdspx-android-arm64.so main.go

echo "Building for armeabi-v7a..."
CGO_ENABLED=1 \
GOOS=android \
GOARCH=arm \
CC="$NDK_TOOLCHAIN/armv7a-linux-androideabi$MIN_SDK-clang" \
go build -buildmode=c-shared -o $LIB_DIR/libgdspx-android-arm32.so main.go

echo "Build android so completed successfully!"

# Paths
GODOT_BINARY="$(resolve_godot_binary)"
if [ -z "$GODOT_BINARY" ] || [ ! -f "$GODOT_BINARY" ]; then
    echo "Error: Unable to locate a Godot editor binary"
    echo "Supported resolution order:"
    echo "1. GODOT_BIN=/abs/path/to/godot-binary"
    echo "2. GODOT_SRC=/abs/path/to/godot (expects built editor under bin/)"
    echo "3. GOPATH/bin/gdspx<version> after running make build-editor"
    exit 1
fi

PROJECT_PATH="$PROJ_DIR/project.godot"
APK_PATH="$PROJ_DIR/builds/test.apk"
BUILD_DIR=$(dirname "$APK_PATH")

# Create builds directory if it doesn't exist
mkdir -p "$BUILD_DIR"

# Check if Godot binary exists
if [ ! -f "$GODOT_BINARY" ]; then
    echo "Error: Godot binary not found at $GODOT_BINARY"
    exit 1
fi

# Check if project file exists
if [ ! -f "$PROJECT_PATH" ]; then
    echo "Error: Godot project file not found at $PROJECT_PATH"
    exit 1
fi

# Import project to ensure resources are up to date
echo "Importing project resources..."
"$GODOT_BINARY" --headless --path "$(dirname "$PROJECT_PATH")" --editor --quit

# Export the project to APK
echo "Exporting Godot project to APK..."
"$GODOT_BINARY" --headless --path "$(dirname "$PROJECT_PATH")" --export-debug "Android" "$APK_PATH"

if [ ! -f "$APK_PATH" ]; then
    echo "Error: APK export failed"
    exit 1
fi

# Check if adb is available
if ! command -v adb &> /dev/null; then
    echo "Error: adb command not found. Please ensure Android SDK platform tools are installed and in your PATH"
    exit 1
fi

# Check if any Android device is connected
if ! adb devices | grep -q "device$"; then
    echo "Error: No Android device connected. Please connect a device and enable USB debugging"
    exit 1
fi

echo "Installing APK..."
adb install -r "$APK_PATH"

if [ $? -eq 0 ]; then
    echo "APK installation successful!"
else
    echo "Error: APK installation failed"
    exit 1
fi

cd -
