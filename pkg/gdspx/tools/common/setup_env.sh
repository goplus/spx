#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_DIR="$SCRIPT_DIR/.."

setup_global_variables() {
    local DEFAULT_PLATFORM=""
    
    # Define Godot version
    VERSION=2.0.1
    ENGINE_VERSION=4.2.2.stable
    GOPATH=$(go env GOPATH)
    PROJ_DIR=$SCRIPT_DIR/..
    ENGINE_DIR=$PROJ_DIR/godot

    cd $PROJ_DIR
    echo "version=$VERSION GOPATH=$GOPATH"

    echo "PROJ_DIR=$PROJ_DIR"
    echo "ENGINE_DIR=$ENGINE_DIR"
    echo "ENGINE_VERSION=$ENGINE_VERSION"
    echo "GOPATH=$GOPATH"
    echo "VERSION=$VERSION"
    
    echo "Detecting platform..."
    echo "$(uname)"
    
    # Set up platform-specific variables
    if [[ "$(uname)" == "Linux" ]]; then
        export TEMPLATE_DIR="$HOME/.local/share/godot/export_templates/$ENGINE_VERSION"
        DEFAULT_PLATFORM="linux"
    elif [[ "$(uname)" == "Darwin" ]]; then  # macOS
        export TEMPLATE_DIR="$HOME/Library/Application Support/Godot/export_templates/$ENGINE_VERSION"
        DEFAULT_PLATFORM="macos"
    elif [[ "$(uname -o 2>/dev/null)" == "Msys" ]] || [[ "$(uname -o 2>/dev/null)" == "Cygwin" ]]; then
        # Windows (Git Bash, Cygwin, MSYS)
        export TEMPLATE_DIR="$APPDATA/Godot/export_templates/$ENGINE_VERSION"
        DEFAULT_PLATFORM="windows"
    else
        echo "Unsupported OS"
        exit 1
    fi
    
    # Create destination directory
    mkdir -p "$TEMPLATE_DIR"
    
    # Detect architecture
    export ARCH="x86_64"
    if [[ "$(uname -m)" == "aarch64" || "$(uname -m)" == "arm64" ]]; then
        export ARCH="arm64"
    fi
    
    # Set default platform if not already set
    if [ -z "$PLATFORM" ]; then
        export PLATFORM="$DEFAULT_PLATFORM"
    fi
    
    echo "Platform: $PLATFORM"
    echo "Architecture: $ARCH"
    echo "Destination directory: $TEMPLATE_DIR"



    return 0
}


download_engine() {
    cd $PROJ_DIR
    if [ ! -d "godot" ]; then
        echo "Godot directory not found. Creating and initializing..."
        mkdir godot
        cd godot
        git init 
        git remote add origin https://github.com/goplus/godot.git
        git fetch --depth 1 origin spx4.2.2
        git checkout spx4.2.2
        echo "Godot repository setup complete."
    else
        cd godot
        echo "Godot directory already exists."
    fi

}

prepare_env() {
    setup_global_variables

    if command -v python3 &>/dev/null; then
        PYTHON=python3
    elif command -v python &>/dev/null; then
        PYTHON=python
    else
        echo "Neither python3 nor python is installed."
        exit 1
    fi

    $PYTHON -c "import sys; print(sys.version)"
    $PYTHON -m pip install scons==4.7.0 --break-system-packages


    scons --version

    if [[ "$(uname)" == "Darwin" ]]; then
        echo "install macos vulkan sdk"
        $ENGINE_DIR/misc/scripts/install_vulkan_sdk_macos.sh
    fi 
    download_engine


}




