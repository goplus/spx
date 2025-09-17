@echo off
setlocal enabledelayedexpansion

REM SPX Game Engine One-Click Installation Script (Windows)
REM Usage: init.bat

echo 🎮 SPX Game Engine One-Click Installation Script (Windows)
echo =========================================================

REM Function: Check if command exists
:CHECK_COMMAND
where %1 >nul 2>&1
if %errorlevel%==0 (
    echo ✅ %1 is installed
    exit /b 0
) else (
    echo ❌ %1 is not installed
    exit /b 1
)

REM Step 1: Check MSYS2
echo.
echo 🔍 Step 1/6: Checking MSYS2 environment...
if exist "C:\msys64\usr\bin\bash.exe" (
    echo ✅ MSYS2 is installed
    set MSYS2_PATH=C:\msys64
) else if exist "C:\msys32\usr\bin\bash.exe" (
    echo ✅ MSYS2 is installed
    set MSYS2_PATH=C:\msys32
) else (
    echo ❌ MSYS2 is not installed
    echo 📦 Please install MSYS2 first:
    echo    1. Visit https://www.msys2.org/
    echo    2. Download and install MSYS2
    echo    3. Run this script again after installation
    pause
    exit /b 1
)

REM Step 2: Setup MSYS2 environment
echo.
echo 🔧 Step 2/6: Configuring MSYS2 environment...
set PATH=%MSYS2_PATH%\mingw64\bin;%MSYS2_PATH%\usr\bin;%PATH%

REM Step 3: Update MSYS2
echo.
echo 🔄 Step 3/6: Updating MSYS2 system...
echo Updating MSYS2, this may take a few minutes...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -Syu --noconfirm"

REM Step 4: Install development tools
echo.
echo 📦 Step 4/6: Installing development tools...
echo Installing toolchain and dependencies...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --needed --noconfirm base-devel mingw-w64-x86_64-toolchain"
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --noconfirm make zip unzip bash"

REM Step 5: Install Go
echo.
echo 🔍 Step 5/6: Installing Go v1.23.0...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --noconfirm mingw-w64-x86_64-go"

REM Verify Go installation
%MSYS2_PATH%\usr\bin\bash.exe -l -c "go version"
if %errorlevel% neq 0 (
    echo ❌ Go installation failed
    pause
    exit /b 1
)

REM Step 6: Install xgo and initialize SPX
echo.
echo 🔧 Step 6/6: Installing xgo and initializing SPX environment...

REM Create installation script
echo echo "📦 Cloning xgo repository..." > %TEMP%\install_spx.sh
echo if [ ! -d "xgo" ]; then >> %TEMP%\install_spx.sh
echo     git clone https://github.com/goplus/xgo.git >> %TEMP%\install_spx.sh
echo fi >> %TEMP%\install_spx.sh
echo. >> %TEMP%\install_spx.sh
echo echo "🔧 Building and installing xgo..." >> %TEMP%\install_spx.sh
echo cd xgo >> %TEMP%\install_spx.sh
echo ./all.bash >> %TEMP%\install_spx.sh
echo cd .. >> %TEMP%\install_spx.sh
echo. >> %TEMP%\install_spx.sh
echo echo "🔧 Initializing SPX environment..." >> %TEMP%\install_spx.sh
echo make setup >> %TEMP%\install_spx.sh
echo. >> %TEMP%\install_spx.sh
echo echo "🧪 Verifying installation..." >> %TEMP%\install_spx.sh
echo if command -v spx ^>^/dev^/null 2^>^&1; then >> %TEMP%\install_spx.sh
echo     spx version >> %TEMP%\install_spx.sh
echo     echo "" >> %TEMP%\install_spx.sh
echo     echo "🎉 SPX Game Engine installation successful!" >> %TEMP%\install_spx.sh
echo     echo "" >> %TEMP%\install_spx.sh
echo     echo "📚 Next steps:" >> %TEMP%\install_spx.sh
echo     echo "   1. Run example game: cd tutorial/00-Hello && spx run" >> %TEMP%\install_spx.sh
echo     echo "   2. View all examples: make list-demos" >> %TEMP%\install_spx.sh
echo     echo "   3. Read documentation: docs/zh/README.md" >> %TEMP%\install_spx.sh
echo     echo "" >> %TEMP%\install_spx.sh
echo     echo "⚠️  Important: Always run SPX commands in MSYS2 MinGW64 terminal" >> %TEMP%\install_spx.sh
echo else >> %TEMP%\install_spx.sh
echo     echo "❌ SPX installation may not be complete, please check error messages" >> %TEMP%\install_spx.sh
echo     exit 1 >> %TEMP%\install_spx.sh
echo fi >> %TEMP%\install_spx.sh

REM Execute installation script
%MSYS2_PATH%\usr\bin\bash.exe -l %TEMP%\install_spx.sh

REM Clean up temporary files
del %TEMP%\install_spx.sh

echo.
echo =========================================================
echo 🎉 Installation complete!
echo.
echo ⚠️  Important notes:
echo    Always run SPX commands in MSYS2 MinGW64 terminal
echo    Launch method: Start Menu → MSYS2 MinGW64
echo.
echo 📚 Quick start:
echo    1. Open MSYS2 MinGW64 terminal
echo    2. cd /path/to/spx
echo    3. cd tutorial/00-Hello && spx run
echo =========================================================

pause