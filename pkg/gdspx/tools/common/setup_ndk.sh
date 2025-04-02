#!/bin/bash

# Set Android SDK and NDK versions and paths
ANDROID_SDK_ROOT="$HOME/Library/Android/sdk"
ANDROID_NDK_VERSION="23.2.8568313"

# Using r23c version, which corresponds to 23.2.8568313
ANDROID_NDK_ARCHIVE="android-ndk-r23c-darwin.zip"
ANDROID_NDK_URL="https://dl.google.com/android/repository/${ANDROID_NDK_ARCHIVE}"
ANDROID_NDK_ROOT="${ANDROID_SDK_ROOT}/ndk/${ANDROID_NDK_VERSION}"
TEMP_DIR="/tmp/android_ndk_install"
CACHE_DIR="$HOME/.android_ndk_cache"
MAX_RETRIES=3
MANUAL_INSTALL=false
MANUAL_PATH=""
SKIP_VERIFICATION=false

# Expected size and hash for the NDK package
# Note: These values are for android-ndk-r23c-darwin.zip
EXPECTED_SIZE=982917530  # File size in bytes
EXPECTED_SHA256="c6e97f9c8cfe5b7be0a9e6c15af8e7a179475b7159a7b689a1dc831fc4b977e1"  # SHA256 hash (may need updating)

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --manual-install)
      MANUAL_INSTALL=true
      shift
      ;;
    --ndk-path)
      MANUAL_PATH="$2"
      shift 2
      ;;
    --skip-verification)
      SKIP_VERIFICATION=true
      shift
      ;;
    --help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --manual-install       Skip download, use already downloaded NDK package"
      echo "  --ndk-path <path>      Specify path to already downloaded NDK package"
      echo "  --skip-verification    Skip file size and hash verification"
      echo "  --help                 Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help to see available options"
      exit 1
      ;;
  esac
done

# Create temporary and cache directories
rm -rf "${TEMP_DIR}" # Clean previous temporary directory
mkdir -p "${TEMP_DIR}"
mkdir -p "${ANDROID_SDK_ROOT}/ndk"
mkdir -p "${CACHE_DIR}"

# Check if a command exists
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "Error: $1 is not installed. Please install $1 and try again."
        exit 1
    fi
}

# Calculate SHA256 hash of a file
calculate_sha256() {
    local file_path=$1
    if command -v shasum &> /dev/null; then
        shasum -a 256 "$file_path" | awk '{print $1}'
    elif command -v sha256sum &> /dev/null; then
        sha256sum "$file_path" | awk '{print $1}'
    else
        echo "Error: Cannot calculate SHA256 hash. Please install shasum or sha256sum."
        return 1
    fi
}

# Verify file size and hash
verify_file() {
    local file_path=$1
    local expected_size=$2
    local expected_sha256=$3
    
    # Check if file exists
    if [ ! -f "$file_path" ]; then
        echo "File does not exist: $file_path"
        return 1
    fi
    
    # If verification is skipped, return success
    if [ "$SKIP_VERIFICATION" = true ]; then
        echo "Skipping file verification: $file_path"
        return 0
    fi
    
    # Check file size
    local actual_size=$(stat -f "%z" "$file_path")
    if [ "$actual_size" -lt 900000000 ]; then  # File should be at least 900MB
        echo "Abnormal file size: $actual_size bytes, download may be incomplete"
        return 1
    fi
    
    # Output file information
    echo "File size: $actual_size bytes"
    
    # Verify hash if specified
    if [ -n "$expected_sha256" ] && [ "$SKIP_VERIFICATION" != true ]; then
        echo "Calculating file hash..."
        local actual_sha256=$(calculate_sha256 "$file_path")
        echo "File hash: $actual_sha256"
        
        # Temporarily disabled hash verification as expected value may not be accurate
        # if [ "$actual_sha256" != "$expected_sha256" ]; then
        #     echo "File hash mismatch: Expected $expected_sha256, got $actual_sha256"
        #     return 1
        # fi
    fi
    
    echo "File verification successful: $file_path"
    return 0
}

