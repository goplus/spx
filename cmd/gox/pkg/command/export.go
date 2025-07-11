package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/gox/pkg/pack"
	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

func (pself *CmdTool) prepareExport() error {
	// copy assets
	projectDir, _ := filepath.Abs(pself.ProjectDir)
	util.CopyDir2(path.Join(projectDir, "../assets"), path.Join(pself.ProjectDir, "assets"))
	return nil
}

func (pself *CmdTool) ExportBuild(platform string) error {
	println("start export: platform =", platform, " ProjectDir =", pself.ProjectDir)
	os.MkdirAll(filepath.Join(pself.ProjectDir, ".builds", strings.ToLower(platform)), os.ModePerm)
	cmd := exec.Command(pself.CmdPath, "--headless", "--quit", "--path", pself.ProjectDir, "--export-debug", platform)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error exporting to web:", err)
	}
	return err
}

func (pself *CmdTool) ExportTemplateWeb() error {
	targetDir := path.Join(pself.ProjectDir, ".builds/webi")
	targetPath := path.Join(targetDir, "engine.html")
	platformName := "Web"
	os.Mkdir(targetDir, 0755)
	// delete gdextension configs
	os.Remove(path.Join(pself.ProjectDir, "gdspx.gdextension"))
	os.Remove(path.Join(pself.ProjectDir, ".godot/extension_list.cfg"))
	return util.RunCommandInDir(pself.ProjectDir, pself.CmdPath, "--headless", "--quit", "--path", pself.ProjectDir, "--export-debug", platformName, targetPath)
}

const (
	webWorkerMode      = "worker"
	webMinigameMode    = "minigame"
	webMiniprogramMode = "miniprogram"
	webNormalMode      = "normal"
)

func (pself *CmdTool) ExportWeb() error {
	pself.exportWebCommon(webNormalMode)
	// copy minigame files
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webNormalMode, pself.WebDir, true)
	return nil
}

func (pself *CmdTool) ExportMinigame() error {
	pself.exportWebCommon(webMinigameMode)
	// move to subdir
	os.Rename(pself.WebDir, pself.WebDir+"_bck")
	os.MkdirAll(path.Join(pself.WebDir), os.ModePerm)
	os.Rename(pself.WebDir+"_bck", path.Join(pself.WebDir, "rawWeb"))

	// copy minigame files
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webMinigameMode, pself.WebDir, true)

	workDir := pself.WebDir

	// safely get build mode with default fallback
	buildMode := *pself.Args.Build

	// create target directories
	engineDir := path.Join(workDir, "engine")
	jsDir := path.Join(workDir, "js")
	rawWebDir := path.Join(workDir, "rawWeb")

	os.MkdirAll(engineDir, os.ModePerm)
	os.MkdirAll(jsDir, os.ModePerm)

	// handle WASM files based on build mode
	godotEditorWasm := path.Join(rawWebDir, "engine.wasm")
	gdspxWasm := path.Join(rawWebDir, "gdspx.wasm")

	if buildMode == "fast" {
		// fast build: move WASM files directly without compression
		if err := pself.moveFile(godotEditorWasm, path.Join(engineDir, "engine.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", godotEditorWasm, err)
		}

		if err := pself.moveFile(gdspxWasm, path.Join(engineDir, "gdspx.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", gdspxWasm, err)
		}
	} else {
		// normal build: compress WASM files
		// check if brotli is installed
		if _, err := exec.LookPath("brotli"); err != nil {
			return fmt.Errorf("error: brotli is not installed")
		}

		fmt.Printf("compress %s...\n", godotEditorWasm)
		if err := pself.compressBrotli(godotEditorWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", godotEditorWasm, err)
		}

		fmt.Printf("compress %s...\n", gdspxWasm)
		if err := pself.compressBrotli(gdspxWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", gdspxWasm, err)
		}

		// move compressed files to engine directory
		if err := pself.moveFilesByPattern(rawWebDir, engineDir, "*.br"); err != nil {
			return fmt.Errorf("failed to move br files: %w", err)
		}
	}

	// move files to engine directory
	if err := pself.moveFilesByPattern(rawWebDir, engineDir, "*.zip"); err != nil {
		return fmt.Errorf("failed to move zip files: %w", err)
	}

	// move js files to js directory
	if err := pself.moveFilesByPattern(rawWebDir, jsDir, "*.js"); err != nil {
		return fmt.Errorf("failed to move js files: %w", err)
	}

	// merge JS files
	if err := pself.mergeJSFiles(jsDir, buildMode != "fast"); err != nil {
		return fmt.Errorf("failed to merge JS files: %w", err)
	}

	// remove minigame directory
	os.RemoveAll(rawWebDir)

	// optionally open WeChat Developer Tools
	if wechatDevTools := os.Getenv("WECHAT_DEV_TOOLS"); wechatDevTools != "" {
		println("open wechat dev tools", workDir)
		cmd := exec.Command(path.Join(wechatDevTools, "cli"), "open", "--project", workDir, "-y")
		cmd.Run() // ignore errors as this is optional
	} else {
		fmt.Printf("WECHAT_DEV_TOOLS is not set, please open project manually %s\n", workDir)
	}

	return nil
}

