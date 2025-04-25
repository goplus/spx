#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

# Target font file path
target_font_path="./template/project/engine/fonts/CnFont.ttf"

# Define font files to look for
font_names=("Songti.ttc" "Times New Roman.ttf" "Times.ttf" "SimSun.ttf" "SimSun.ttc" )

# Detect OS type and set corresponding font search paths
case "$(uname -s)" in
    Darwin*)
        # macOS
        font_find_paths=(
            "/Library/Fonts"
            "/System/Library/Fonts"
            "/System/Library/Fonts/Supplemental"
        )
        ;;
    Linux*)
        # Linux
        font_find_paths=(
            "/system/fonts/"
            "/usr/share/fonts/"
            "/usr/local/share/fonts/"
            "$HOME/.fonts/"
        )
        ;;
    CYGWIN*|MINGW*|MSYS*|Windows*)
        # Windows
        font_find_paths=(
            "C:\\windows\\fonts"
        )
        ;;
    *)
        echo "Unknown operating system, cannot determine font paths"
        exit 1
        ;;
esac

# Find font file in specified path
find_font_at_path() {
    local find_path="$1"
    local font_names=("${@:2}")
    
    for font_name in "${font_names[@]}"; do
        try_file="$find_path/$font_name"
        if [ -f "$try_file" ]; then
            echo "$try_file"
            return 0
        fi
    done
    
    return 1
}

# Main function
main() {
    has_found=false
    font_path=""
    
    # Search for fonts in all paths and names
    for find_path in "${font_find_paths[@]}"; do
        for font_name in "${font_names[@]}"; do
            try_file="$find_path/$font_name"
            if [ -f "$try_file" ]; then
                cp "$try_file" "$target_font_path"
                if [ $? -ne 0 ]; then
                    echo "Failed to copy font file!" >&2
                    exit 1
                fi
                chmod 644 "$target_font_path"
                echo "Copied $try_file to $target_font_path"
                has_found=true
                break 2
            fi
        done
    done
    
    # If no font is found, display message and exit
    if [ "$has_found" = false ]; then
        # Special message for macOS
        if [[ "$(uname -s)" == Darwin* ]]; then
            echo "Warning: No required font files found (${font_names[*]})." >&2
            echo "Chinese characters may not be supported on this macOS system." >&2
            echo "Searched paths: ${font_find_paths[*]}" >&2
        else
            echo "Warning: No required font files found (${font_names[*]})." >&2
            echo "Searched paths: ${font_find_paths[*]}" >&2
        fi
    fi
}

# Execute main function
main
