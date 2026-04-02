package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/pack"
	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

func (pself *CmdTool) prepareExport() error {
	projectDir, _ := filepath.Abs(pself.ProjectDir)
	util.CopyDir2(filepath.Join(projectDir, "..", "assets"), filepath.Join(pself.ProjectDir, "assets"))
	return nil
}

func (pself *CmdTool) ExportBuild(platform string) error {
	fmt.Printf("Starting export: platform=%s, ProjectDir=%s\n", platform, pself.ProjectDir)
	os.MkdirAll(filepath.Join(pself.ProjectDir, ".builds", strings.ToLower(platform)), os.ModePerm)
	cmd := exec.Command(pself.CmdPath, "--headless", "--quit", "--path", pself.ProjectDir, "--export-debug", platform)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error exporting to web:", err)
	}
	return err
}

func (pself *CmdTool) ExportTemplateWeb() error {
	targetDir := filepath.Join(pself.ProjectDir, ".builds", "webi")
	targetPath := filepath.Join(targetDir, "engine.html")
	platformName := "Web"
	os.Mkdir(targetDir, 0755)
	os.Remove(filepath.Join(pself.ProjectDir, "gdspx.gdextension"))
	os.Remove(filepath.Join(pself.ProjectDir, ".godot", "extension_list.cfg"))
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
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webNormalMode, pself.WebDir, true)
	return nil
}

func (pself *CmdTool) ExportMinigame() error {
	pself.exportWebCommon(webMinigameMode)
	os.Rename(pself.WebDir, pself.WebDir+"_bck")
	os.MkdirAll(filepath.Join(pself.WebDir), os.ModePerm)
	os.Rename(pself.WebDir+"_bck", filepath.Join(pself.WebDir, "rawWeb"))

	util.CopyDir(pself.PlatformFS, "template/platform/web"+webMinigameMode, pself.WebDir, true)

	workDir := pself.WebDir

	buildMode := *pself.Args.Build

	engineDir := filepath.Join(workDir, "engine")
	jsDir := filepath.Join(workDir, "js")
	rawWebDir := filepath.Join(workDir, "rawWeb")

	os.MkdirAll(engineDir, os.ModePerm)
	os.MkdirAll(jsDir, os.ModePerm)

	godotEditorWasm := filepath.Join(rawWebDir, "engine.wasm")
	ispxWasm := filepath.Join(rawWebDir, "ispx.wasm")

	if buildMode == "fast" {
		if err := pself.moveFile(godotEditorWasm, filepath.Join(engineDir, "engine.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", godotEditorWasm, err)
		}

		if err := pself.moveFile(ispxWasm, filepath.Join(engineDir, "ispx.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", ispxWasm, err)
		}
	} else {
		if _, err := exec.LookPath("brotli"); err != nil {
			return fmt.Errorf("error: brotli is not installed")
		}

		fmt.Printf("compress %s...\n", godotEditorWasm)
		if err := pself.compressBrotli(godotEditorWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", godotEditorWasm, err)
		}

		fmt.Printf("compress %s...\n", ispxWasm)
		if err := pself.compressBrotli(ispxWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", ispxWasm, err)
		}

		if err := pself.moveFilesByPattern(rawWebDir, engineDir, "*.br"); err != nil {
			return fmt.Errorf("failed to move br files: %w", err)
		}
	}

	if err := pself.moveFilesByPattern(rawWebDir, engineDir, "*.zip"); err != nil {
		return fmt.Errorf("failed to move zip files: %w", err)
	}

	if err := pself.moveFilesByPattern(rawWebDir, jsDir, "*.js"); err != nil {
		return fmt.Errorf("failed to move js files: %w", err)
	}

	if err := pself.mergeJSFiles(jsDir, buildMode != "fast"); err != nil {
		return fmt.Errorf("failed to merge JS files: %w", err)
	}

	os.RemoveAll(rawWebDir)

	if wechatDevTools := os.Getenv("WECHAT_DEV_TOOLS"); wechatDevTools != "" {
		println("open wechat dev tools", workDir)
		cmd := exec.Command(filepath.Join(wechatDevTools, "cli"), "open", "--project", workDir, "-y")
		cmd.Run() // ignore errors as this is optional
	} else {
		fmt.Printf("WECHAT_DEV_TOOLS is not set, please open project manually %s\n", workDir)
	}

	return nil
}

func (pself *CmdTool) ExportMiniprogram() error {
	pself.exportWebCommon(webMiniprogramMode)
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webMiniprogramMode, pself.WebDir, true)
	return nil
}