func (pself *CmdTool) ExportMiniprogram() error {
	pself.exportWebCommon(webMiniprogramMode)
	// copy miniprogram files
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webMiniprogramMode, pself.WebDir, true)
	return nil
}

func (pself *CmdTool) ExportWebWorker() error {
	pself.exportWebCommon(webWorkerMode)
	extDir := path.Join(pself.WebDir, "__"+webWorkerMode)
	// copy miniprogram files
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webWorkerMode, extDir, true)

	var filesToMerge []string
	// merge ext/*.js to engine.worker.js
	os.Rename(path.Join(pself.WebDir, "go.wasm.exec.js"), path.Join(extDir, "go.wasm.exec.js"))
	if entries, err := os.ReadDir(extDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.HasSuffix(entry.Name(), ".js") {
				filesToMerge = append(filesToMerge, path.Join(extDir, entry.Name()))
			}
		}
	}

	insertCode := ""
	for _, jsFile := range filesToMerge {
		if util.IsFileExist(jsFile) {
			content, err := os.ReadFile(jsFile)
			if err != nil {
				return err
			}
			insertCode += "\n\n\n" + string(content)
		}
	}

	// insert worker code
	engineBytes, _ := os.ReadFile(path.Join(pself.WebDir, "engine.js"))
	engineStr := string(engineBytes)

	// 1. insert handleGameAppMessage, dirty code to fix minigame
	keyStr := "{if(initializedJS){checkMailbox()}}"
	if !strings.Contains(engineStr, keyStr) {
		println("engine.js not contains keyStr: ", keyStr)
		os.Exit(1)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameAppMessageId) {handleGameAppMessage(e.data);}")
	// 2. insert worker code , dirty code to fix minigame
	keyStr = ";throw ex}}self.onmessage=handleMessage}"
	if !strings.Contains(engineStr, keyStr) {
		println("engine.js not contains keyStr: ", keyStr)
		os.Exit(1)
	}

	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+insertCode)
	os.WriteFile(path.Join(pself.WebDir, "engine.js"), []byte(engineStr), 0644)

	os.RemoveAll(extDir)
	return nil
}

