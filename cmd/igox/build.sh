#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

# Pin Go toolchain version
export GOTOOLCHAIN=go1.24.4

# Function to install and check binaryen
install_binaryen() {
    # Check if wasm-opt is installed
    if command -v wasm-opt &> /dev/null; then
        echo "wasm-opt is already installed"
        return 0
    else
        echo "wasm-opt not detected, trying to install binaryen..."
        
        # Install binaryen based on the system type
        case "$SYSTEM" in
            Linux)
                # Detect Linux distribution
                if [ -f /etc/os-release ]; then
                    . /etc/os-release
                    if [[ "$ID" == "ubuntu" || "$ID" == "debian" ]]; then
                        echo "Detected Ubuntu/Debian system, installing binaryen with apt"
                        sudo apt-get update && sudo apt-get install -y binaryen
                    elif [[ "$ID" == "fedora" || "$ID" == "rhel" || "$ID" == "centos" ]]; then
                        echo "Detected Fedora/RHEL/CentOS system, installing binaryen with dnf/yum"
                        sudo dnf install -y binaryen || sudo yum install -y binaryen
                    else
                        echo "Unrecognized Linux distribution, trying to install with apt"
                        sudo apt-get update && sudo apt-get install -y binaryen
                    fi
                else
                    echo "Unable to determine Linux distribution, trying to install with apt"
                    sudo apt-get update && sudo apt-get install -y binaryen
                fi
                ;;
            Darwin)
                echo "Detected macOS system, installing binaryen with Homebrew"
                brew install binaryen
                ;;
            *)
                echo "Unrecognized operating system: $SYSTEM, cannot automatically install binaryen"
                ;;
        esac
        
        # Check again if installation was successful
        if command -v wasm-opt &> /dev/null; then
            echo "binaryen installation successful"
            return 0
        else
            echo "binaryen installation failed, will use basic compilation version"
            return 1
        fi
    fi
}

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

echo "Generating ispx wraps..."
# Install required Go dependencies
if ! go generate embedded_pkgs.go > /dev/null 2>&1; then
    echo "Error during go generate, showing full output:"
    go generate embedded_pkgs.go
fi
go mod tidy

# Detect system type
SYSTEM="$(uname -s)"

# Choose build method based on --opt parameter and binaryen installation status
if [ "$1" = "--opt" ]; then
    # Try to install/check binaryen
    if install_binaryen; then
        echo "Building with optimization..."
        GOOS=js GOARCH=wasm go build -tags canvas -trimpath -ldflags "-s -w -checklinkname=0" -o gdspx_raw.wasm
        wasm-opt -Oz --enable-bulk-memory -o gdspx.wasm gdspx_raw.wasm
        
        # Compress with brotli
        compress_with_brotli "gdspx.wasm" "gdspx.wasm.br"
        
        # Clean up temporary file
        rm -f gdspx_raw.wasm
    else
        echo "binaryen not available, skipping optimization step, using basic compilation..."
        GOOS=js GOARCH=wasm go build -tags canvas -ldflags -checklinkname=0 -o gdspx.wasm
    fi
else
    echo "Building with basic version..."
    GOOS=js GOARCH=wasm go build -tags canvas -ldflags -checklinkname=0 -o gdspx.wasm
fi 

echo "gdspx.wasm has been created"
