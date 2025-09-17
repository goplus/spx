#!/bin/bash

# SPX Game Engine One-Click Installation Script (macOS/Linux)
# Usage: ./init.bash

set -e  # Exit immediately on error

echo "🎮 SPX Game Engine One-Click Installation Script"
echo "================================================"

# Detect operating system
OS="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    echo "❌ Unsupported operating system: $OSTYPE"
    exit 1
fi

echo "📍 Detected operating system: $OS"

# Function: Check if command exists
check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        echo "✅ $1 is installed"
        return 0
    else
        echo "❌ $1 is not installed"
        return 1
    fi
}

# Function: Check Go version
check_go_version() {
    if check_command go; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        REQUIRED_VERSION="1.23.0"
        if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
            echo "✅ Go version meets requirement: $GO_VERSION"
            return 0
        else
            echo "❌ Go version does not meet requirement, need >= $REQUIRED_VERSION, current: $GO_VERSION"
            return 1
        fi
    fi
    return 1
}

# Step 1: Check and install Go
echo ""
echo "🔍 Step 1/5: Checking Go installation..."
if ! check_go_version; then
    echo "📦 Installing Go v1.23.0..."
    
    if [[ "$OS" == "macos" ]]; then
        # macOS installation
        if check_command brew; then
            echo "Installing Go using Homebrew..."
            brew install go@1.23 || {
                echo "Homebrew installation failed, trying to download from official website..."
                GO_PKG="go1.23.0.darwin-amd64.pkg"
                curl -L "https://golang.org/dl/$GO_PKG" -o "/tmp/$GO_PKG"
                echo "Please manually install the downloaded package: /tmp/$GO_PKG"
                echo "After installation, please run this script again"
                exit 1
            }
        else
            echo "Installing Homebrew..."
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            brew install go@1.23
        fi
    elif [[ "$OS" == "linux" ]]; then
        # Linux installation
        echo "Downloading Go from official website..."
        GO_TAR="go1.23.0.linux-amd64.tar.gz"
        wget "https://golang.org/dl/$GO_TAR" -O "/tmp/$GO_TAR"
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "/tmp/$GO_TAR"
        
        # Add to PATH
        if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
            echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
        fi
        export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    fi
    
    # Verify installation
    if ! check_go_version; then
        echo "❌ Go installation failed, please install manually and run script again"
        exit 1
    fi
fi

# Step 2: Check and install make
echo ""
echo "🔍 Step 2/5: Checking make tool..."
if ! check_command make; then
    echo "📦 Installing make..."
    
    if [[ "$OS" == "macos" ]]; then
        echo "Installing Xcode Command Line Tools..."
        xcode-select --install || {
            if check_command brew; then
                brew install make
            else
                echo "❌ Please manually install Xcode Command Line Tools"
                exit 1
            fi
        }
    elif [[ "$OS" == "linux" ]]; then
        if check_command apt-get; then
            sudo apt-get update && sudo apt-get install -y build-essential
        elif check_command yum; then
            sudo yum groupinstall -y "Development Tools"
        elif check_command pacman; then
            sudo pacman -S --needed base-devel
        else
            echo "❌ Unsupported Linux distribution, please install make manually"
            exit 1
        fi
    fi
fi

# Step 3: Check and install git
echo ""
echo "🔍 Step 3/5: Checking git..."
if ! check_command git; then
    echo "📦 Installing git..."
    
    if [[ "$OS" == "macos" ]]; then
        if check_command brew; then
            brew install git
        else
            echo "git is usually installed with Xcode Command Line Tools"
        fi
    elif [[ "$OS" == "linux" ]]; then
        if check_command apt-get; then
            sudo apt-get install -y git
        elif check_command yum; then
            sudo yum install -y git
        elif check_command pacman; then
            sudo pacman -S git
        fi
    fi
fi

# Step 4: Install xgo
echo ""
echo "🔍 Step 4/5: Installing xgo..."
if [ ! -d "xgo" ]; then
    echo "📦 Cloning xgo repository..."
    git clone https://github.com/goplus/xgo.git
fi

cd xgo
echo "🔧 Building and installing xgo..."
./all.bash
cd ..

# Step 5: Initialize SPX environment
echo ""
echo "🔍 Step 5/5: Initializing SPX environment..."
echo "🔧 Running make setup..."
make setup

# Verify installation
echo ""
echo "🧪 Verifying installation..."
if check_command spx; then
    spx version
    echo ""
    echo "🎉 SPX Game Engine installation successful!"
    echo ""
    echo "📚 Next steps:"
    echo "   1. Run example game: cd tutorial/00-Hello && spx run"
    echo "   2. View all examples: make list-demos"
    echo "   3. Read documentation: docs/zh/README.md"
else
    echo "❌ SPX installation may not be complete, please check error messages"
    exit 1
fi