func (pself *CmdTool) exportWebCommon(mode string) error {
	pself.Clear()
	templateDir := path.Join(pself.GoBinPath, "gdspxrt"+pself.Version+"_web"+mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web dir file not found: " + templateDir)
	}

	dstPath := path.Join(pself.ProjectDir, ".builds/web")
	os.MkdirAll(dstPath, os.ModePerm)
	util.CopyDir2(templateDir, dstPath)

	println("==> _exportWeb", dstPath)
	// copy project files
	util.CopyDir(pself.ProjectFS, "template/project", pself.ProjectDir, true)
	dir := pself.TargetDir
	util.SetupFile(false, path.Join(dir, ".gitignore"), pself.GitignoreTxt)
	os.Rename(path.Join(dir, ".gitignore.txt"), path.Join(dir, ".gitignore"))

	os.Rename(path.Join(dstPath, "godot.editor.html"), path.Join(dstPath, "index.html"))

	// overwrite web files
	util.CopyDir(pself.PlatformFS, "template/platform/web", pself.WebDir, true)
	// Append ext/*.js to engine.worker.js then remove them

	pack.PackProject(pself.TargetDir, path.Join(pself.WebDir, "game.zip"))
	//pack.PackEngineRes(pself.ProjectFS, pself.WebDir)
	util.CopyFile(pself.getWasmPath(), path.Join(pself.WebDir, "gdspx.wasm"))
	util.CopyFile(pself.getWasmPath()+".br", path.Join(pself.WebDir, "gdspx.wasm.br"))
	pack.SaveEngineHash(pself.WebDir)
	return nil
}

func (pself *CmdTool) Export() error {
	targetDir := path.Join(pself.ProjectDir, ".builds/pc")
	targetPath := path.Join(targetDir, PcExportName)
	platformName := ""
	if runtime.GOOS == "windows" {
		targetPath += ".exe"
		platformName = "Win"
	} else if runtime.GOOS == "darwin" {
		platformName = "Mac"
		targetPath += ".app"
	} else if runtime.GOOS == "linux" {
		platformName = "Linux"
	}

	os.Mkdir(targetDir, 0755)
	return util.RunCommandInDir(pself.ProjectDir, pself.CmdPath, "--headless", "--quit", "--path", pself.ProjectDir, "--export-debug", platformName, targetPath)
}

func (pself *CmdTool) ExportIos() error {
	pself.prepareExport()

	pself.BuildDll()
	// include ios files to build
	files, _ := filepath.Glob(filepath.Join(pself.ProjectDir, "go", "ios*"))
	for _, file := range files {
		if strings.HasSuffix(file, ".txt") {
			newName := strings.TrimSuffix(file, ".txt")
			os.Rename(file, newName)
		}
	}

	// First build the iOS libraries
	if err := pself.buildIosLibraries(); err != nil {
		return fmt.Errorf("failed to build iOS libraries: %w", err)
	}

	// Set up paths
	ipaPath := filepath.Join(pself.ProjectDir, ".builds", "ios", "Game.ipa")
	buildDir := filepath.Dir(ipaPath)

	// Create builds directory if it doesn't exist
	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Check if Godot binary exists
	if _, err := os.Stat(pself.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", pself.CmdPath)
	}

	// Check if project file exists
	projectFilePath := filepath.Join(pself.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}

	// Import project to ensure resources are up to date
	fmt.Println("Importing project resources...")
	cmd := exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--editor", "--quit")
	if err := cmd.Run(); err != nil {
		fmt.Println("Warning: project import may have issues:", err)
	}

	// Export the project to IPA
	fmt.Println("Exporting Godot project to IPA...")
	cmd = exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--export-debug", "iOS", ipaPath)

	// Capture standard output and error
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("IPA export failed: %w", err)
	}

	// Check if IPA was created
	if _, err := os.Stat(ipaPath); os.IsNotExist(err) {
		return fmt.Errorf("IPA export failed: file not created at %s", ipaPath)
	}

	log.Println("IPA export completed successfully!", ipaPath)
	if *pself.Args.Install {
		log.Println("Try to install ipa to devices...")
		// install ipa to device
		cmd = exec.Command("ios-deploy", "--bundle", ipaPath)

		// Capture standard output and error
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("IPA install failed: %w", err)
		}
	}
	return nil
}

