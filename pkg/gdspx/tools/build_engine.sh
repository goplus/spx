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
COMMON_ARGS='
            optimize=size 
            use_volk=no 
            deprecated=no 
            openxr=false 
            vulkan=false 
            graphite=false 
            disable_3d_physics=true 
            disable_navigation=true 
            module_msdfgen_enabled=false 
            module_text_server_adv_enabled=false 
            module_text_server_fb_enabled=true 
            module_gdscript_enabled=true 
            module_freetype_enabled=true 
            module_minimp3_enabled=true 
            module_svg_enabled=true 
            module_jpg_enabled=true 
            module_ogg_enabled=true 
            module_zip_enabled=true 
            module_mobile_vr_enabled=false
            module_openxr_enabled=false
            module_webxr_enabled=false
            module_text_server_adv_enabled=false
            module_webrtc_enabled=false
            module_godot_physics_2d_enabled=true '

EXTRA_OPT_ARGS='disable_3d=true'


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
        scons platform=windows target=$target_build_str $COMMON_ARGS
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
        scons platform=web target=editor $COMMON_ARGS

           
        cp bin/godot.web.editor.wasm32.zip bin/web_editor.zip
        cp bin/web_editor.zip $GOPATH/bin/gdspx$VERSION"_web.zip"
        if [ "$EDITOR_ONLY" = true ]; then
            exit 0
        fi 
        thread_flags=".nothreads"
        # build web templates
        scons platform=web target=template_debug threads=no $COMMON_ARGS $EXTRA_OPT_ARGS debug_symbols=true 
        echo "Wait zip file to finished ..."
        sleep 2
        cp bin/godot.web.template_debug.wasm32$thread_flags.zip bin/web_dlink_debug.zip
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
    echo "download to $dst_dir"
    # download engine pack
    local url=""

    local template_dir="$TEMPLATE_DIR"
    
    # Check if web zip files exist and download if they don't
    local web_pack_file="$dst_dir/gdspx"$VERSION"_webpack.zip"
    local web_editor_file="$dst_dir/gdspx"$VERSION"_web.zip"
    
    echo "===>Download task 1/5 ....web template..."
    if [ -f "$web_pack_file" ]; then
        echo "Web template file already exists, skipping download"
    else
        echo "Downloading web template..."
        curl -L -o "$web_pack_file" $url_prefix"web.zip" || exit
    fi
    
    echo "===>Download task 2/5 ....web editor..."
    if [ -f "$web_editor_file" ]; then
        echo "Web editor file already exists, skipping download"
    else
        echo "Downloading web editor..."
        curl -L -o "$web_editor_file" $url_prefix"editor-web.zip" || exit
    fi
    
    local filename="$web_pack_file"
    cp -f $filename "$template_dir/web_dlink_debug.zip"
    cp -f $filename "$template_dir/web_dlink_release.zip"
    cp -f $filename "$template_dir/web_debug.zip"
    cp -f $filename "$template_dir/web_release.zip"
    
    platform_name=$platform
    local binary_postfix=""
    if [ "$platform" = "linux" ]; then
        platform_name="linuxbsd"
    elif [ "$platform" = "windows" ]; then
        binary_postfix=".exe"
    fi

    local zip_name="editor-$platform-"$arch".zip"
    local binary_name="godot.$platform_name.editor.$arch$binary_postfix"
    local final_binary="$dst_dir/gdspx$VERSION"$binary_postfix
    url=$url_prefix$zip_name
    
    echo "===>Download task 3/5 ....pc editor..."
    # Check if editor binary already exists
    if [ -f "$final_binary" ]; then
        echo "Editor binary already exists, skipping download"
    else
        echo "Downloading pc editor..."
        curl -L -o "$dst_dir/$zip_name" "$url" || exit
        unzip -o "$dst_dir/$zip_name" -d "$tmp_dir" > /dev/null 2>&1  || exit
        cp -f "$tmp_dir/$binary_name" "$dst_dir/gdspx$VERSION"$binary_postfix  || exit
        rm -rf "$dst_dir/$zip_name"
    fi

    # download template
    zip_name="$platform-"$arch".zip"
    binary_name="godot.$platform_name.template_release.$arch$binary_postfix"
    local template_binary="$dst_dir/gdspxrt$VERSION"$binary_postfix

    url=$url_prefix$zip_name
    
    echo "===>Download task 4/5 ....pc template..."
    # Check if template binary already exists
    if [ -f "$template_binary" ]; then
        echo "Template binary already exists, skipping download"
    else
        echo "Downloading pc template..."
        curl -L -o "$dst_dir/$zip_name" "$url" || exit
        unzip -o "$dst_dir/$zip_name" -d "$tmp_dir" > /dev/null 2>&1  || exit
        cp -f "$tmp_dir/$binary_name" "$template_binary"  || exit
    fi

    local filename="$template_binary"
    # copy to build template dir
    if [ "$platform" = "linux" ]; then
        cp "$filename" "$template_dir/linux_debug.arm32"
        cp "$filename" "$template_dir/linux_debug.arm64"
        cp "$filename" "$template_dir/linux_debug.x86_32"
        cp "$filename" "$template_dir/linux_debug.x86_64"
        cp "$filename" "$template_dir/linux_release.arm32"
        cp "$filename" "$template_dir/linux_release.arm64"
        cp "$filename" "$template_dir/linux_release.x86_32"
        cp "$filename" "$template_dir/linux_release.x86_64"

    elif [ "$platform" = "windows" ]; then
        cp "$filename" "$template_dir/windows_debug_x86_32_console.exe"
        cp "$filename" "$template_dir/windows_debug_x86_32.exe"
        cp "$filename" "$template_dir/windows_debug_x86_64_console.exe"
        cp "$filename" "$template_dir/windows_debug_x86_64.exe"
        cp "$filename" "$template_dir/windows_release_x86_32_console.exe"
        cp "$filename" "$template_dir/windows_release_x86_32.exe"
        cp "$filename" "$template_dir/windows_release_x86_64_console.exe"
        cp "$filename" "$template_dir/windows_release_x86_64.exe"

    elif [ "$platform" = "macos" ]; then
        echo "===>Download task 5/5 ....macOS template..."
        local macos_zip="$template_dir/macos.zip"
        if [ -f "$macos_zip" ]; then
            echo "macOS template already exists, skipping download"
        else
            echo "Downloading macOS template..."
            curl -L -o "$macos_zip" $url_prefix"macos.zip" || exit
        fi
    else 
        echo "Unsupported platform for editor download: $platform"
        exit 1
    fi
    echo "===>Download task done ...."

    # Clean up temporary files if they exist
    [ -f "$dst_dir/$zip_name" ] && rm -f "$dst_dir/$zip_name"
    [ -f "$tmp_dir/$zip_name" ] && rm -f "$tmp_dir/$zip_name"
    [ -d "$tmp_dir" ] && rm -rf "$tmp_dir"
    
    # List final files
    echo "Files in $dst_dir:"
    ls -l "$dst_dir"
    echo "Files in $template_dir:"
    ls -l "$template_dir"
}