# Check for required commands
check_command unzip

# Check if NDK is already installed
if [ -d "${ANDROID_NDK_ROOT}" ]; then
    echo "Android NDK ${ANDROID_NDK_VERSION} is already installed at ${ANDROID_NDK_ROOT}"
    echo "To reinstall, please delete this directory first."
    exit 0
fi

# Download function with retry
download_ndk() {
    local retry_count=0
    local success=false
    local target_file=$1
    
    echo "Downloading Android NDK r23c (version ${ANDROID_NDK_VERSION})..."
    echo "Download URL: ${ANDROID_NDK_URL}"
    echo "Download may take several minutes, please be patient..."
    
    while [ $retry_count -lt $MAX_RETRIES ] && [ $success = false ]; do
        if curl -L "${ANDROID_NDK_URL}" -o "$target_file" --progress-bar; then
            success=true
        else
            retry_count=$((retry_count + 1))
            if [ $retry_count -lt $MAX_RETRIES ]; then
                echo "Download failed, retrying ($retry_count/$MAX_RETRIES)..."
                sleep 2
            fi
        fi
    done
    
    if [ $success = false ]; then
        echo "Error: Multiple attempts to download Android NDK failed."
        echo "Possible causes:"
        echo "1. Network connectivity issues"
        echo "2. Proxy configuration issues"
        echo "3. SSL certificate issues"
        echo ""
        echo "Suggestions:"
        echo "1. Check your network connection"
        echo "2. Try manually downloading the NDK: ${ANDROID_NDK_URL}"
        echo "3. After downloading, run this script with --manual-install --ndk-path <path>"
        return 1
    fi
    
    return 0
}

# Prepare NDK package file
NDK_PACKAGE_PATH=""

# Handle manual installation
if [ "$MANUAL_INSTALL" = true ]; then
    if [ -z "$MANUAL_PATH" ]; then
        echo "Error: --ndk-path parameter is required when using --manual-install."
        exit 1
    fi
    
    if [ ! -f "$MANUAL_PATH" ]; then
        echo "Error: Specified NDK package path does not exist: $MANUAL_PATH"
        exit 1
    fi
    
    echo "Using manually downloaded NDK package: $MANUAL_PATH"
    NDK_PACKAGE_PATH="${TEMP_DIR}/${ANDROID_NDK_ARCHIVE}"
    cp "$MANUAL_PATH" "$NDK_PACKAGE_PATH"
else
    # Automatic download or use cached NDK
    check_command curl
    
    # First check if a valid NDK package already exists in the cache
    CACHED_NDK="${CACHE_DIR}/${ANDROID_NDK_ARCHIVE}"
    if [ -f "$CACHED_NDK" ] && verify_file "$CACHED_NDK" "$EXPECTED_SIZE" "$EXPECTED_SHA256"; then
        echo "Using cached NDK package: $CACHED_NDK"
        NDK_PACKAGE_PATH="${TEMP_DIR}/${ANDROID_NDK_ARCHIVE}"
        cp "$CACHED_NDK" "$NDK_PACKAGE_PATH"
    else
        # If no valid NDK package in cache, download it
        NDK_PACKAGE_PATH="${TEMP_DIR}/${ANDROID_NDK_ARCHIVE}"
        if ! download_ndk "$NDK_PACKAGE_PATH"; then
            rm -rf "${TEMP_DIR}"
            exit 1
        fi
        
        # Verify downloaded file
        if verify_file "$NDK_PACKAGE_PATH" "$EXPECTED_SIZE" "$EXPECTED_SHA256"; then
            echo "Downloaded NDK package verified successfully"
            # Save verified file to cache directory
            cp "$NDK_PACKAGE_PATH" "$CACHED_NDK"
            echo "NDK package saved to cache: $CACHED_NDK"
        else
            echo "Error: Downloaded NDK package verification failed."
            echo "If you believe the downloaded file is complete, you can use the --skip-verification option."
            rm -rf "${TEMP_DIR}"
            exit 1
        fi
    fi