func (pself *CmdTool) ExportWebWorker() error {
	pself.exportWebCommon(webWorkerMode)
	extDir := filepath.Join(pself.WebDir, "__"+webWorkerMode)
	util.CopyDir(pself.PlatformFS, "template/platform/web"+webWorkerMode, extDir, true)

	var filesToMerge []string
	os.Rename(filepath.Join(pself.WebDir, "go.wasm.exec.js"), filepath.Join(extDir, "go.wasm.exec.js"))
	if entries, err := os.ReadDir(extDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.HasSuffix(entry.Name(), ".js") {
				filesToMerge = append(filesToMerge, filepath.Join(extDir, entry.Name()))
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

	engineBytes, _ := os.ReadFile(filepath.Join(pself.WebDir, "engine.js"))
	engineStr := string(engineBytes)

	// Patch the worker message hook.
	keyStr := "{if(initializedJS){checkMailbox()}}"
	if !strings.Contains(engineStr, keyStr) {
		println("engine.js not contains keyStr: ", keyStr)
		os.Exit(1)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameAppMessageId) {handleGameAppMessage(e.data);}")
	// Inject the worker bundle.
	keyStr = ";throw ex}}self.onmessage=handleMessage}"
	if !strings.Contains(engineStr, keyStr) {
		println("engine.js not contains keyStr: ", keyStr)
		os.Exit(1)
	}

	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+insertCode)
	os.WriteFile(filepath.Join(pself.WebDir, "engine.js"), []byte(engineStr), 0644)

	os.RemoveAll(extDir)
	return nil
}

func (pself *CmdTool) exportWebCommon(mode string) error {
	pself.Clear()
	templateDir := filepath.Join(pself.GoBinPath, "gdspxrt"+pself.Version+"_web"+mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web dir file not found: " + templateDir)
	}

	dstPath := filepath.Join(pself.ProjectDir, ".builds", "web")
	os.MkdirAll(dstPath, os.ModePerm)
	util.CopyDir2(templateDir, dstPath)

	println("==> _exportWeb", dstPath)
	util.CopyDir(pself.ProjectFS, "template/project", pself.ProjectDir, true)

	os.Rename(filepath.Join(dstPath, "godot.editor.html"), filepath.Join(dstPath, "index.html"))

	ispxWebDir, err := pself.getIspxWebDir()
	if err != nil {
		return err
	}
	util.CopyDir2(ispxWebDir, pself.WebDir)

	util.CopyDir(pself.PlatformFS, "template/platform/web", pself.WebDir, true)

	output, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("failed to get GOROOT: %w", err)
	}
	goroot := strings.TrimSpace(string(output))
	wasmExecPath := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	if err := util.CopyFile(wasmExecPath, filepath.Join(pself.WebDir, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to copy wasm_exec.js: %w", err)
	}
	if err := pack.PackProject(pself.TargetDir, filepath.Join(pself.WebDir, "game.zip")); err != nil {
		return err
	}
	wasmPath, wasmBrPath := pself.getWasmPaths()
	if err := util.CopyFile(wasmPath, filepath.Join(pself.WebDir, "ispx.wasm")); err != nil {
		return fmt.Errorf("failed to copy ispx wasm from %s: %w", wasmPath, err)
	}
	if wasmBrPath != "" {
		if err := util.CopyFile(wasmBrPath, filepath.Join(pself.WebDir, "ispx.wasm.br")); err != nil {
			return fmt.Errorf("failed to copy compressed ispx wasm from %s: %w", wasmBrPath, err)
		}
	}
	return nil
}

func (pself *CmdTool) Export() error {
	targetDir := filepath.Join(pself.ProjectDir, ".builds", "pc")
	targetPath := filepath.Join(targetDir, PcExportName)
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
	fmt.Println("===> Starting iOS IPA export process...")

	pself.prepareExport()
	pself.BuildDll()

	files, _ := filepath.Glob(filepath.Join(pself.ProjectDir, "go", "ios*"))
	for _, file := range files {
		if strings.HasSuffix(file, ".txt") {
			newName := strings.TrimSuffix(file, ".txt")
			os.Rename(file, newName)
		}
	}

	fmt.Println("===> Building iOS libraries...")
	if err := pself.buildIosLibraries(); err != nil {
		return fmt.Errorf("failed to build iOS libraries: %w", err)
	}
	fmt.Println("===> iOS libraries build completed successfully!")

	ipaPath := filepath.Join(pself.ProjectDir, ".builds", "ios", "Game.ipa")
	buildDir := filepath.Dir(ipaPath)
	fmt.Printf("===> IPA output path: %s\n", ipaPath)

	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	fmt.Printf("===> Build directory created: %s\n", buildDir)

	if _, err := os.Stat(pself.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", pself.CmdPath)
	}
	fmt.Printf("===> Godot binary found at: %s\n", pself.CmdPath)

	projectFilePath := filepath.Join(pself.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}
	fmt.Printf("===> Godot project file found at: %s\n", projectFilePath)

	homeDir, err := os.UserHomeDir()
	if err == nil {
		var templateDir string
		if runtime.GOOS == "darwin" {
			templateDir = filepath.Join(homeDir, "Library", "Application Support", "Godot", "export_templates")
		} else if runtime.GOOS == "linux" {
			templateDir = filepath.Join(homeDir, ".local", "share", "godot", "export_templates")
		} else if runtime.GOOS == "windows" {
			templateDir = filepath.Join(os.Getenv("APPDATA"), "Godot", "export_templates")
		}

		if templateDir != "" {
			fmt.Printf("===> Checking export templates at: %s\n", templateDir)
			if entries, err := os.ReadDir(templateDir); err == nil {
				fmt.Println("===> Available export template versions:")
				for _, entry := range entries {
					if entry.IsDir() {
						versionDir := filepath.Join(templateDir, entry.Name())
						fmt.Printf("     - %s\n", entry.Name())
						if files, err := os.ReadDir(versionDir); err == nil {
							iosFiles := []string{}
							for _, f := range files {
								if strings.Contains(f.Name(), "ios") {
									iosFiles = append(iosFiles, f.Name())
								}
							}
							if len(iosFiles) > 0 {
								fmt.Printf("       iOS templates: %v\n", iosFiles)
							}
						}
					}
				}
			} else {
				fmt.Printf("===> Warning: Could not read template directory: %v\n", err)
			}
		}
	}

	fmt.Println("===> Importing project resources...")
	cmd := exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--editor", "--quit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("===> Warning: Project import had issues: %v\n", err)
	}

	fmt.Println("===> Exporting Godot project to IPA...")
	fmt.Printf("===> Export command: %s --headless --path %s --export-debug iOS %s\n",
		pself.CmdPath, pself.ProjectDir, ipaPath)

	cmd = exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--export-debug", "iOS", ipaPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("IPA export failed: %w", err)
	}

	if _, err := os.Stat(ipaPath); os.IsNotExist(err) {
		return fmt.Errorf("IPA export failed: file not created at %s", ipaPath)
	}

	log.Println("===> IPA export completed successfully!", ipaPath)
	if *pself.Args.Install {
		log.Println("Try to install ipa to devices...")
		cmd = exec.Command("ios-deploy", "--bundle", ipaPath)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("IPA install failed: %w", err)
		}
	}
	return nil
}

