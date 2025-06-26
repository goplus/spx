#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_DIR="$SCRIPT_DIR/.."

setup_global_variables() {
    xgo version || true
    go version || true

    local DEFAULT_PLATFORM=""

    # Define Godot version
    VERSION=$(cat $SCRIPT_DIR/version)
    ENGINE_GIT_TAG="spx"$VERSION
    ENGINE_VERSION=4.2.2.stable
    if [ "$OS" = "Windows_NT" ]; then
        IFS=';' read -r first_gopath _ <<< "$(go env GOPATH)"
        GOPATH="$first_gopath"
    else
        IFS=':' read -r first_gopath _ <<< "$(go env GOPATH)"
        GOPATH="$first_gopath"
    fi

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
    if [[ "$(uname -m)" == "i386" || "$(uname -m)" == "i686" ]]; then
        export ARCH="x86_32"
    fi

    curOS=""
    case "$(uname -s)" in
        Linux*)     curOS="linux" ;;
        Darwin*)    curOS="macOS" ;;
        CYGWIN*|MINGW*|MSYS*) curOS="windows" ;;
        *)          curOS="Unknown" ;;
    esac

    ARCH=""
    case "$curOS" in
        "linux"|"macOS")
            RAW_ARCH=$(uname -m)
            case "$RAW_ARCH" in
                x86_64)     ARCH="x86_64" ;;
                i386|i686)  ARCH="x86_32" ;;
                aarch64)    ARCH="arm64" ;;
                armv7l|arm) ARCH="arm32" ;;
                *)
                    if [ "$OS" = "macOS" ]; then
                        if sysctl -n machdep.cpu.brand_string | grep -qi "Apple"; then
                            ARCH="arm64"
                        else
                            ARCH="$RAW_ARCH"
                        fi
                    else
                        ARCH="$RAW_ARCH"
                    fi
                    ;;
            esac
            ;;

        "windows")
            if [ "$PROCESSOR_ARCHITECTURE" = "AMD64" ] || [ "$PROCESSOR_ARCHITEW6432" = "AMD64" ]; then
                ARCH="x86_64"
            elif [ "$PROCESSOR_ARCHITECTURE" = "x86" ]; then
                ARCH="x86_32"
            elif [ "$PROCESSOR_ARCHITECTURE" = "ARM64" ]; then
                ARCH="arm64"
            else
                ARCH="Unknown"
            fi
            ;;

        *)
            ARCH="Unknown"
            ;;
    esac


    # Set default platform if not already set
    if [ -z "$PLATFORM" ]; then
        export PLATFORM="$DEFAULT_PLATFORM"
    fi
    
    echo "Platform: $PLATFORM"
    echo "Architecture: $ARCH"
    echo "Destination directory: $TEMPLATE_DIR"
    echo "Source tag: $ENGINE_GIT_TAG"
    return 0
}


download_engine() {
    cd $PROJ_DIR
    if [ ! -d "godot" ]; then
        echo "Godot directory not found. Creating and initializing..."
        git clone --depth 1 --branch $ENGINE_GIT_TAG https://github.com/goplus/godot.git
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
    download_engine
    if [[ "$(uname)" == "Darwin" ]]; then
        # check if vulkan sdk is already installed
        if command -v vulkaninfo &>/dev/null || [ -d "$HOME/VulkanSDK" ]; then
            echo "vulkan sdk already installed, skip installation"
        else
            echo "install macos vulkan sdk"
            $ENGINE_DIR/misc/scripts/install_vulkan_sdk_macos.sh
        fi
    fi 
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
    local EMSDK_VERSION="3.1.62"
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
        # emsdk exists, check if version matches
        cd "$EMSDK_DIR/emsdk" || exit
        
        # activate environment variables to make emcc available
        source ./emsdk_env.sh &> /dev/null
        
        # check if emcc is available and its version
        if command -v emcc &> /dev/null; then
            CURRENT_VERSION=$(emcc --version | head -n 1 | awk '{print $3}')
            echo "current emcc version: $CURRENT_VERSION, target version: $EMSDK_VERSION"
            
            # compare versions
            if [ "$CURRENT_VERSION" != "$EMSDK_VERSION" ]; then
                echo "emcc version mismatch, installing target version $EMSDK_VERSION..."
                ./emsdk install $EMSDK_VERSION
                ./emsdk activate $EMSDK_VERSION
            else
                echo "emcc version matches, no need to re-install"
                ./emsdk activate $EMSDK_VERSION
            fi
        else
            echo "emcc not found, activating emsdk..."
            ./emsdk activate $EMSDK_VERSION
        fi
    fi
    
    # Set up environment variables based on platform
    if [[ "$(uname -o 2>/dev/null)" == "Msys" ]] || [[ "$(uname -o 2>/dev/null)" == "Cygwin" ]]; then
        # Windows
        source ./emsdk_env.sh
        # Windows path check, need to consider backslashes (\) and possible different path prefixes
        # Get the actual path of emscripten
        EMSCRIPTEN_PATH="$(cd "$EMSDK_DIR/emsdk/upstream/emscripten" 2>/dev/null && pwd -W 2>/dev/null || echo "$EMSDK_DIR/emsdk/upstream/emscripten")"
        
        # First check if this path already exists in PATH (considering path separators)
        PATH_FOUND=0
        IFS=':;' read -ra PATH_DIRS <<< "$PATH"
        for p in "${PATH_DIRS[@]}"; do
            # Convert paths to lowercase for comparison (Windows is case-insensitive)
            p_lower=$(echo "$p" | tr '[:upper:]' '[:lower:]')
            emscripten_lower=$(echo "$EMSCRIPTEN_PATH" | tr '[:upper:]' '[:lower:]')
            
            # Convert backslashes to forward slashes for easier comparison
            p_normalized=${p_lower//\\/\/}
            emscripten_normalized=${emscripten_lower//\\/\/}
            
            if [[ "$p_normalized" == "$emscripten_normalized" ]]; then
                PATH_FOUND=1
                break
            fi
        done
        
        if [[ $PATH_FOUND -eq 0 ]]; then
            # Path doesn't exist, add it to PATH
            export PATH="$EMSCRIPTEN_PATH:$PATH"
            echo "Added emscripten to PATH: $EMSCRIPTEN_PATH"
        else
            echo "Emscripten path already in PATH"
        fi
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

