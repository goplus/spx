#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# copy version file
cp -f $SCRIPT_DIR/../../../cmd/gox/template/version $SCRIPT_DIR
# PCK version， only changed when spx's engine resource is updated
PCK_VERSION=2.0.30
EDITOR_ONLY=false
PLATFORM=""
DOWNLOAD=false
DOWNLOAD_ENGINE=false
MODE=""
while getopts "p:m:edg" opt; do
    case "$opt" in
        d) DOWNLOAD=true ;;
        g) DOWNLOAD_ENGINE=true ;;
        p) PLATFORM="$OPTARG" ;;
        e) EDITOR_ONLY=true ;;
        m) MODE="$OPTARG";;
        *) echo "Usage: $0 [-p platform] [-e] [-d] [-g] [-m mode]"; exit 1 ;;
    esac
done

source $SCRIPT_DIR/common/setup_env.sh
cd $PROJ_DIR

COMMON_ARGS='
            optimize=size
            use_volk=no 
            deprecated=no 
            minizip=yes  
            openxr=false 
            vulkan=false 
            graphite=false 
            disable_3d_physics=true 
            module_msdfgen_enabled=false 
            module_text_server_adv_enabled=false 
            module_text_server_fb_enabled=true 
            modules_enabled_by_default=true 
            module_gdscript_enabled=true 
            module_freetype_enabled=true 
            module_minimp3_enabled=true 
            module_svg_enabled=true 
            module_jpg_enabled=true 
            module_ogg_enabled=true 
            module_regex_enabled=true 
            module_zip_enabled=true 
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

        WEB_ARGS=""
        thread_flags=""
        if [ "$MODE" = "minigame" ]; then
            thread_flags=".nothreads"
            WEB_ARGS="threads=no"
        elif [ "$MODE" = "worker" ]; then
            # build web templates with threads(web worker)
            thread_flags=""
            WEB_ARGS="threads=yes proxy_to_pthread=true"
        elif [ "$MODE" = "miniprogram" ]; then
            thread_flags=".nothreads"
            WEB_ARGS="threads=no"
        else 
            thread_flags=""
            WEB_ARGS="threads=yes"
        fi

        echo "scons platform=web target=template_release $COMMON_ARGS $EXTRA_OPT_ARGS $WEB_ARGS"
        scons platform=web target=template_release $COMMON_ARGS $EXTRA_OPT_ARGS $WEB_ARGS
        echo "Wait zip file to finished ..."
        sleep 1
        cp bin/godot.web.template_release.wasm32$thread_flags.zip bin/web_dlink_debug.zip
        cp bin/web_dlink_debug.zip $GOPATH/bin/gdspx$VERSION"_webpack.zip"

        rm "$template_dir"/web_*.zip
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_nothreads_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_nothreads_release.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_nothreads_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_nothreads_release.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_dlink_release.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_debug.zip"
        cp bin/web_dlink_debug.zip "$template_dir/web_release.zip"

    else
        echo "Unknown platform"
    fi
}