func (pself *CmdTool) buildIosLibraries() error {
	// Configuration variables
	frameworkName := "gdspx"
	libDir := filepath.Join(pself.ProjectDir, "lib")
	xcframeworkPath := filepath.Join(libDir, "lib"+frameworkName+".ios.xcframework")
	buildDir := filepath.Join(pself.ProjectDir, ".godot", "tmp", "gobuild")
	simulatorDir := filepath.Join(buildDir, "simulator")
	deviceDir := filepath.Join(buildDir, "device")
	headersDir := filepath.Join(buildDir, "headers")
	goSrcDir := filepath.Join(pself.ProjectDir, "go")

	// Create directories
	os.RemoveAll(buildDir)
	os.RemoveAll(xcframeworkPath)
	for _, dir := range []string{simulatorDir, deviceDir, libDir, headersDir} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	fmt.Println("📦 Building Go libraries for iOS...")

	// Create a dummy header file with the required exports
	headerContent := `#ifndef LIBGDSPX_H
#define LIBGDSPX_H

#include <stdlib.h>

// GDExtension initialization function
void GDExtensionInit(void *p_interface, const void *p_library, void *r_initialization);

#endif // LIBGDSPX_H
`
	if err := os.WriteFile(filepath.Join(headersDir, "libgdspx.h"), []byte(headerContent), 0644); err != nil {
		return fmt.Errorf("failed to create header file: %w", err)
	}

	// Copy C headers to the headers directory
	headerFiles, err := filepath.Glob(filepath.Join(goSrcDir, "*.h"))
	if err != nil {
		return fmt.Errorf("failed to find header files: %w", err)
	}
	for _, headerFile := range headerFiles {
		destFile := filepath.Join(headersDir, filepath.Base(headerFile))
		if err := util.CopyFile(headerFile, destFile); err != nil {
			return fmt.Errorf("failed to copy header file %s: %w", headerFile, err)
		}
	}

	// Get SDK paths
	simulatorSdkPath, err := exec.Command("xcrun", "--sdk", "iphonesimulator", "--show-sdk-path").Output()
	if err != nil {
		return fmt.Errorf("failed to get simulator SDK path: %w", err)
	}
	deviceSdkPath, err := exec.Command("xcrun", "--sdk", "iphoneos", "--show-sdk-path").Output()
	if err != nil {
		return fmt.Errorf("failed to get device SDK path: %w", err)
	}

	// Disable signal handling in Go for iOS
	os.Setenv("GODEBUG", "cgocheck=0,asyncpreemptoff=1,panicnil=1")

	// Build for iOS Simulator (x86_64)
	fmt.Println("🔨 Building for iOS Simulator (x86_64)...")
	cmd := exec.Command("go", "build", "-tags=ios,packmode", "-buildmode=c-archive", "-trimpath", "-ldflags=-w -s", "-o", filepath.Join(simulatorDir, "libgdspx-x86_64.a"), ".")
	cmd.Dir = goSrcDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=darwin",
		"GOARCH=amd64",
		"CGO_CFLAGS=-isysroot "+strings.TrimSpace(string(simulatorSdkPath))+" -mios-simulator-version-min=12.0 -arch x86_64 -fembed-bitcode",
		"CGO_LDFLAGS=-isysroot "+strings.TrimSpace(string(simulatorSdkPath))+" -mios-simulator-version-min=12.0 -arch x86_64",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build for iOS Simulator (x86_64): %w", err)
	}

	// Build for iOS Simulator (arm64)
	fmt.Println("🔨 Building for iOS Simulator (arm64)...")
	cmd = exec.Command("go", "build", "-tags=ios,packmode", "-buildmode=c-archive", "-trimpath", "-ldflags=-w -s", "-o", filepath.Join(simulatorDir, "libgdspx-arm64-sim.a"), ".")
	cmd.Dir = goSrcDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=darwin",
		"GOARCH=arm64",
		"CGO_CFLAGS=-isysroot "+strings.TrimSpace(string(simulatorSdkPath))+" -mios-simulator-version-min=12.0 -arch arm64 -fembed-bitcode",
		"CGO_LDFLAGS=-isysroot "+strings.TrimSpace(string(simulatorSdkPath))+" -mios-simulator-version-min=12.0 -arch arm64",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build for iOS Simulator (arm64): %w", err)
	}

	// Build for iOS Device (arm64)
	fmt.Println("🔨 Building for iOS Device (arm64)...")
	cmd = exec.Command("go", "build", "-tags=ios,packmode", "-buildmode=c-archive", "-trimpath", "-ldflags=-w -s", "-o", filepath.Join(deviceDir, "libgdspx-arm64.a"), ".")
	cmd.Dir = goSrcDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=darwin",
		"GOARCH=arm64",
		"CGO_CFLAGS=-isysroot "+strings.TrimSpace(string(deviceSdkPath))+" -mios-version-min=12.0 -arch arm64 -fembed-bitcode",
		"CGO_LDFLAGS=-isysroot "+strings.TrimSpace(string(deviceSdkPath))+" -mios-version-min=12.0 -arch arm64",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build for iOS Device (arm64): %w", err)
	}

	// Create a fat binary for simulator (combines arm64 and x86_64)
	fmt.Println("🔗 Creating fat binary for simulator...")
	cmd = exec.Command("lipo", "-create", "-output", filepath.Join(simulatorDir, "libgdspx.a"),
		filepath.Join(simulatorDir, "libgdspx-x86_64.a"),
		filepath.Join(simulatorDir, "libgdspx-arm64-sim.a"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create fat binary for simulator: %w", err)
	}

	// Create XCFramework
	fmt.Println("🎁 Creating XCFramework...")
	cmd = exec.Command("xcrun", "xcodebuild", "-create-xcframework",
		"-library", filepath.Join(simulatorDir, "libgdspx.a"), "-headers", headersDir,
		"-library", filepath.Join(deviceDir, "libgdspx-arm64.a"), "-headers", headersDir,
		"-output", xcframeworkPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create XCFramework: %w", err)
	}

	// Clean up temporary build files
	fmt.Println("🧹 Cleaning up temporary build files...")
	os.RemoveAll(buildDir)

	fmt.Println("✅ Successfully built libgdspx.ios.xcframework!")
	fmt.Println("📍 Location:", xcframeworkPath)

	return nil
}

