#!/bin/bash

# SPX Game Engine One-Click Installation Script (macOS/Linux)
# Usage: ./init.bash

set -e  # Exit immediately on error

echo "🎮 SPX Game Engine One-Click Installation Script"
echo "================================================"

# Detect operating system
OS="unknown"
if [[ "$OSTYPE" == "linux"* ]]; then
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
echo "🔍 Step 1/6: Checking Go installation..."
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

# Step 2: Install Linux dependencies (matching GitHub Actions)
if [[ "$OS" == "linux" ]]; then
    echo ""
    echo "🔍 Step 2/6: Installing Linux system dependencies..."
    echo "📦 Installing graphics and audio libraries..."
    
    if check_command apt-get; then
        sudo apt-get update && sudo apt-get install -y gcc libgl1-mesa-dev libegl1-mesa-dev libgles2-mesa-dev libx11-dev xorg-dev libasound2-dev libopenal-dev
    elif check_command yum; then
        sudo yum install -y gcc mesa-libGL-devel mesa-libEGL-devel mesa-libGLES-devel libX11-devel xorg-x11-server-devel alsa-lib-devel openal-soft-devel
    elif check_command pacman; then
        sudo pacman -S --needed gcc mesa xorg-server-devel alsa-lib openal
    else
        echo "❌ Unsupported Linux distribution, please install dependencies manually"
        echo "Required packages: gcc libgl1-mesa-dev libegl1-mesa-dev libgles2-mesa-dev libx11-dev xorg-dev libasound2-dev libopenal-dev"
        exit 1
    fi
fi

# Step 3: Check and install make
echo ""
echo "🔍 Step 3/6: Checking make tool..."
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

# Step 4: Check and install git
echo ""
echo "🔍 Step 4/6: Checking git..."
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

# Step 5: Install xgo v1.5.0 (matching GitHub Actions)
echo ""
echo "🔍 Step 5/6: Installing xgo v1.5.0..."
if [ ! -d "xgo" ]; then
    echo "📦 Cloning xgo v1.5.0 repository..."
    git clone --depth 1 --branch v1.5.0 https://github.com/goplus/xgo.git
else
    echo "📦 xgo directory exists, ensuring correct version..."
    cd xgo
    git fetch --depth 1 origin v1.5.0
    git checkout v1.5.0
    cd ..
fi

cd xgo
echo "🔧 Building and installing xgo v1.5.0..."
./all.bash
cd ..

# Step 6: Initialize SPX environment and run tests (matching GitHub Actions)
echo ""
echo "🔍 Step 6/6: Initializing SPX environment and running tests..."

# Setup SPX environment (matching .github/actions/deps)
echo "🔧 Running make setup..."
make setup

# Verify spx installation
if ! check_command spx; then
    echo "❌ SPX installation failed"
    exit 1
fi

echo "✅ SPX setup completed, starting tests..."

# Run tests (matching .github/actions/project-test)
echo ""
echo "🔨 Step 6a: Building project..."
go build -v $(go list ./... | grep -v /internal/webffi)
if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi
echo "✅ Build completed successfully"

echo ""
echo "🧪 Step 6b: Running tests..."
go test -v $(go list ./... | grep -v /internal/webffi)
if [ $? -ne 0 ]; then
    echo "❌ Tests failed"
    exit 1
fi
echo "✅ Tests completed successfully"

echo ""
echo "🔧 Step 6c: Running xgo compilation..."
xgo go ./...
if [ $? -ne 0 ]; then
    echo "❌ xgo compilation failed"
    exit 1
fi
echo "✅ xgo compilation completed successfully"

echo ""
echo "🎮 Step 6d: Running test demo..."
spx run -path="test/CI" -headless=true &>cilog.txt &
TEST_PID=$!

# Wait for test to complete (max 30 seconds)
echo "Waiting for test demo to complete..."
for i in {1..30}; do
    if grep -q "===>SpxCIRunSucc" cilog.txt 2>/dev/null; then
        echo "✅ Test demo completed successfully"
        break
    fi
    if ! kill -0 $TEST_PID 2>/dev/null; then
        echo "❌ Test demo process terminated unexpectedly"
        cat cilog.txt
        rm -f cilog.txt
        exit 1
    fi
    sleep 1
done

# Check final result
if grep -q "===>SpxCIRunSucc" cilog.txt; then
    echo "✅ Test demo verification passed"
    rm -f cilog.txt
else
    echo "❌ Test demo failed: success mark not found"
    echo "Log contents:"
    cat cilog.txt
    rm -f cilog.txt
    exit 1
fi

# Kill test process if still running
if kill -0 $TEST_PID 2>/dev/null; then
    kill $TEST_PID
    wait $TEST_PID 2>/dev/null
fi

echo ""
echo "🎉 SPX Game Engine installation and testing completed successfully!"
echo ""
echo "📚 Next steps:"
echo "   1. Run example game: cd tutorial/00-Hello && spx run"
echo "   2. View all examples: make list-demos"
echo "   3. Read documentation: docs/zh/README.md"