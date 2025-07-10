#!/bin/bash

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJ_DIR="$SCRIPT_DIR/../../.."
echo $PROJ_DIR

# Function to compress wasm file with brotli
compress_with_brotli() {
    local input_file="$1"
    local output_file="$2"
    
    if [ -z "$input_file" ] || [ -z "$output_file" ]; then
        echo "Error: compress_with_brotli requires input and output file parameters"
        return 1
    fi
    
    if [ ! -f "$input_file" ]; then
        echo "Error: Input file $input_file does not exist"
        return 1
    fi
    
    # Check if brotli is installed
    local brotli_installed=false
    if command -v brotli &> /dev/null; then
        echo "brotli is already installed"
        brotli_installed=true
    else
        echo "brotli not detected, trying to install..."
        
        # Install brotli based on the system type
        case "$SYSTEM" in
            Linux)
                # Detect Linux distribution
                if [ -f /etc/os-release ]; then
                    . /etc/os-release
                    if [[ "$ID" == "ubuntu" || "$ID" == "debian" ]]; then
                        echo "Detected Ubuntu/Debian system, installing brotli with apt"
                        sudo apt-get update && sudo apt-get install -y brotli
                    elif [[ "$ID" == "fedora" || "$ID" == "rhel" || "$ID" == "centos" ]]; then
                        echo "Detected Fedora/RHEL/CentOS system, installing brotli with dnf/yum"
                        sudo dnf install -y brotli || sudo yum install -y brotli
                    else
                        echo "Unrecognized Linux distribution, trying to install with apt"
                        sudo apt-get update && sudo apt-get install -y brotli
                    fi
                else
                    echo "Unable to determine Linux distribution, trying to install with apt"
                    sudo apt-get update && sudo apt-get install -y brotli
                fi
                ;;
            Darwin)
                echo "Detected macOS system, installing brotli with Homebrew"
                brew install brotli
                ;;
            *)
                echo "Unrecognized operating system: $SYSTEM, cannot automatically install brotli"
                ;;
        esac
        
        # Check again if installation was successful
        if command -v brotli &> /dev/null; then
            echo "brotli installation successful"
            brotli_installed=true
        else
            echo "brotli installation failed, will skip compression step"
        fi
    fi
    
    if $brotli_installed; then
        echo "Compressing $input_file with brotli..."
        brotli -q 11 -o "$output_file" "$input_file"
        if [ $? -eq 0 ]; then
            echo "$output_file has been created"
            return 0
        else
            echo "Error: brotli compression failed"
            return 1
        fi
    else
        echo "brotli not available, skipping compression"
        return 1
    fi
}

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
    local mode="${1:-default}"
    do_prepare_export
    dstdir="$GOPATH/bin/gdspxrt"$TEMP_VERSION"_web"$mode
    echo "exporting web runtime..."
    
    spx exporttemplateweb 

    rm -rf "$dstdir" 
    cp -rf ./project/.builds/webi  "$dstdir" 
    mv "$dstdir/engine.pck" "$dstdir/engine.zip"

    # write mode to engine.js
    engine_mode_define="var EnginePackMode = '$mode';"
    echo "engine_mode_define: $engine_mode_define"
    temp_file=$(mktemp)
    echo "$engine_mode_define" > "$temp_file"
    cat "$dstdir/engine.js" >> "$temp_file"
    mv "$temp_file" "$dstdir/engine.js"

    echo "exporting web runtime done: $dstdir (mode: $mode)"

    # Clean up
    rm -rf "$CURRENT_PATH/.tmp"
    return 0
}

do_compresswasm() {
    TEMP_VERSION=$(cat "$CURRENT_PATH/cmd/gox/template/version") 
    dstdir="$GOPATH/bin/gdspxrt"$TEMP_VERSION"_web"
    rm -rf "$dstdir/engine.wasm.br"
    rm -rf "$dstdir/../gdspx.wasm.br"
    compress_with_brotli "$dstdir/engine.wasm" "$dstdir/engine.wasm.br"
    compress_with_brotli "$dstdir/../gdspx.wasm" "$dstdir/../gdspx.wasm.br"
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


# Main function to handle arguments
main() {
    if [ $# -eq 0 ]; then
        echo "Usage: $0 [command] [options]"
        echo "Commands:"
        echo "  exportweb - Create a web release package"
        echo "  exportpack  - Set up and package the application"
        echo "  extrawebtemplate [mode] - Export web runtime template (mode: worker|main|default)"
        echo "  compresswasm - Compress WASM files with brotli"
        echo "  runweb [path] [port] - Run a web server (default path: tutorial/01-Weather, default port: 8106)"
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
        compresswasm)
            do_compresswasm
            ;;
        extrawebtemplate)
            mode="$1"
            do_extra_webtemplate "$mode"
            ;;
        *)
            echo "Unknown command: $command"
            echo "Available commands: exportweb, exportpack, extrawebtemplate, compresswasm, runweb"
            return 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"