func (pself *CmdTool) buildIosLibraries() error {
	frameworkName := "gdspx"
	libDir := filepath.Join(pself.ProjectDir, "lib")
	xcframeworkPath := filepath.Join(libDir, "lib"+frameworkName+".ios.xcframework")
	buildDir := filepath.Join(pself.ProjectDir, ".godot", "tmp", "gobuild")
	simulatorDir := filepath.Join(buildDir, "simulator")
	deviceDir := filepath.Join(buildDir, "device")
	headersDir := filepath.Join(buildDir, "headers")
	goSrcDir := filepath.Join(pself.ProjectDir, "go")

	os.RemoveAll(buildDir)
	os.RemoveAll(xcframeworkPath)
	for _, dir := range []string{simulatorDir, deviceDir, libDir, headersDir} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	fmt.Println("📦 Building Go libraries for iOS...")

	headerContent := `#ifndef LIBGDSPX_H
#define LIBGDSPX_H

#include <stdlib.h>

// GDExtension entry point.
void GDExtensionInit(void *p_interface, const void *p_library, void *r_initialization);

#endif // LIBGDSPX_H
`
	if err := os.WriteFile(filepath.Join(headersDir, "libgdspx.h"), []byte(headerContent), 0644); err != nil {
		return fmt.Errorf("failed to create header file: %w", err)
	}

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

	simulatorSdkPath, err := exec.Command("xcrun", "--sdk", "iphonesimulator", "--show-sdk-path").Output()
	if err != nil {
		return fmt.Errorf("failed to get simulator SDK path: %w", err)
	}
	deviceSdkPath, err := exec.Command("xcrun", "--sdk", "iphoneos", "--show-sdk-path").Output()
	if err != nil {
		return fmt.Errorf("failed to get device SDK path: %w", err)
	}

	os.Setenv("GODEBUG", "cgocheck=0,asyncpreemptoff=1,panicnil=1")

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

	fmt.Println("🔗 Creating fat binary for simulator...")
	cmd = exec.Command("lipo", "-create", "-output", filepath.Join(simulatorDir, "libgdspx.a"),
		filepath.Join(simulatorDir, "libgdspx-x86_64.a"),
		filepath.Join(simulatorDir, "libgdspx-arm64-sim.a"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create fat binary for simulator: %w", err)
	}

	fmt.Println("🎁 Creating XCFramework...")
	cmd = exec.Command("xcrun", "xcodebuild", "-create-xcframework",
		"-library", filepath.Join(simulatorDir, "libgdspx.a"), "-headers", headersDir,
		"-library", filepath.Join(deviceDir, "libgdspx-arm64.a"), "-headers", headersDir,
		"-output", xcframeworkPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create XCFramework: %w", err)
	}

	fmt.Println("🧹 Cleaning up temporary build files...")
	os.RemoveAll(buildDir)

	fmt.Println("✅ Successfully built libgdspx.ios.xcframework!")
	fmt.Println("📍 Location:", xcframeworkPath)

	return nil
}

func (pself *CmdTool) ExportApk() error {
	pself.prepareExport()
	pself.BuildDll()
	if err := pself.buildAndroidLibraries(); err != nil {
		return fmt.Errorf("failed to build Android libraries: %w", err)
	}

	apkPath := filepath.Join(pself.ProjectDir, ".builds", "android", "game.apk")
	buildDir := filepath.Dir(apkPath)

	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	if _, err := os.Stat(pself.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", pself.CmdPath)
	}

	projectFilePath := filepath.Join(pself.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}

	fmt.Println("Importing project resources...")
	cmd := exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--editor", "--quit")
	if err := cmd.Run(); err != nil {
	}

	fmt.Println("Exporting Godot project to APK...")
	cmd = exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--export-debug", "Android", apkPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("APK export failed: %w", err)
	}

	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		fmt.Println("APK export failed: file not created at ", apkPath)
		return nil
	}
	log.Println("APK export completed successfully!", apkPath)

	_, err := exec.LookPath("adb")
	if err != nil {
		fmt.Println("adb command not found. Please ensure Android SDK platform tools are installed and in your PATH")
		return nil
	}

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

	androidNdkRoot := os.Getenv("ANDROID_NDK_ROOT")
	if androidNdkRoot == "" {
		fmt.Println("ANDROID_NDK_ROOT environment variable is not set")
		return nil
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH

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

	if err := os.MkdirAll(libDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	ndkToolchain := filepath.Join(androidNdkRoot, "toolchains", "llvm", "prebuilt", hostTag, "bin")
	minSdk := "21"

	type androidBuildConfig struct {
		name       string
		goArch     string
		outputFile string
		ccPrefix   string
	}

	builds := []androidBuildConfig{
		{
			name:       "arm64-v8a",
			goArch:     "arm64",
			outputFile: "libgdspx-android-arm64.so",
			ccPrefix:   "aarch64-linux-android",
		},
		{
			name:       "armeabi-v7a",
			goArch:     "arm",
			outputFile: "libgdspx-android-arm32.so",
			ccPrefix:   "armv7a-linux-androideabi",
		},
	}

	for _, build := range builds {
		fmt.Printf("Building for %s... %s\n", build.name, goDir)

		cmd := exec.Command("go", "build", "-tags=android,packmode", "-buildmode=c-shared", "-o", filepath.Join(libDir, build.outputFile), ".")
		cmd.Dir = goDir
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=1",
			"GOOS=android",
			"GOARCH="+build.goArch,
			"CC="+filepath.Join(ndkToolchain, build.ccPrefix+minSdk+"-clang"),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build for %s: %w", build.name, err)
		}
	}

	fmt.Println("Build android so completed successfully!")
	return nil
}

