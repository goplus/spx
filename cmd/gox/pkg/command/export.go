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
	"github.com/goplus/spx/v2/internal/webhost"
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

const (
	webWorkerMode      = "worker"
	webMinigameMode    = "minigame"
	webMiniprogramMode = "miniprogram"
	webNormalMode      = "normal"
)

func validateWebMode(mode string) error {
	switch mode {
	case webNormalMode, webWorkerMode, webMinigameMode, webMiniprogramMode:
		return nil
	default:
		return fmt.Errorf("invalid web mode %q", mode)
	}
}

func webRuntimeZipName(mode string) string {
	switch mode {
	case webWorkerMode:
		return "spx_web_worker.zip"
	case webMinigameMode:
		return "spx_web_minigame.zip"
	case webMiniprogramMode:
		return "spx_web_miniprogram.zip"
	default:
		return "spx_web.zip"
	}
}

func (pself *CmdTool) ExportWeb() error {
	if err := pself.exportWebCommon(webNormalMode); err != nil {
		return err
	}
	if err := pself.applyWebNormalLayout(pself.WebDir); err != nil {
		return fmt.Errorf("failed to copy web platform assets: %w", err)
	}
	return nil
}

func (pself *CmdTool) ExportMinigame() error {
	if err := pself.exportWebCommon(webMinigameMode); err != nil {
		return err
	}
	return pself.applyWebMinigameLayout(pself.WebDir, *pself.Args.Build)
}

func (pself *CmdTool) ExportMiniprogram() error {
	if err := pself.exportWebCommon(webMiniprogramMode); err != nil {
		return err
	}
	if err := pself.applyWebMiniprogramLayout(pself.WebDir); err != nil {
		return fmt.Errorf("failed to copy miniprogram platform assets: %w", err)
	}
	return nil
}

func (pself *CmdTool) ExportWebWorker() error {
	if err := pself.exportWebCommon(webWorkerMode); err != nil {
		return err
	}
	return pself.applyWebWorkerLayout(pself.WebDir)
}

func (pself *CmdTool) exportWebCommon(mode string) error {
	if err := validateWebMode(mode); err != nil {
		return err
	}
	if err := pself.Clear(); err != nil {
		return err
	}
	if err := util.CopyDir(pself.ProjectFS, "template/project", pself.ProjectDir, true); err != nil {
		return fmt.Errorf("failed to copy project scaffold: %w", err)
	}
	dir := pself.TargetDir
	if err := util.SetupFile(false, filepath.Join(dir, ".gitignore"), pself.GitignoreTxt); err != nil {
		return fmt.Errorf("failed to write project .gitignore: %w", err)
	}
	if err := os.Rename(filepath.Join(dir, ".gitignore.txt"), filepath.Join(dir, ".gitignore")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to rename project .gitignore: %w", err)
	}
	if err := pself.stageWebRuntimeBase(pself.WebDir, mode); err != nil {
		return err
	}
	pack.PackProject(pself.TargetDir, filepath.Join(pself.WebDir, "game.zip"))
	return nil
}

func (pself *CmdTool) ExportWebRuntime() error {
	mode := webNormalMode
	if pself.Args.Mode != nil && *pself.Args.Mode != "" && *pself.Args.Mode != "none" {
		mode = *pself.Args.Mode
	}
	if err := validateWebMode(mode); err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "spx-webruntime-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary runtime dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	stageDir := filepath.Join(workDir, "runtime")
	if err := pself.stageWebRuntimeBase(stageDir, mode); err != nil {
		return err
	}

	switch mode {
	case webNormalMode:
		if err := pself.applyWebNormalLayout(stageDir); err != nil {
			return err
		}
	case webWorkerMode:
		if err := pself.applyWebWorkerLayout(stageDir); err != nil {
			return err
		}
	case webMinigameMode:
		if err := pself.applyWebMinigameLayout(stageDir, *pself.Args.Build); err != nil {
			return err
		}
	case webMiniprogramMode:
		if err := pself.applyWebMiniprogramLayout(stageDir); err != nil {
			return err
		}
	}

	outputPath, err := filepath.Abs(webRuntimeZipName(mode))
	if err != nil {
		return fmt.Errorf("failed to resolve runtime zip path: %w", err)
	}
	if err := pack.PackDir(stageDir, outputPath); err != nil {
		return fmt.Errorf("failed to pack web runtime bundle: %w", err)
	}
	fmt.Println(outputPath, "has been created")
	return nil
}