func (pself *CmdTool) ExportApk() error {
	pself.prepareExport()
	pself.BuildDll()
	// First build the dynamic libraries for Android
	if err := pself.buildAndroidLibraries(); err != nil {
		return fmt.Errorf("failed to build Android libraries: %w", err)
	}

	// Set up paths
	apkPath := filepath.Join(pself.ProjectDir, ".builds", "android", "game.apk")
	buildDir := filepath.Dir(apkPath)

	// Create builds directory if it doesn't exist
	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Check if Godot binary exists
	if _, err := os.Stat(pself.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", pself.CmdPath)
	}

	// Check if project file exists
	projectFilePath := filepath.Join(pself.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}

	// Import project to ensure resources are up to date
	fmt.Println("Importing project resources...")
	cmd := exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--editor", "--quit")
	if err := cmd.Run(); err != nil {
	}

	// Export the project to APK
	fmt.Println("Exporting Godot project to APK...")
	cmd = exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--export-debug", "Android", apkPath)

	// Capture standard output and error
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("APK export failed: %w", err)
	}

	// Check if APK was created
	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		fmt.Println("APK export failed: file not created at ", apkPath)
		return nil
	}
	log.Println("APK export completed successfully!", apkPath)

	// Check if adb is available
	_, err := exec.LookPath("adb")
	if err != nil {
		fmt.Println("adb command not found. Please ensure Android SDK platform tools are installed and in your PATH")
		return nil
	}

	// Check if any Android device is connected
	cmd = exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("failed to check for connected devices:", err)
		return nil
	}

	if !strings.Contains(string(output), "device\n") {
		fmt.Println("no Android device connected. Please connect a device and enable USB debugging")
		return nil
	}

	if *pself.Args.Install {
		// Install the APK
		fmt.Println("Installing APK...")
		cmd = exec.Command("adb", "install", "-r", apkPath)
		if err := cmd.Run(); err != nil {
			fmt.Println("APK installation failed:", err)
			return nil
		}
		fmt.Println("APK installation successful!")
	}
	return nil
}