// compressBrotli runs brotli on a file.
func (pself *CmdTool) compressBrotli(filePath string) error {
	cmd := exec.Command("brotli", "-f", "-q", "11", filePath)
	return cmd.Run()
}

// moveFile moves one file.
func (pself *CmdTool) moveFile(srcFile, dstFile string) error {
	return os.Rename(srcFile, dstFile)
}

// moveFilesByPattern moves matching files.
func (pself *CmdTool) moveFilesByPattern(srcDir, dstDir, pattern string) error {
	files, err := filepath.Glob(filepath.Join(srcDir, pattern))
	if err != nil {
		return err
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		dstFile := filepath.Join(dstDir, fileName)
		if err := os.Rename(file, dstFile); err != nil {
			return err
		}
	}

	return nil
}

// mergeJSFiles merges JavaScript files.
func (pself *CmdTool) mergeJSFiles(jsDir string, isCompressed bool) error {
	jsFiles := []string{"header.js", "engine.js", "go.wasm.exec.js", "worker.message.manager.js", "game.js"}
	outputFile := filepath.Join(jsDir, "engine_new.js")

	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := bufio.NewWriter(output)
	defer writer.Flush()

	compressionFlag := fmt.Sprintf("var FFI = null;\nconst isWasmCompressed = %t;\n\n", isCompressed)
	if _, err := writer.WriteString(compressionFlag); err != nil {
		return err
	}

	for _, jsFile := range jsFiles {
		filePath := filepath.Join(jsDir, jsFile)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
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

		os.Remove(filePath)
	}

	return os.Rename(outputFile, filepath.Join(jsDir, "engine.js"))
}
