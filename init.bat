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
echo 🔍 Step 1/8: Checking MSYS2 environment...
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
echo 🔧 Step 2/8: Configuring MSYS2 environment...
set PATH=%MSYS2_PATH%\mingw64\bin;%MSYS2_PATH%\usr\bin;%PATH%

REM Step 3: Update MSYS2
echo.
echo 🔄 Step 3/8: Updating MSYS2 system...
echo Updating MSYS2, this may take a few minutes...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -Syu --noconfirm"

REM Step 4: Install development tools
echo.
echo 📦 Step 4/8: Installing development tools...
echo Installing toolchain and dependencies...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --needed --noconfirm base-devel mingw-w64-x86_64-toolchain"
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --noconfirm make zip unzip bash"

REM Step 5: Install Go v1.23.0
echo.
echo 🔍 Step 5/8: Installing Go v1.23.0...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "pacman -S --noconfirm mingw-w64-x86_64-go"

REM Verify Go installation
%MSYS2_PATH%\usr\bin\bash.exe -l -c "go version"
if %errorlevel% neq 0 (
    echo ❌ Go installation failed
    pause
    exit /b 1
)

REM Step 6: Install xgo v1.5.0 (matching GitHub action setup-xgo approach)
echo.
echo 🔧 Step 6/8: Installing xgo v1.5.0...

REM Create xgo installation script
echo echo "📦 Installing xgo v1.5.0..." > %TEMP%\install_xgo.sh
echo GO_VERSION=$(go version ^| awk '{print $3}' ^| sed 's/go//') >> %TEMP%\install_xgo.sh
echo echo "Detected Go version: $GO_VERSION" >> %TEMP%\install_xgo.sh
echo. >> %TEMP%\install_xgo.sh
echo echo "🔧 Cloning and building xgo v1.5.0..." >> %TEMP%\install_xgo.sh
echo if [ ! -d "xgo" ]; then >> %TEMP%\install_xgo.sh
echo     git clone --depth 1 --branch v1.5.0 https://github.com/goplus/xgo.git >> %TEMP%\install_xgo.sh
echo fi >> %TEMP%\install_xgo.sh
echo cd xgo >> %TEMP%\install_xgo.sh
echo ./all.bash >> %TEMP%\install_xgo.sh
echo cd .. >> %TEMP%\install_xgo.sh
echo echo "✅ xgo v1.5.0 installed successfully" >> %TEMP%\install_xgo.sh

REM Execute xgo installation
%MSYS2_PATH%\usr\bin\bash.exe -l %TEMP%\install_xgo.sh
if %errorlevel% neq 0 (
    echo ❌ xgo installation failed
    del %TEMP%\install_xgo.sh
    pause
    exit /b 1
)

REM Clean up xgo installation script
del %TEMP%\install_xgo.sh

REM Step 7: Setup SPX environment (matching GitHub action deps step)
echo.
echo 🔧 Step 7/8: Setting up SPX environment...
%MSYS2_PATH%\usr\bin\bash.exe -l -c "make setup"
if %errorlevel% neq 0 (
    echo ❌ SPX setup failed
    pause
    exit /b 1
)

REM Step 8: Run tests (matching GitHub action project-test steps)
echo.
echo 🧪 Step 8/8: Running tests and verification...

REM Create test script
echo echo "🔨 Building project..." > %TEMP%\test_spx.sh
echo go build -v $(go list ./... ^| grep -v /internal/webffi) >> %TEMP%\test_spx.sh
echo if [ $? -ne 0 ]; then >> %TEMP%\test_spx.sh
echo     echo "❌ Build failed" >> %TEMP%\test_spx.sh
echo     exit 1 >> %TEMP%\test_spx.sh
echo fi >> %TEMP%\test_spx.sh
echo. >> %TEMP%\test_spx.sh
echo echo "🧪 Running tests..." >> %TEMP%\test_spx.sh
echo go test -v $(go list ./... ^| grep -v /internal/webffi) >> %TEMP%\test_spx.sh
echo if [ $? -ne 0 ]; then >> %TEMP%\test_spx.sh
echo     echo "❌ Tests failed" >> %TEMP%\test_spx.sh
echo     exit 1 >> %TEMP%\test_spx.sh
echo fi >> %TEMP%\test_spx.sh
echo. >> %TEMP%\test_spx.sh
echo echo "🔧 Running xgo compilation..." >> %TEMP%\test_spx.sh
echo xgo go ./... >> %TEMP%\test_spx.sh
echo if [ $? -ne 0 ]; then >> %TEMP%\test_spx.sh
echo     echo "❌ xgo compilation failed" >> %TEMP%\test_spx.sh
echo     exit 1 >> %TEMP%\test_spx.sh
echo fi >> %TEMP%\test_spx.sh
echo. >> %TEMP%\test_spx.sh
echo echo "🎮 Running test demo..." >> %TEMP%\test_spx.sh
echo spx run -path="test/CI" -headless=true ^>cilog.txt 2^>^&1 ^& >> %TEMP%\test_spx.sh
echo sleep 10 >> %TEMP%\test_spx.sh
echo. >> %TEMP%\test_spx.sh
echo echo "✅ Checking test results..." >> %TEMP%\test_spx.sh
echo if grep -q "===>SpxCIRunSucc" cilog.txt; then >> %TEMP%\test_spx.sh
echo     echo "✅ Test demo completed successfully" >> %TEMP%\test_spx.sh
echo     rm -f cilog.txt >> %TEMP%\test_spx.sh
echo else >> %TEMP%\test_spx.sh
echo     echo "❌ Test demo failed: success mark not found" >> %TEMP%\test_spx.sh
echo     echo "Log contents:" >> %TEMP%\test_spx.sh
echo     cat cilog.txt >> %TEMP%\test_spx.sh
echo     rm -f cilog.txt >> %TEMP%\test_spx.sh
echo     exit 1 >> %TEMP%\test_spx.sh
echo fi >> %TEMP%\test_spx.sh
echo. >> %TEMP%\test_spx.sh
echo echo "🎉 SPX Game Engine installation and testing completed successfully!" >> %TEMP%\test_spx.sh
echo echo "" >> %TEMP%\test_spx.sh
echo echo "📚 Next steps:" >> %TEMP%\test_spx.sh
echo echo "   1. Run example game: cd tutorial/00-Hello && spx run" >> %TEMP%\test_spx.sh
echo echo "   2. View all examples: make list-demos" >> %TEMP%\test_spx.sh
echo echo "   3. Read documentation: docs/zh/README.md" >> %TEMP%\test_spx.sh
echo echo "" >> %TEMP%\test_spx.sh
echo echo "⚠️  Important: Always run SPX commands in MSYS2 MinGW64 terminal" >> %TEMP%\test_spx.sh

REM Execute test script
%MSYS2_PATH%\usr\bin\bash.exe -l %TEMP%\test_spx.sh

REM Clean up test script
del %TEMP%\test_spx.sh

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