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

	"github.com/goplus/spx/v2/cmd/gox/pkg/pack"
	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

func (pself *CmdTool) prepareExport() error {
	// copy assets
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
	// delete gdextension configs
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
	if err := pself.exportWebCommon(webNormalMode); err != nil {
		return err
	}
	// copy minigame files
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webNormalMode, pself.WebDir, true); err != nil {
		return fmt.Errorf("failed to copy web template files: %w", err)
	}
	return nil
}

func (pself *CmdTool) ExportMinigame() error {
	if err := pself.exportWebCommon(webMinigameMode); err != nil {
		return err
	}
	// move to subdir
	if err := os.Rename(pself.WebDir, pself.WebDir+"_bck"); err != nil {
		return fmt.Errorf("failed to backup web output directory: %w", err)
	}
	os.MkdirAll(filepath.Join(pself.WebDir), os.ModePerm)
	if err := os.Rename(pself.WebDir+"_bck", filepath.Join(pself.WebDir, "rawWeb")); err != nil {
		return fmt.Errorf("failed to move raw web files: %w", err)
	}

	// copy minigame files
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webMinigameMode, pself.WebDir, true); err != nil {
		return fmt.Errorf("failed to copy minigame template files: %w", err)
	}

	workDir := pself.WebDir

	// create target directories
	engineDir := filepath.Join(workDir, "engine")
	jsDir := filepath.Join(workDir, "js")
	rawWebDir := filepath.Join(workDir, "rawWeb")

	os.MkdirAll(engineDir, os.ModePerm)
	os.MkdirAll(jsDir, os.ModePerm)

	// handle WASM files based on build mode
	godotEditorWasm := filepath.Join(rawWebDir, "engine.wasm")
	ispxWasm := filepath.Join(rawWebDir, "ispx.wasm")

	if err := pself.moveFile(godotEditorWasm, filepath.Join(engineDir, "engine.wasm")); err != nil {
		return fmt.Errorf("failed to move %s: %w", godotEditorWasm, err)
	}

	if err := pself.moveFile(ispxWasm, filepath.Join(engineDir, "ispx.wasm")); err != nil {
		return fmt.Errorf("failed to move %s: %w", ispxWasm, err)
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
	if err := pself.mergeJSFiles(jsDir); err != nil {
		return fmt.Errorf("failed to merge JS files: %w", err)
	}

	// remove minigame directory
	if err := os.RemoveAll(rawWebDir); err != nil {
		return fmt.Errorf("failed to cleanup raw web directory: %w", err)
	}

	// optionally open WeChat Developer Tools
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
	if err := pself.exportWebCommon(webMiniprogramMode); err != nil {
		return err
	}
	// copy miniprogram files
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webMiniprogramMode, pself.WebDir, true); err != nil {
		return fmt.Errorf("failed to copy miniprogram template files: %w", err)
	}
	return nil
}

func (pself *CmdTool) ExportWebWorker() error {
	if err := pself.exportWebCommon(webWorkerMode); err != nil {
		return err
	}
	extDir := filepath.Join(pself.WebDir, "__"+webWorkerMode)
	// copy miniprogram files
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webWorkerMode, extDir, true); err != nil {
		return fmt.Errorf("failed to copy web worker template files: %w", err)
	}

	var filesToMerge []string
	// merge ext/*.js to engine.worker.js
	if err := os.Rename(filepath.Join(pself.WebDir, "go.wasm.exec.js"), filepath.Join(extDir, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to move go.wasm.exec.js: %w", err)
	}
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

	// insert worker code
	engineBytes, err := os.ReadFile(filepath.Join(pself.WebDir, "engine.js"))
	if err != nil {
		return fmt.Errorf("failed to read engine.js: %w", err)
	}
	engineStr := string(engineBytes)

	// 1. insert handleGameAppMessage, dirty code to fix minigame
	keyStr := "{if(initializedJS){checkMailbox()}}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js does not contain expected marker: %s", keyStr)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameAppMessageId) {handleGameAppMessage(e.data);}")
	// 2. insert worker code , dirty code to fix minigame
	keyStr = ";throw ex}}self.onmessage=handleMessage}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js does not contain expected marker: %s", keyStr)
	}

	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+insertCode)
	if err := os.WriteFile(filepath.Join(pself.WebDir, "engine.js"), []byte(engineStr), 0644); err != nil {
		return fmt.Errorf("failed to write engine.js: %w", err)
	}

	if err := os.RemoveAll(extDir); err != nil {
		return fmt.Errorf("failed to remove temporary worker directory: %w", err)
	}
	return nil
}