func (pself *CmdTool) stageWebRuntimeBase(dstPath, mode string) error {
	if err := validateWebMode(mode); err != nil {
		return err
	}
	templateDir := filepath.Join(pself.GoBinPath, "gdspxrt"+pself.Version+"_web"+mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web dir file not found: " + templateDir)
	}

	if err := os.MkdirAll(dstPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create web runtime dir: %w", err)
	}
	if err := util.CopyDir2(templateDir, dstPath); err != nil {
		return fmt.Errorf("failed to copy engine web template: %w", err)
	}

	println("==> _stageWebRuntime", dstPath)
	if err := os.Rename(filepath.Join(dstPath, "godot.editor.html"), filepath.Join(dstPath, "index.html")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to rename godot.editor.html: %w", err)
	}

	// Copy host runtime assets that bridge the engine runtime and ispx wasm.
	if err := util.CopyDir(webhost.Assets, ".", dstPath, true); err != nil {
		return fmt.Errorf("failed to copy web host runtime assets: %w", err)
	}

	// Copy gox-specific common web files (index.html, fflate.js, engine.worker.js).
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web", dstPath, true); err != nil {
		return fmt.Errorf("failed to copy common web platform assets: %w", err)
	}

	// copy wasm_exec.js from GOROOT
	output, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("failed to get GOROOT: %w", err)
	}
	goroot := strings.TrimSpace(string(output))
	wasmExecPath := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	if err := util.CopyFile(wasmExecPath, filepath.Join(dstPath, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to copy wasm_exec.js: %w", err)
	}
	if err := util.CopyFile(pself.getWasmPath(), filepath.Join(dstPath, "ispx.wasm")); err != nil {
		return fmt.Errorf("failed to copy ispx.wasm: %w", err)
	}
	if err := util.CopyFile(pself.getWasmPath()+".br", filepath.Join(dstPath, "ispx.wasm.br")); err != nil {
		return fmt.Errorf("failed to copy ispx.wasm.br: %w", err)
	}
	return nil
}

func (pself *CmdTool) applyWebNormalLayout(webDir string) error {
	return util.CopyDir(pself.PlatformFS, "template/platform/web"+webNormalMode, webDir, true)
}

func (pself *CmdTool) applyWebMiniprogramLayout(webDir string) error {
	return util.CopyDir(pself.PlatformFS, "template/platform/web"+webMiniprogramMode, webDir, true)
}

func (pself *CmdTool) applyWebWorkerLayout(webDir string) error {
	extDir := filepath.Join(webDir, "__"+webWorkerMode)
	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webWorkerMode, extDir, true); err != nil {
		return fmt.Errorf("failed to copy web worker platform assets: %w", err)
	}

	var filesToMerge []string
	if err := os.Rename(filepath.Join(webDir, "go.wasm.exec.js"), filepath.Join(extDir, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to move go.wasm.exec.js into worker extension dir: %w", err)
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

	engineBytes, err := os.ReadFile(filepath.Join(webDir, "engine.js"))
	if err != nil {
		return err
	}
	engineStr := string(engineBytes)

	keyStr := "{if(initializedJS){checkMailbox()}}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js does not contain worker mailbox insertion point: %s", keyStr)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameAppMessageId) {handleGameAppMessage(e.data);}")

	keyStr = ";throw ex}}self.onmessage=handleMessage}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js does not contain worker bootstrap insertion point: %s", keyStr)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+insertCode)
	if err := os.WriteFile(filepath.Join(webDir, "engine.js"), []byte(engineStr), 0o644); err != nil {
		return err
	}

	if err := os.RemoveAll(extDir); err != nil {
		return err
	}
	return nil
}

func (pself *CmdTool) applyWebMinigameLayout(webDir, buildMode string) error {
	if err := os.Rename(webDir, webDir+"_bck"); err != nil {
		return fmt.Errorf("failed to backup raw web runtime: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(webDir), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create minigame web dir: %w", err)
	}
	rawWebDir := filepath.Join(webDir, "rawWeb")
	if err := os.Rename(webDir+"_bck", rawWebDir); err != nil {
		return fmt.Errorf("failed to move raw web runtime into minigame layout: %w", err)
	}

	if err := util.CopyDir(pself.PlatformFS, "template/platform/web"+webMinigameMode, webDir, true); err != nil {
		return fmt.Errorf("failed to copy minigame platform assets: %w", err)
	}

	engineDir := filepath.Join(webDir, "engine")
	jsDir := filepath.Join(webDir, "js")
	if err := os.MkdirAll(engineDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create minigame engine dir: %w", err)
	}
	if err := os.MkdirAll(jsDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create minigame js dir: %w", err)
	}

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

	if err := os.RemoveAll(rawWebDir); err != nil {
		return err
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
func (pself *CmdTool) mergeJSFiles(jsDir string, isCompressed bool) error {
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

	// write compression flag at the beginning
	compressionFlag := fmt.Sprintf("var FFI = null;\nconst isWasmCompressed = %t;\n\n", isCompressed)
	if _, err := writer.WriteString(compressionFlag); err != nil {
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
