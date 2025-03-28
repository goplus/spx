#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

EDITOR_ONLY=false
PLATFORM=""
while getopts "p:e" opt; do
    case "$opt" in
        p) PLATFORM="$OPTARG" ;;
        e) EDITOR_ONLY=true ;;
        *) echo "Usage: $0 [-p platform] [-e]"; exit 1 ;;
    esac
done

source $SCRIPT_DIR/common/setup_env.sh
cd $PROJ_DIR

build_target() {
    local engine_dir=$1
    local platform=$2
    local template_dir=$3

    echo "save to $template_dir"
    cd $engine_dir || exit
    if [ "$platform" = "linux" ]; then
        scons platform=linuxbsd target=template_debug
        scons platform=linuxbsd target=template_release
        cp bin/godot.linuxbsd.template_* "$template_dir/"
        #mv "$template_dir/godot.linuxbsd.template_debug"*  "$template_dir/linux_debug.$ARCH"
        #mv "$template_dir/godot.linuxbsd.template_release"*  "$template_dir/linux_release.$ARCH"

    elif [ "$platform" = "windows" ]; then
        scons platform=windows target=template_debug arch=x86_32
        scons platform=windows target=template_release arch=x86_32
        scons platform=windows target=template_debug arch=x86_64
        scons platform=windows target=template_release arch=x86_64
        echo "Destination binary path: $template_dir"
        cp bin/windows*.exe "$template_dir/"

    elif [ "$platform" = "macos" ]; then
        scons platform=macos target=template_debug arch=arm64
        scons platform=macos target=template_release arch=arm64
        scons platform=macos target=template_debug arch=x86_64
        scons platform=macos target=template_release arch=x86_64 generate_bundle=yes

        echo "lipo ..."
        lipo -create bin/godot.macos.template_release.x86_64 bin/godot.macos.template_release.arm64 -output bin/godot.macos.template_release.universal
        lipo -create bin/godot.macos.template_debug.x86_64 bin/godot.macos.template_debug.arm64 -output bin/godot.macos.template_debug.universal
        
        echo "create app ..."
        cd bin
        cp -r ../misc/dist/macos_template.app .
        mkdir -p macos_template.app/Contents/MacOS
        cp godot.macos.template_release.universal macos_template.app/Contents/MacOS/godot_macos_release.universal
        cp godot.macos.template_debug.universal macos_template.app/Contents/MacOS/godot_macos_debug.universal
        chmod +x macos_template.app/Contents/MacOS/godot_macos*
        zip -q -9 -r macos.zip macos_template.app
        
        cp macos.zip "$template_dir/macos.zip"
        cd ..
        echo "done"

    elif [ "$platform" = "ios" ]; then
        scons platform=ios vulkan=True target=template_debug ios_simulator=yes arch=arm64 
        scons platform=ios vulkan=True target=template_debug ios_simulator=yes arch=x86_64
        scons platform=ios vulkan=True target=template_release ios_simulator=yes arch=arm64 
        scons platform=ios vulkan=True target=template_release ios_simulator=yes arch=x86_64 generate_bundle=yes
        scons platform=ios vulkan=True target=template_debug ios_simulator=no
        scons platform=ios vulkan=True target=template_release ios_simulator=no generate_bundle=yes 

        cp -f bin/godot_ios.zip "$template_dir/ios.zip"

    elif [ "$platform" = "android" ]; then
        # Ensure JDK 17 is installed for Android builds
        ensure_jdk
        cd $engine_dir || exit
        scons platform=android target=template_debug arch=arm32
        scons platform=android target=template_debug arch=arm64
        scons platform=android target=template_release arch=arm32
        scons platform=android target=template_release arch=arm64
        cd platform/android/java || exit
        # On Linux and macOS
        ./gradlew generateGodotTemplates

        cd $engine_dir || exit
        cp -f bin/android*.apk "$template_dir/"
        cp -f bin/android_source.zip "$template_dir/"

    elif [ "$platform" = "web" ]; then   
        # Setup emsdk environment
        ensure_emsdk
        # Change to godot directory
        cd $engine_dir || exit

        # build web editor
        scons platform=web target=editor
        cp bin/godot.web.editor.wasm32.zip bin/web_editor.zip
        cp bin/web_editor.zip $GOPATH/bin/gdspx$VERSION"_web.zip"
        if [ "$EDITOR_ONLY" = true ]; then
            exit 0
        fi 

        # build web templates
        scons platform=web target=template_release threads=no
        echo "Wait zip file to finished ..."
        sleep 2
        cp bin/godot.web.template_release.wasm32.zip bin/web_dlink_debug.zip
        rm "$template_dir"/web_*.zip
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_release.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_release.zip"
        # copy to tool dir
        cp bin/web_dlink_debug.zip $GOPATH/bin/gdspx$VERSION"_webpack.zip"
    else
        echo "Unknown platform"
    fi
}

if [ "$EDITOR_ONLY" = true ]; then
    cd $ENGINE_DIR
    if [ "$PLATFORM" == "web" ]; then
        build_target "$ENGINE_DIR" "$PLATFORM" "$TEMPLATE_DIR"
        exit 0
    fi

    if [ "$OS" = "Windows_NT" ]; then
        scons target=editor vsproj=yes dev_build=yes
    else
        scons target=editor dev_build=yes
    fi
    
    dstBinPath="$GOPATH/bin/gdspx$VERSION"
    echo "Destination binary path: $dstBinPath"
    if [ "$OS" = "Windows_NT" ]; then
        cp bin/godot.windows.editor.dev.$ARCH $dstBinPath"_win.exe"
    elif [[ "$(uname)" == "Linux" ]]; then
        cp bin/godot.linuxbsd.editor.dev.$ARCH $dstBinPath"_linux"
    else
        cp bin/godot.macos.editor.dev.$ARCH $dstBinPath"_darwin"
    fi
    exit 0
fi 

build_target "$ENGINE_DIR" "$PLATFORM" "$TEMPLATE_DIR"
cd $PROJ_DIR