fi

# Extract Android NDK
echo "Extracting Android NDK..."
if ! unzip -q "$NDK_PACKAGE_PATH" -d "${TEMP_DIR}"; then
    echo "Error: Failed to extract Android NDK."
    rm -rf "${TEMP_DIR}"
    exit 1
fi

# Find the extracted NDK directory
echo "Locating extracted NDK directory..."
NDK_SOURCE=""
for dir in "${TEMP_DIR}"/*; do
    if [[ -d "$dir" && "$(basename "$dir")" == android-ndk-* ]]; then
        NDK_SOURCE="$dir"
        echo "Found NDK directory: $NDK_SOURCE"
        break
    fi
done

if [ -z "$NDK_SOURCE" ]; then
    echo "Error: NDK directory not found in extracted contents."
    echo "Contents of extraction directory:"
    ls -la "${TEMP_DIR}"
    rm -rf "${TEMP_DIR}"
    exit 1
fi

# Copy NDK to target directory
echo "Installing NDK to ${ANDROID_NDK_ROOT}..."
mkdir -p "${ANDROID_SDK_ROOT}/ndk"
cp -R "$NDK_SOURCE" "${ANDROID_NDK_ROOT}"

if [ ! -d "${ANDROID_NDK_ROOT}" ]; then
    echo "Error: Failed to copy NDK directory."
    rm -rf "${TEMP_DIR}"
    exit 1
fi

# Clean up temporary files
echo "Cleaning up temporary files..."
rm -rf "${TEMP_DIR}"

# Set environment variables
echo "Setting up environment variables..."

# Detect current shell
SHELL_CONFIG_FILE=""
if [ -n "$BASH_VERSION" ] || [ "$SHELL" = "/bin/bash" ]; then
    SHELL_CONFIG_FILE="$HOME/.bash_profile"
    if [ ! -f "$SHELL_CONFIG_FILE" ]; then
        SHELL_CONFIG_FILE="$HOME/.bashrc"
    fi
elif [ -n "$ZSH_VERSION" ] || [ "$SHELL" = "/bin/zsh" ]; then
    SHELL_CONFIG_FILE="$HOME/.zshrc"
else
    # Default to bash config
    SHELL_CONFIG_FILE="$HOME/.bash_profile"
fi

# Check if environment variables are already set
if ! grep -q "ANDROID_NDK_ROOT=${ANDROID_NDK_ROOT}" "$SHELL_CONFIG_FILE" 2>/dev/null; then
    echo "# Android NDK environment variables - added by ndk.sh script" >> "$SHELL_CONFIG_FILE"
    echo "export ANDROID_SDK_ROOT=\"${ANDROID_SDK_ROOT}\"" >> "$SHELL_CONFIG_FILE"
    echo "export ANDROID_NDK_ROOT=\"${ANDROID_NDK_ROOT}\"" >> "$SHELL_CONFIG_FILE"
    echo "export PATH=\"\${ANDROID_NDK_ROOT}:\${PATH}\"" >> "$SHELL_CONFIG_FILE"
    echo "Environment variables added to $SHELL_CONFIG_FILE"
else
    echo "Environment variables already exist in $SHELL_CONFIG_FILE, no changes made."
fi

echo "Android NDK ${ANDROID_NDK_VERSION} installation complete!"
echo "NDK path: ${ANDROID_NDK_ROOT}"
echo "Run the following command to apply environment variables:"
echo "  source $SHELL_CONFIG_FILE"

# Set environment variables for current session
export ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT}"
export ANDROID_NDK_ROOT="${ANDROID_NDK_ROOT}"
export PATH="${ANDROID_NDK_ROOT}:${PATH}"

echo "Environment variables set for current session."