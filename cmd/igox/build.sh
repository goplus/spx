#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

# Pin Go toolchain version
export GOTOOLCHAIN=go1.24.4

# Install required Go dependencies
go generate embedded_pkgs.go
go mod tidy

# Detect system type
SYSTEM="$(uname -s)"
BINARYEN_INSTALLED=false

# Check if wasm-opt is installed
if command -v wasm-opt &> /dev/null; then
    echo "wasm-opt is already installed"
    BINARYEN_INSTALLED=true
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
        BINARYEN_INSTALLED=true
    else
        echo "binaryen installation failed, will use basic compilation version"
    fi
fi

# Choose build method based on --opt parameter and binaryen installation status
if [ "$1" = "--opt" ] && $BINARYEN_INSTALLED; then
    echo "Building with optimization..."
    GOOS=js GOARCH=wasm go build -tags canvas -trimpath -ldflags "-s -w -checklinkname=0" -o gdspx_raw.wasm
    wasm-opt -Oz --enable-bulk-memory -o gdspx.wasm gdspx_raw.wasm
else 
    if [ "$1" = "--opt" ] && ! $BINARYEN_INSTALLED; then
        echo "binaryen not installed, skipping optimization step, using basic compilation..."
    else
        echo "Building with basic version..."
    fi
    GOOS=js GOARCH=wasm go build -tags canvas -ldflags -checklinkname=0 -o gdspx.wasm
fi 

echo "gdspx.wasm has been created"