func (pself *CmdTool) buildAndroidLibraries() error {
	libDir := filepath.Join(pself.ProjectDir, "lib")
	goDir := filepath.Join(pself.ProjectDir, "go")

	// Check if ANDROID_NDK_ROOT is set
	androidNdkRoot := os.Getenv("ANDROID_NDK_ROOT")
	if androidNdkRoot == "" {
		fmt.Println("ANDROID_NDK_ROOT environment variable is not set")
		return nil
	}

	// Detect system architecture and OS
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Set host tag based on OS and architecture
	hostTag := ""
	switch osName {
	case "windows":
		hostTag = "windows-x86_64"
	case "linux":
		if arch == "amd64" {
			hostTag = "linux-x86_64"
		} else if arch == "arm64" {
			hostTag = "linux-aarch64"
		} else {
			return fmt.Errorf("unsupported Linux architecture: %s", arch)
		}
	case "darwin":
		hostTag = "darwin-x86_64"
	default:
		return fmt.Errorf("unsupported operating system: %s", osName)
	}

	// Create lib directory if it doesn't exist
	if err := os.MkdirAll(libDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	// Set NDK toolchain path and minimum SDK version
	ndkToolchain := filepath.Join(androidNdkRoot, "toolchains", "llvm", "prebuilt", hostTag, "bin")
	minSdk := "21"

	// Build for arm64-v8a
	fmt.Println("Building for arm64-v8a...", goDir)
	cmd := exec.Command("go", "build", "-tags=android,packmode", "-buildmode=c-shared", "-o", filepath.Join(libDir, "libgdspx-android-arm64.so"), ".")
	cmd.Dir = goDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=android",
		"GOARCH=arm64",
		"CC="+filepath.Join(ndkToolchain, "aarch64-linux-android"+minSdk+"-clang"),
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build for arm64-v8a: %w", err)
	}

	// Build for armeabi-v7a
	fmt.Println("Building for armeabi-v7a...")
	cmd = exec.Command("go", "build", "-tags=android,packmode", "-buildmode=c-shared", "-o", filepath.Join(libDir, "libgdspx-android-arm32.so"), ".")
	cmd.Dir = goDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=android",
		"GOARCH=arm",
		"CC="+filepath.Join(ndkToolchain, "armv7a-linux-androideabi"+minSdk+"-clang"),
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build for armeabi-v7a: %w", err)
	}

	fmt.Println("Build android so completed successfully!")
	return nil
}

// compressBrotli compresses a file using brotli
func (pself *CmdTool) compressBrotli(filePath string) error {
	cmd := exec.Command("brotli", "-f", "-q", "11", filePath)
	return cmd.Run()
}

// moveFile moves a single file from source to destination
func (pself *CmdTool) moveFile(srcFile, dstFile string) error {
	return os.Rename(srcFile, dstFile)
}

// moveFilesByPattern moves files matching a pattern
func (pself *CmdTool) moveFilesByPattern(srcDir, dstDir, pattern string) error {
	files, err := filepath.Glob(path.Join(srcDir, pattern))
	if err != nil {
		return err
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		dstFile := path.Join(dstDir, fileName)
		if err := os.Rename(file, dstFile); err != nil {
			return err
		}
	}

	return nil
}

// mergeJSFiles merges JavaScript files
func (pself *CmdTool) mergeJSFiles(jsDir string, isCompressed bool) error {
	// file merge order
	jsFiles := []string{"header.js", "engine.js", "go.wasm.exec.js", "storage.manager.js", "worker.message.manager.js", "game.js"}
	outputFile := path.Join(jsDir, "engine_new.js")

	// create output file
	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := bufio.NewWriter(output)
	defer writer.Flush()

	// write compression flag at the beginning
	compressionFlag := fmt.Sprintf("var FFI = null;\nconst isWasmCompressed = %t;\n\n", isCompressed)
	if _, err := writer.WriteString(compressionFlag); err != nil {
		return err
	}

	// merge file contents
	for _, jsFile := range jsFiles {
		filePath := path.Join(jsDir, jsFile)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue // skip non-existent files
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		file.Close()
		if err != nil {
			return err
		}

		// remove original file
		os.Remove(filePath)
	}

	// rename output file
	return os.Rename(outputFile, path.Join(jsDir, "engine.js"))
}