download_editor() {
    setup_global_variables
    local saved_platform=$PLATFORM
    local saved_mode=$MODE
    echo "===> Downloading pc via download_engine..."
    download_engine || exit

    # Download and extract gdspxrt.pck zip file
    echo "===> Downloading gdspxrt.pck..."
    local pck_url="https://github.com/goplus/spx/releases/download/v2.0.0-pre.30/gdspxrt.pck.${PCK_VERSION}.zip"
    local tmp_dir=$SCRIPT_DIR/bin
    local dst_dir=$GOPATH/bin
    local pck_zip="$tmp_dir/gdspxrt.pck.${PCK_VERSION}.zip"
    
    mkdir -p "$tmp_dir"
    mkdir -p "$dst_dir"
    
    echo "Downloading from: $pck_url"
    if curl -L -o "$pck_zip" "$pck_url"; then
        echo "Download successful, extracting gdspxrt.pck..."
        local pck_tmp_dir=$(mktemp -d)
        unzip -o "$pck_zip" -d "$pck_tmp_dir" > /dev/null 2>&1 || exit
        
        # Copy extracted files to GOPATH/bin
        if [ -d "$pck_tmp_dir" ]; then
            # Copy all files from extracted directory to GOPATH/bin
            for file in "$pck_tmp_dir"/*; do
                if [ -f "$file" ]; then
                    cp -f "$file" "$dst_dir/"
                    echo "Copied $(basename "$file") to $dst_dir"
                fi
            done
            echo "gdspxrt.pck files copied to $dst_dir"
        fi
        
        mv $dst_dir/gdspxrt.pck $dst_dir/gdspxrt$VERSION.pck
        # Clean up
        rm -rf "$pck_tmp_dir"
        rm -f "$pck_zip"
        echo "gdspxrt.pck downloaded and extracted successfully!"
    else
        echo "Warning: Failed to download gdspxrt.pck from $pck_url"
        echo "Continuing without pck file..."
    fi

    # List final files
    echo "Files in $GOPATH/bin:"
    ls -l "$GOPATH/bin"
}

download_engine() {
    setup_global_variables
    local platform=$PLATFORM
    local arch=$ARCH
    local template_dir="$TEMPLATE_DIR"
    local url_prefix="https://github.com/goplus/godot/releases/download/spx$VERSION/"
    local tmp_dir=$SCRIPT_DIR/bin
    local dst_dir=$GOPATH/bin

    mkdir -p "$tmp_dir"
    mkdir -p "$dst_dir"
    mkdir -p "$template_dir"
    echo "Downloading engine templates for platform: $platform"
    echo "Template directory: $template_dir"
    echo "URL prefix: $url_prefix"

    if [ "$platform" = "android" ]; then
        local android_zip="$template_dir/android.zip"
        local android_tmp_dir=$(mktemp -d)

        echo "===> Downloading Android engine templates..."
        if [ -f "$android_zip" ]; then
            echo "Android template archive already exists, removing old version..."
            rm -f "$android_zip"
        fi

        echo "Downloading from: ${url_prefix}android.zip"
        if curl -L -o "$android_zip" "${url_prefix}android.zip"; then
            echo "Download successful, extracting Android templates..."

            # Extract android templates to template directory
            unzip -o "$android_zip" -d "$android_tmp_dir" > /dev/null 2>&1

            # Copy all APK files and android_source.zip to template directory
            if [ -f "$android_tmp_dir/android_debug.apk" ]; then
                cp -f "$android_tmp_dir/android_debug.apk" "$template_dir/"
                echo "Copied android_debug.apk"
            fi

            if [ -f "$android_tmp_dir/android_release.apk" ]; then
                cp -f "$android_tmp_dir/android_release.apk" "$template_dir/"
                echo "Copied android_release.apk"
            fi

            if [ -f "$android_tmp_dir/android_source.zip" ]; then
                cp -f "$android_tmp_dir/android_source.zip" "$template_dir/"
                echo "Copied android_source.zip"
            fi

            # Clean up
            rm -rf "$android_tmp_dir"
            rm -f "$android_zip"

            echo "Android engine templates downloaded and extracted successfully!"
        else
            echo "Error: Failed to download Android templates from ${url_prefix}android.zip"
            rm -rf "$android_tmp_dir"
            exit 1
        fi

    elif [ "$platform" = "ios" ]; then
        local ios_zip="$template_dir/ios.zip"

        echo "===> Downloading iOS engine templates..."
        if [ -f "$ios_zip" ]; then
            echo "iOS template already exists, removing old version..."
            rm -f "$ios_zip"
        fi

        echo "Downloading from: ${url_prefix}ios.zip"
        if curl -L -o "$ios_zip" "${url_prefix}ios.zip"; then
            echo "iOS engine templates downloaded successfully!"
        else
            echo "Error: Failed to download iOS templates from ${url_prefix}ios.zip"
            exit 1
        fi

    elif [ "$platform" = "web" ]; then
        # Get mode parameter (default: normal)
        local web_mode="${MODE:-normal}"

        # Validate mode
        if [[ "$web_mode" != "normal" && "$web_mode" != "worker" && "$web_mode" != "minigame" && "$web_mode" != "miniprogram" ]]; then
            echo "Error: Invalid mode '$web_mode'. Supported modes: normal, worker, minigame, miniprogram"
            echo "Usage: $0 -g -p web [-m mode]"
            exit 1
        fi

        # Template package names from godot CI based on mode
        local template_name=""
        case "$web_mode" in
            normal)
                template_name="web.zip"
                ;;
            worker)
                template_name="web-worker.zip"
                ;;
            minigame)
                template_name="web-minigame.zip"
                ;;
            miniprogram)
                template_name="web-miniprogram.zip"
                ;;
        esac

        echo "===> Setting up web $web_mode templates..."
        echo "Mode: $web_mode"
        echo "Version: $VERSION"
        echo "URL Prefix: $url_prefix"
        echo "Template Name: $template_name"
        echo "Template Directory: $template_dir"

        # Download template if not exists
        # For normal mode, use webpack.zip to match download_editor naming convention
        if [ "$web_mode" = "normal" ]; then
            local template_file="$dst_dir/gdspx${VERSION}_webpack.zip"
        else
            local template_file="$dst_dir/gdspx${VERSION}_web${web_mode}.zip"
        fi
        if [ -f "$template_file" ]; then
            echo "Web $web_mode template already exists, skipping download"
        else
            echo "Downloading web $web_mode template..."
            echo "URL: ${url_prefix}${template_name}"
            if curl -L -o "$template_file" "${url_prefix}${template_name}"; then
                echo "Download successful: $template_file"
            else
                echo "Error: Failed to download web $web_mode template"
                echo "Make sure the godot release contains: $template_name"
                exit 1
            fi
        fi

        # Setup template directory structure with mode-specific templates
        echo "===> Setting up template directory structure..." "$template_file" "$template_dir"

        cp -f "$template_file" "$template_dir/web_dlink_nothreads_debug.zip"
        cp -f "$template_file" "$template_dir/web_dlink_nothreads_release.zip"
        cp -f "$template_file" "$template_dir/web_nothreads_debug.zip"
        cp -f "$template_file" "$template_dir/web_nothreads_release.zip"
        cp -f "$template_file" "$template_dir/web_dlink_debug.zip"
        cp -f "$template_file" "$template_dir/web_dlink_release.zip"
        cp -f "$template_file" "$template_dir/web_debug.zip"
        cp -f "$template_file" "$template_dir/web_release.zip"

        echo "===> Web $web_mode setup complete"
        echo "  - Template downloaded: $template_file"
        echo "  - Templates installed to: $template_dir"
        echo "  - Mode: $web_mode"

    elif [ "$platform" = "linux" ] || [ "$platform" = "windows" ] || [ "$platform" = "macos" ]; then
        # PC platform: download editor or template based on MODE
        platform_name=$platform
        local binary_postfix=""
        if [ "$platform" = "linux" ]; then
            platform_name="linuxbsd"
        elif [ "$platform" = "windows" ]; then
            binary_postfix=".exe"
        fi

        if [ "$MODE" = "editor" ]; then
            # Download PC editor only
            local zip_name="editor-$platform-"$arch".zip"
            local binary_name="godot.$platform_name.editor.$arch$binary_postfix"
            local final_binary="$dst_dir/gdspx$VERSION"$binary_postfix
            local url=$url_prefix$zip_name
            
            echo "===> Downloading PC editor..."
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

            # Clean up temporary files if they exist
            [ -f "$tmp_dir/$zip_name" ] && rm -f "$tmp_dir/$zip_name"
            [ -d "$tmp_dir" ] && rm -rf "$tmp_dir"
        else
            # Download PC template only (MODE not set or other value)
            local zip_name="$platform-"$arch".zip"
            local binary_name="godot.$platform_name.template_release.$arch$binary_postfix"
            local template_binary="$dst_dir/gdspxrt$VERSION"$binary_postfix
            local url=$url_prefix$zip_name
            
            echo "===> Downloading PC template..."
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
                echo "===> Downloading macOS template..."
                local macos_zip="$template_dir/macos.zip"
                if [ -f "$macos_zip" ]; then
                    echo "macOS template already exists, skipping download"
                else
                    echo "Downloading macOS template..."
                    curl -L -o "$macos_zip" $url_prefix"macos.zip" || exit
                fi
            fi

            # Clean up temporary files if they exist
            [ -f "$dst_dir/$zip_name" ] && rm -f "$dst_dir/$zip_name"
            [ -f "$tmp_dir/$zip_name" ] && rm -f "$tmp_dir/$zip_name"
            [ -d "$tmp_dir" ] && rm -rf "$tmp_dir"
        fi

    else
        echo "Error: Unsupported platform for download_engine: $platform"
        echo "Supported platforms: android, ios, web, linux, windows, macos"
        exit 1
    fi

    echo "===> Download engine completed!"
    echo "Files in $template_dir:"
    ls -la "$template_dir"
}

build_editor(){
    prepare_env
    cd $ENGINE_DIR
    if [ "$PLATFORM" == "web" ]; then
        build_template "$PLATFORM"
        return 0
    fi
    
    echo scons target=editor dev_build=yes $COMMON_ARGS
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

# main logic
if [ "$DOWNLOAD" = true ]; then
    # download editor
    download_editor || exit
elif [ "$DOWNLOAD_ENGINE" = true ]; then
    # download engine templates (android/ios)
    download_engine || exit
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