# Function to check and ensure JDK 17 is installed
ensure_jdk() {
    echo "Checking JDK 17 installation..."
    # Check if java is installed and version is 17
    if command -v java >/dev/null 2>&1; then
        export PATH="/opt/homebrew/opt/openjdk@17/bin:$PATH"
        JAVA_VERSION=$(java -version 2>&1 | grep -i version | cut -d'"' -f2 | cut -d'.' -f1)
        if [ "$JAVA_VERSION" = "17" ]; then
            echo "JDK 17 is already installed."
            return 0
        fi
    fi
    
    echo "JDK 17 not found. Installing..."
    
    # Install JDK 17 based on OS
    if [[ "$(uname)" == "Linux" ]]; then
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update
            sudo apt-get install -y openjdk-17-jdk
        elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y java-17-openjdk-devel
        elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y java-17-openjdk-devel
        else
            echo "Unsupported Linux distribution. Please install JDK 17 manually."
            exit 1
        fi
    elif [[ "$(uname)" == "Darwin" ]]; then  # macOS
        if command -v brew >/dev/null 2>&1; then
            brew install openjdk@17
            export PATH="/opt/homebrew/opt/openjdk@17/bin:$PATH"
        else
            echo "Homebrew not found. Please install Homebrew first or install JDK 17 manually."
            echo "You can download it from: https://adoptium.net/temurin/releases/?version=17"
            exit 1
        fi
    elif [[ "$(uname -o 2>/dev/null)" == "Msys" ]] || [[ "$(uname -o 2>/dev/null)" == "Cygwin" ]]; then
        echo "On Windows, please install JDK 17 manually from: https://adoptium.net/temurin/releases/?version=17"
        echo "After installation, ensure JAVA_HOME is set correctly in your environment variables."
        read -p "Press Enter to continue once JDK 17 is installed..." 
    else
        echo "Unsupported OS. Please install JDK 17 manually."
        exit 1
    fi
    
    # Verify installation
    if command -v java >/dev/null 2>&1; then
        JAVA_VERSION=$(java -version 2>&1 | grep -i version | cut -d'"' -f2 | cut -d'.' -f1)
        if [ "$JAVA_VERSION" = "17" ]; then
            echo "JDK 17 installed successfully."
            return 0
        fi
    fi
    
    echo "Failed to install JDK 17. Please install it manually."
    exit 1
}

# Function to setup emsdk for web builds
ensure_emsdk() {
    local EMSDK_VERSION="3.1.39"
    local EMSDK_DIR=""
    
    # Determine global installation directory based on platform
    if [[ "$(uname)" == "Linux" ]]; then
        EMSDK_DIR="$HOME/.local/share/emsdk"
    elif [[ "$(uname)" == "Darwin" ]]; then  # macOS
        EMSDK_DIR="$HOME/Library/Application Support/emsdk"
    elif [[ "$(uname -o 2>/dev/null)" == "Msys" ]] || [[ "$(uname -o 2>/dev/null)" == "Cygwin" ]]; then
        # Windows (Git Bash, Cygwin, MSYS)
        EMSDK_DIR="$APPDATA/emsdk"
    else
        echo "Unsupported OS for emsdk installation"
        exit 1
    fi
    
    echo "Using emsdk installation directory: $EMSDK_DIR"
    
    # Create the directory if it doesn't exist
    mkdir -p "$EMSDK_DIR"
    
    # Check if emsdk is already installed in the global location
    if [ ! -d "$EMSDK_DIR/emsdk" ]; then
        echo "emsdk not found in global location, installing emsdk..."
        cd "$EMSDK_DIR" || exit
        git clone git@github.com:emscripten-core/emsdk.git
        cd emsdk || exit
        ./emsdk install $EMSDK_VERSION
        ./emsdk activate $EMSDK_VERSION
    else
        # emsdk exists, just activate it
        cd "$EMSDK_DIR/emsdk" || exit
        ./emsdk activate $EMSDK_VERSION
    fi
    
    # Set up environment variables based on platform
    if [[ "$(uname -o 2>/dev/null)" == "Msys" ]] || [[ "$(uname -o 2>/dev/null)" == "Cygwin" ]]; then
        # Windows
        source ./emsdk_env.sh
        export PATH="$EMSDK_DIR/emsdk/upstream/emscripten:$PATH"
    else
        # Linux and macOS
        source ./emsdk_env.sh
    fi
      
    # Verify installation
    if command -v em++ >/dev/null 2>&1; then
        echo "emsdk is set up successfully:"
        em++ --version
        return 0
    else
        echo "Failed to set up emsdk. Please check the installation."
        exit 1
    fi
}




# Call the function
prepare_env