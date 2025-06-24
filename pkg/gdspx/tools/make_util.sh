#!/bin/bash

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJ_DIR="$SCRIPT_DIR/../../.."
echo $PROJ_DIR

# Set CURRENT_PATH to the project root directory
CURRENT_PATH="$PROJ_DIR"
# Define a function for the release web functionality
do_exportweb() {
    echo "Starting exportweb..."
    
    # Create temporary directory
    mkdir -p "$CURRENT_PATH/.tmp/web"
    
    # Execute the exportweb commands
    (cd "$CURRENT_PATH/.tmp/web" 
     mkdir -p assets 
     echo '{"map":{"width":480,"height":360}}' > assets/index.json 
     echo "" > main.spx 
     rm -rf ./project/.builds/*web 
     spx exportweb 
     cd ./project/.builds/web 
     rm -f game.zip 
     zip -r "$CURRENT_PATH/spx_web.zip" * 
     echo "$CURRENT_PATH/spx_web.zip has been created") || {
        echo "Error: Failed to create web export"
        return 1
    }
    
    # Clean up
    rm -rf "$CURRENT_PATH/.tmp"
    echo "exportweb completed successfully"
    return 0
}
do_prepare_export() {
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
    rm -rf "$CURRENT_PATH/.tmp/web" 
    mkdir -p "$CURRENT_PATH/.tmp/web" 
    cp "$CURRENT_PATH/cmd/gox/template/project/runtime.gdextension.txt" "$GOPATH/bin/runtime.gdextension" || {
        echo "Error: Failed to prepare exportpack environment"
        return 1
    }
    
    # Execute the exportpack commands
    cd "$CURRENT_PATH/.tmp/web" 
    mkdir -p assets 
    echo '{"map":{"width":480,"height":360}}' > assets/index.json 
    echo "" > main.spx 
    rm -rf ./project/.builds/*web 
    mkdir -p "$GOPATH/bin" 

    TEMP_VERSION=$(cat "$CURRENT_PATH/cmd/gox/template/version") 
    OUTPUT_PCK="$GOPATH/bin/gdspxrt$TEMP_VERSION.pck" 
    ls $GOPATH/bin
}

# Define a function for the exportpack functionality
do_extra_webtemplate() {
    do_prepare_export
    dstdir="$GOPATH/bin/gdspxrt"$TEMP_VERSION"_web"
    echo "exporting web runtime..."
    spx exportwebruntime 
    rm -rf "$dstdir" 
    cp -rf ./project/.builds/webi  "$dstdir" 
    mv "$dstdir/engine.pck" "$dstdir/engine.zip"
    echo "exporting web runtime done: $dstdir"
    # Clean up
    rm -rf "$CURRENT_PATH/.tmp"
    return 0
}

# Define a function for the exportpack functionality
do_exportpack() {
    do_prepare_export
    echo "Starting exportpack..."
   
    echo "exporting pck..."
    spx export 
    
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
    # Clean up
    rm -rf "$CURRENT_PATH/.tmp"
    return 0
}

# Define a function for the runweb functionality
do_runweb() {
    local target_path="$1"
    
    # Default path if not provided
    if [ -z "$target_path" ]; then
        target_path="tutorial/01-Weather"
    fi
    
    echo "Starting runweb with path: $target_path"
    
    # Kill any running gdspx_web_server.py processes
    echo "Killing gdspx_web_server.py if running..."
    
    # 检测操作系统类型
    if [ "$OS" = "Windows_NT" ] || [[ "$(uname -s 2>/dev/null)" == MINGW* ]]; then
        # Windows 环境 - 使用 taskkill 命令
        echo "Windows environment detected, using taskkill"
        # 直接执行 taskkill 命令
        taskkill /F /FI "IMAGENAME eq python.exe" 2>/dev/null || true
        taskkill /F /FI "IMAGENAME eq pythonw.exe" 2>/dev/null || true
        taskkill /F /FI "IMAGENAME eq python3.exe" 2>/dev/null || true
    elif command -v pgrep > /dev/null; then
        # Unix/Linux 环境 - 使用 pgrep 和 kill
        PIDS=$(pgrep -f gdspx_web_server.py)
        if [ -n "$PIDS" ]; then
            echo "Killing process: $PIDS"
            kill -9 $PIDS
        else
            echo "No gdspx_web_server.py process found."
        fi
    else
        echo "Neither taskkill nor pgrep available, skipping process killing"
    fi
    
    # Run cmdweb and start the web server
    (cd "$PROJ_DIR/cmd/gox/" && ./install.sh --web) 
    (cd "$CURRENT_PATH/$target_path" && spx clear && spx runweb -serveraddr=":$2") || {
        echo "Error: Failed to run web server"
        return 1
    }
    
    echo "runweb completed successfully"
    return 0
}

# Main function to handle arguments
main() {
    if [ $# -eq 0 ]; then
        echo "Usage: $0 [command] [options]"
        echo "Commands:"
        echo "  exportweb - Create a web release package"
        echo "  exportpack  - Set up and package the application"
        echo "  runweb [path] - Run a web server (default path: tutorial/01-Weather)"
        return 1
    fi

    command="$1"
    shift

    case "$command" in
        exportweb)
            do_exportweb
            ;;
        exportpack)
            do_exportpack
            ;;
        extrawebtemplate)
            do_extra_webtemplate
            ;;
        runweb)
            do_runweb "$1" "$2"
            ;;
        *)
            echo "Unknown command: $command"
            echo "Available commands: exportweb, exportpack, runweb"
            return 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"