build_editor(){
    prepare_env
    cd $ENGINE_DIR
    if [ "$PLATFORM" == "web" ]; then
        build_template "$PLATFORM"
        return 0
    fi
    if [ "$OS" = "Windows_NT" ]; then
        scons target=editor dev_build=yes $COMMON_ARGS vsproj=yes 
    else
        scons target=editor dev_build=yes $COMMON_ARGS
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

# Define a function for the release web functionality
exportweb() {
    echo "Starting exportweb... $PROJ_DIR"
    
    # Create temporary directory
    mkdir -p "$PROJ_DIR/.tmp/web"
    
    # Execute the exportweb commands
    (cd "$PROJ_DIR/.tmp/web" 
     mkdir -p assets 
     echo '{"map":{"width":480,"height":360}}' > assets/index.json 
     echo "" > main.spx 
     rm -rf ./project/.builds/*web 
     spx exportweb 
     cd ./project/.builds/web 
     rm -f game.zip 
     zip -r "$PROJ_DIR/spx_web.zip" * 
     echo "$PROJ_DIR/spx_web.zip has been created") || {
        echo "Error: Failed to create web export"
        return 1
    }
    
    # Clean up
    rm -rf "$PROJ_DIR/.tmp"
    echo "exportweb completed successfully"
    return 0
}

# Define a function for the exportpack functionality
exportpack() {
    echo "Starting exportpack..."
    
    # Check GOPATH
    if [ -z "$GOPATH" ]; then
        # If GOPATH is not set, attempt to get it
        if command -v go > /dev/null; then
            if [ "$OS" = "Windows_NT" ]; then
                IFS=';' read -r GOPATH _ <<< "$(go env GOPATH)"
            else
                IFS=':' read -r GOPATH _ <<< "$(go env GOPATH)"
            fi
        fi
        
        if [ -z "$GOPATH" ]; then
            echo "Error: GOPATH is not set"
            return 1
        fi
    fi
    
    # Create temporary directory and copy files
    rm -rf "$PROJ_DIR/.tmp/web" 
    mkdir -p "$PROJ_DIR/.tmp/web" 
    cp "$PROJ_DIR/cmd/gox/template/project/runtime.gdextension.txt" "$GOPATH/bin/runtime.gdextension" || {
        echo "Error: Failed to prepare exportpack environment"
        return 1
    }
    
    # Execute the exportpack commands
    cd "$PROJ_DIR/.tmp/web" 
    mkdir -p assets 
    echo '{"map":{"width":480,"height":360}}' > assets/index.json 
    echo "" > main.spx 
    rm -rf ./project/.builds/*web 
    mkdir -p "$GOPATH/bin" 
    echo "exporting pck..."

    spx export 
    TEMP_VERSION=$(cat "$PROJ_DIR/cmd/gox/template/version") 
    OUTPUT_PCK="$GOPATH/bin/gdspxrt$TEMP_VERSION.pck" 
    echo $OUTPUT_PCK
    # Check if the files exist before copying
    if [ -f "./project/.builds/pc/gdexport.pck" ]; then
        echo "Copying gdexport.pck to $OUTPUT_PCK"
        cp "./project/.builds/pc/gdexport.pck" "$OUTPUT_PCK"
    fi
    
    # For macOS builds
    if [ -d "./project/.builds/pc/gdexport.app/Contents/Resources" ] && \
       [ "$(ls -A ./project/.builds/pc/gdexport.app/Contents/Resources/*.pck 2>/dev/null)" ]; then
        echo "Copying macOS resources to $OUTPUT_PCK"
        cp ./project/.builds/pc/gdexport.app/Contents/Resources/*.pck "$OUTPUT_PCK"
    fi
    

    echo "exporting web runtime..."
    spx exportwebruntime 
    dstdir="$GOPATH/bin/gdspxrt"$TEMP_VERSION"_web"
    rm -rf "$dstdir" 
    cp -rf ./project/.builds/webi  "$dstdir" 
    
    # Clean up
    rm -rf "$PROJ_DIR/.tmp"
    echo "exportpack completed successfully"
    return 0
}

# Define a function for the runweb functionality
runweb() {
    local target_path="$1"
    
    # Default path if not provided
    if [ -z "$target_path" ]; then
        target_path="tutorial/01-Weather"
    fi
    
    echo "Starting runweb with path: $target_path"
    
    # Kill any running gdspx_web_server.py processes
    echo "Killing gdspx_web_server.py if running..."
    if command -v pgrep > /dev/null; then
        PIDS=$(pgrep -f gdspx_web_server.py)
        if [ -n "$PIDS" ]; then
            echo "Killing process: $PIDS"
            kill -9 $PIDS
        else
            echo "No gdspx_web_server.py process found."
        fi
    else
        echo "pgrep command not found, skipping process killing"
    fi
    
    # Run cmdweb and start the web server
    (cd "$PROJ_DIR/cmd/gox/" && ./install.sh --web) 
    (cd "$PROJ_DIR/$target_path" && spx clear && spx runweb -serveraddr=":8106") || {
        echo "Error: Failed to run web server"
        return 1
    }
    
    echo "runweb completed successfully"
    return 0
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