func (pself *CmdTool) exportWebCommon(mode string) error {
	if err := pself.Clear(); err != nil {
		return err
	}
	templateDir := pself.getTemplateDir(mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web template not found: " + templateDir + ". Run 'make setup-web MODE=" + mode + "'")
	}

	dstPath := filepath.Join(pself.ProjectDir, ".builds", "web")
	os.MkdirAll(dstPath, os.ModePerm)
	if err := util.CopyDir2(templateDir, dstPath); err != nil {
		return fmt.Errorf("failed to copy web template from %s to %s: %w", templateDir, dstPath, err)
	}

	println("==> _exportWeb", dstPath)
	// copy project files
	if err := util.CopyDir(pself.ProjectFS, "template/project", pself.ProjectDir, true); err != nil {
		return fmt.Errorf("failed to copy project template files: %w", err)
	}
	dir := pself.TargetDir
	if err := util.SetupFile(false, filepath.Join(dir, ".gitignore"), pself.GitignoreTxt); err != nil {
		return fmt.Errorf("failed to setup .gitignore: %w", err)
	}

	if err := os.Rename(filepath.Join(dstPath, "godot.editor.html"), filepath.Join(dstPath, "index.html")); err != nil {
		return fmt.Errorf("failed to rename godot.editor.html to index.html: %w", err)
	}

	// Copy ispx web runtime files from share/ispx
	ispxWebDir, err := pself.getIspxWebDir()
	if err != nil {
		return err
	}
	if err := util.CopyDir2(ispxWebDir, pself.WebDir); err != nil {
		return fmt.Errorf("failed to copy ispx web runtime files: %w", err)
	}

	// Copy gox-specific web files (index.html, fflate.js, engine.worker.js)
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web", pself.WebDir, true); err != nil {
		return fmt.Errorf("failed to copy gox web files: %w", err)
	}

	// copy wasm_exec.js from share dir
	wasmExecPath := filepath.Join(pself.ShareDir, "wasm_exec.js")
	if err := util.CopyFile(wasmExecPath, filepath.Join(pself.WebDir, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to copy wasm_exec.js: %w", err)
	}
	// Append ext/*.js to engine.worker.js then remove them

	pack.PackProject(pself.TargetDir, filepath.Join(pself.WebDir, "game.zip"))
	//pack.PackEngineRes(pself.ProjectFS, pself.WebDir)
	if err := util.CopyFile(pself.getWasmPath(), filepath.Join(pself.WebDir, "ispx.wasm")); err != nil {
		return fmt.Errorf("failed to copy ispx.wasm: %w", err)
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

	// include ios files to build
	files, _ := filepath.Glob(filepath.Join(pself.ProjectDir, "go", "ios*"))
	for _, file := range files {
		if strings.HasSuffix(file, ".txt") {
			newName := strings.TrimSuffix(file, ".txt")
			os.Rename(file, newName)
		}
	}

	// First build the iOS libraries
	fmt.Println("===> Building iOS libraries...")
	if err := pself.buildIosLibraries(); err != nil {
		return fmt.Errorf("failed to build iOS libraries: %w", err)
	}
	fmt.Println("===> iOS libraries build completed successfully!")

	// Set up paths
	ipaPath := filepath.Join(pself.ProjectDir, ".builds", "ios", "Game.ipa")
	buildDir := filepath.Dir(ipaPath)
	fmt.Printf("===> IPA output path: %s\n", ipaPath)

	// Create builds directory if it doesn't exist
	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	fmt.Printf("===> Build directory created: %s\n", buildDir)

	// Check if Godot binary exists
	if _, err := os.Stat(pself.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", pself.CmdPath)
	}
	fmt.Printf("===> Godot binary found at: %s\n", pself.CmdPath)

	// Check if project file exists
	projectFilePath := filepath.Join(pself.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}
	fmt.Printf("===> Godot project file found at: %s\n", projectFilePath)

	// Check for export templates
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
						// Check for iOS templates
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

	// Import project to ensure resources are up to date
	fmt.Println("===> Importing project resources...")
	cmd := exec.Command(pself.CmdPath, "--headless", "--path", pself.ProjectDir, "--editor", "--quit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("===> Warning: Project import had issues: %v\n", err)
	}

	// Export the project to IPA
	fmt.Println("===> Exporting Godot project to IPA...")
	fmt.Printf("===> Export command: %s --headless --path %s --export-debug iOS %s\n",
		pself.CmdPath, pself.ProjectDir, ipaPath)

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

	log.Println("===> IPA export completed successfully!", ipaPath)
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

	// Define build configurations for different Android architectures
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

	// Build for each architecture
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

// moveFile moves a single file from source to destination
func (pself *CmdTool) moveFile(srcFile, dstFile string) error {
	return os.Rename(srcFile, dstFile)
}

// moveFilesByPattern moves files matching a pattern
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

// mergeJSFiles merges JavaScript files
func (pself *CmdTool) mergeJSFiles(jsDir string) error {
	// file merge order
	jsFiles := []string{"header.js", "engine.js", "go.wasm.exec.js", "worker.message.manager.js", "game.js"}
	outputFile := filepath.Join(jsDir, "engine_new.js")

	// create output file
	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := bufio.NewWriter(output)
	defer writer.Flush()

	if _, err := writer.WriteString("var FFI = null;\n\n"); err != nil {
		return err
	}

	// merge file contents
	for _, jsFile := range jsFiles {
		filePath := filepath.Join(jsDir, jsFile)
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
	return os.Rename(outputFile, filepath.Join(jsDir, "engine.js"))
}
