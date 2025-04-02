#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJ_DIR=$SCRIPT_DIR/..
cd $PROJ_DIR
GODOT_PATH="$PROJ_DIR/godot"

# Check if podman is available
podman=`command -v podman`
if [ -z "$podman" ]; then
  echo "podman needs to be in PATH for this script to work."
  exit 1
fi

# Ensure the Godot source directory exists
if [ ! -d "$GODOT_PATH" ]; then
  echo "Error: Godot source directory does not exist: $GODOT_PATH"
  exit 1
fi

# Create logs directory if it doesn't exist
mkdir -p $PROJ_DIR/logs

# Function to build Godot engine for a specific platform
build_platform() {
    local godot_path="$GODOT_PATH"
    local platform=$1
    shift 1
    local scons_args=$@
    local img_version="4.x-f36"
    
    echo "Building for platform: $platform"
    echo "Build arguments: $scons_args"
    
    # Validate platform
    case "$platform" in
        windows|linux|web|android|osx|ios)
            ;;
        *)
            echo "Error: Invalid platform '$platform'"
            echo "Supported platforms: windows, linux, web, android, osx, ios"
            exit 1
            ;;
    esac
    
    echo "Starting Godot build..."
    echo "Platform: ${platform}"
    echo "Using image version: ${img_version}"
    echo "Godot source path: ${godot_path}"
    echo "Scons arguments: ${scons_args}"
    echo "----------------------------------------"
    

    "$podman" run -it --rm \
        -v "${godot_path}":/root/godot:z \
        godot-${platform}:${img_version} \
        bash -c "cd /root/godot && scons platform=${platform} ${scons_args}" \
        2>&1 | tee logs/godot_${platform}.log
        
    local build_status=${PIPESTATUS[0]}
    if [ $build_status -ne 0 ]; then
        echo "Error: Build failed for platform $platform with status $build_status"
        exit $build_status
    fi
    
    echo "Build completed successfully for $platform"
    echo "----------------------------------------"
}

echo "Starting multi-platform Godot builds..."
echo "Godot source path: ${GODOT_PATH}"
echo "----------------------------------------"

# ios
IOSP1="IOS_SDK_PATH='/root/ioscross/arm64/SDK/iPhoneOS17.0.sdk'"
IOSP2="IOS_TOOLCHAIN_PATH='/root/ioscross/arm64'"
IOSP3="ios_triple='arm-apple-darwin11-'"
build_platform ios target=template_debug ios_simulator=no $IOSP1 $IOSP2 $IOSP3
build_platform ios target=template_release ios_simulator=no generate_bundle=yes $IOSP1 $IOSP2 $IOSP3
build_platform ios target=template_debug ios_simulator=yes arch=arm64 generate_bundle=yes $IOSP1 $IOSP2 $IOSP3

# android
cd $GODOT_PATH || exit
scons platform=android target=template_debug arch=arm32
scons platform=android target=template_debug arch=arm64
cd platform/android/java || exit
# On Linux and macOS
./gradlew generateGodotTemplates
# osx
build_platform osx osxcross_sdk=darwin23 arch=arm64 vulkan=false target=template_release

# web
./tools/init_web.sh
# android
cd $GODOT_PATH || exit
build_platform android target=template_release arch=arm32
build_platform android target=template_release arch=arm64 && cd $GODOT_PATH/platform/android/java && ./gradlew generateGodotTemplates

# linux 
cd $GODOT_PATH || exit
scons platform=linux target=template_release

# Build templates
# windows
build_platform windows target=template_debug arch=x86_32
build_platform windows target=template_release arch=x86_32
build_platform windows target=template_debug arch=x86_64
build_platform windows target=template_release arch=x86_64

echo "All builds completed successfully!"
echo "Build logs can be found in the logs directory"
