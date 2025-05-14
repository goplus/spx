#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# copy version file
cp -f $SCRIPT_DIR/../../../cmd/gox/template/version $SCRIPT_DIR

EDITOR_ONLY=false
PLATFORM=""
DOWNLOAD=false
while getopts "p:ed" opt; do
    case "$opt" in
        d) DOWNLOAD=true ;;
        p) PLATFORM="$OPTARG" ;;
        e) EDITOR_ONLY=true ;;
        *) echo "Usage: $0 [-p platform] [-e]"; exit 1 ;;
    esac
done
source $SCRIPT_DIR/common/setup_env.sh
cd $PROJ_DIR

build_template() {
    prepare_env
    local engine_dir="$ENGINE_DIR"
    local platform=$PLATFORM
    local template_dir="$TEMPLATE_DIR"

    echo "save to $template_dir"
    cd $engine_dir || exit

    dstBinPath="$GOPATH/bin/gdspxrt$VERSION"  #gdspxrt 
    echo "Destination binary path: $dstBinPath"
    local target_build_str="template_release"
    if [ "$platform" = "linux" ]; then
        scons platform=linuxbsd target=$target_build_str
        cp bin/godot.linuxbsd.$target_build_str.$ARCH $dstBinPath

    elif [ "$platform" = "windows" ]; then
        scons platform=windows target=$target_build_str
        cp bin/godot.windows.$target_build_str.$ARCH.exe $dstBinPath".exe"

    elif [ "$platform" = "macos" ]; then
        scons platform=macos target=$target_build_str
        cp bin/godot.macos.$target_build_str.$ARCH $dstBinPath

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
        scons platform=web target=editor \
            debug_symbols="no" \
            optimize=size \
            use_volk=no \
            deprecated=no \
            minizip=no  \
            openxr=false \
            vulkan=false \
            graphite=false \
            disable_3d_physics=true \
            disable_navigation=true \
            module_msdfgen_enabled=false \
            module_text_server_adv_enabled=false \
            module_text_server_fb_enabled=true \
            modules_enabled_by_default="no" \
            module_gdscript_enabled="yes" \
            module_text_server_fb_enabled="yes" \
            module_freetype_enabled="yes" \
            module_minimp3_enabled="yes" \
            module_svg_enabled="yes" \
            module_jpg_enabled="yes" \
            module_ogg_enabled="yes" \
            module_godot_physics_2d_enabled="yes" 

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
download_editor() {
    setup_global_variables
    local platform=$PLATFORM
    local arch=$ARCH
    local tmp_dir=$SCRIPT_DIR/bin
    local dst_dir=$GOPATH/bin
    local url_prefix="https://github.com/goplus/godot/releases/download/spx$VERSION/"
    mkdir -p "$tmp_dir"
    mkdir -p "$dst_dir"

    # download engine pack
    url=$url_prefix"gdspxrt.pck"
    curl -L -o "$dst_dir/gdspxrt$VERSION.pck" "$url" || exit

    if [ "$platform" = "linux" ]; then
        zip_name="editor-linux-"$arch".zip"
        url=$url_prefix$zip_name
        curl -L -o "$tmp_dir/$zip_name" "$url" || exit
        unzip -o "$tmp_dir/$zip_name" -d "$tmp_dir" > /dev/null 2>&1  || exit
        rm -rf "$tmp_dir/$zip_name"

        cp -f "$tmp_dir/godot.linuxbsd.editor.$arch" "$dst_dir/gdspx$VERSION"  || exit
        rm -rf "$tmp_dir/godot.linuxbsd.editor.$arch"

    elif [ "$platform" = "windows" ]; then
        zip_name="editor-windows-"$arch".zip"
        url=$url_prefix$zip_name
        curl -L -o "$tmp_dir/$zip_name" "$url" || exit
        unzip -o "$tmp_dir/$zip_name" -d "$tmp_dir" > /dev/null 2>&1  || exit
        rm -rf "$tmp_dir/$zip_name"
        cp -f "$tmp_dir/godot.windows.editor.$arch.exe" "$dst_dir/gdspx$VERSION"".exe"  || exit
        rm -rf "$tmp_dir/godot.windows.editor.$arch.exe"

    elif [ "$platform" = "macos" ]; then
        zip_name="editor-macos-universal.zip"
        url=$url_prefix$zip_name
        curl -L -o "$tmp_dir/$zip_name" "$url" || exit
        unzip -o "$tmp_dir/$zip_name" -d "$tmp_dir" > /dev/null 2>&1  || exit
        rm -rf "$tmp_dir/$zip_name"
        
        cp -f "$tmp_dir/Godot.app/Contents/MacOS/Godot" "$dst_dir/gdspx$VERSION"  || exit
        rm -rf "$tmp_dir/Godot.app"
    else 
        echo "Unsupported platform for editor download: $platform"
        exit 1
    fi
}

build_editor(){
    prepare_env
    cd $ENGINE_DIR
    if [ "$PLATFORM" == "web" ]; then
        build_template "$PLATFORM"
        return 0
    fi

    if [ "$OS" = "Windows_NT" ]; then
        scons target=editor vsproj=yes dev_build=yes
    else
        scons target=editor dev_build=yes
    fi
    
    dstBinPath="$GOPATH/bin/gdspx$VERSION"
    echo "Destination binary path: $dstBinPath"
    if [ "$OS" = "Windows_NT" ]; then
        cp bin/godot.windows.editor.dev.$ARCH $dstBinPath".exe"
    elif [[ "$(uname)" == "Linux" ]]; then
        cp bin/godot.linuxbsd.editor.dev.$ARCH $dstBinPath
    else
        cp bin/godot.macos.editor.dev.$ARCH $dstBinPath
    fi
}



# main logic
if [ "$DOWNLOAD" = true ]; then
    # download editor
    download_editor || exit
elif [ "$EDITOR_ONLY" = true ]; then
    # build editor
    build_editor || exit
else 
    # build template
    build_template || exit
fi 
cd $PROJ_DIR

echo "Environment initialized successfully!"
echo "Try the following command to run the demo:"
echo "spx run -path tutorial/00-Hello"
