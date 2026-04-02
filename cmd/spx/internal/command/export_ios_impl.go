package command

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

type iosLibraryPaths struct {
	libDir          string
	xcframeworkPath string
	buildDir        string
	simulatorDir    string
	deviceDir       string
	headersDir      string
	goSrcDir        string
}

type iosSDKPaths struct {
	simulator string
	device    string
}

type iosArchiveBuild struct {
	name       string
	outputPath string
	env        []string
}

// ExportIos exports the current project as an iOS IPA.
func (cmd *CmdTool) ExportIos() error {
	fmt.Println("===> starting iOS IPA export process...")

	if err := cmd.prepareExport(); err != nil {
		return err
	}
	cmd.BuildDll()
	if err := cmd.renameIosArtifacts(); err != nil {
		return err
	}

	fmt.Println("===> building iOS libraries...")
	if err := cmd.buildIosLibraries(); err != nil {
		return fmt.Errorf("failed to build iOS libraries: %w", err)
	}
	fmt.Println("===> built iOS libraries successfully")

	ipaPath, err := cmd.prepareIosOutput()
	if err != nil {
		return err
	}

	if err := cmd.validateIosExportInputs(); err != nil {
		return err
	}
	cmd.logIosExportTemplates()
	cmd.importIosProjectResources()

	if err := cmd.exportIosIPA(ipaPath); err != nil {
		return err
	}
	return cmd.installIosIPA(ipaPath)
}

func (cmd *CmdTool) renameIosArtifacts() error {
	files, err := filepath.Glob(filepath.Join(cmd.ProjectDir, "go", "ios*"))
	if err != nil {
		return fmt.Errorf("failed to list iOS artifacts: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".txt") {
			continue
		}
		newName := strings.TrimSuffix(file, ".txt")
		if err := os.Rename(file, newName); err != nil {
			return fmt.Errorf("failed to rename %s: %w", file, err)
		}
	}
	return nil
}

func (cmd *CmdTool) prepareIosOutput() (string, error) {
	ipaPath := filepath.Join(cmd.ProjectDir, ".builds", "ios", "Game.ipa")
	buildDir := filepath.Dir(ipaPath)
	fmt.Printf("===> output IPA path: %s\n", ipaPath)

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}
	fmt.Printf("===> build directory created: %s\n", buildDir)
	return ipaPath, nil
}

func (cmd *CmdTool) validateIosExportInputs() error {
	if _, err := os.Stat(cmd.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("godot binary not found at %s", cmd.CmdPath)
	}
	fmt.Printf("===> found Godot binary at: %s\n", cmd.CmdPath)

	projectFilePath := filepath.Join(cmd.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("godot project file not found at %s", projectFilePath)
	}
	fmt.Printf("===> found Godot project file at: %s\n", projectFilePath)
	return nil
}

func (cmd *CmdTool) logIosExportTemplates() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	templateDir := ""
	switch runtime.GOOS {
	case "darwin":
		templateDir = filepath.Join(homeDir, "Library", "Application Support", "Godot", "export_templates")
	case "linux":
		templateDir = filepath.Join(homeDir, ".local", "share", "godot", "export_templates")
	case "windows":
		templateDir = filepath.Join(os.Getenv("APPDATA"), "Godot", "export_templates")
	}
	if templateDir == "" {
		return
	}

	fmt.Printf("===> checking export templates at: %s\n", templateDir)
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		fmt.Printf("===> warning: could not read template directory: %v\n", err)
		return
	}

	fmt.Println("===> available export template versions:")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(templateDir, entry.Name())
		fmt.Printf("     - %s\n", entry.Name())

		files, err := os.ReadDir(versionDir)
		if err != nil {
			continue
		}

		var iosFiles []string
		for _, file := range files {
			if strings.Contains(file.Name(), "ios") {
				iosFiles = append(iosFiles, file.Name())
			}
		}
		if len(iosFiles) > 0 {
			fmt.Printf("       iOS templates: %v\n", iosFiles)
		}
	}
}

func (cmd *CmdTool) importIosProjectResources() {
	fmt.Println("===> importing project resources...")
	if err := util.ExecCommand(
		util.CommandOptions{},
		cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--editor", "--quit",
	); err != nil {
		fmt.Printf("===> warning: project import had issues: %v\n", err)
	}
}

func (cmd *CmdTool) exportIosIPA(ipaPath string) error {
	fmt.Println("===> exporting Godot project to IPA...")
	fmt.Printf("===> export command: %s --headless --path %s --export-debug iOS %s\n",
		cmd.CmdPath, cmd.ProjectDir, ipaPath)

	if err := util.ExecCommand(
		util.CommandOptions{},
		cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--export-debug", "iOS", ipaPath,
	); err != nil {
		return fmt.Errorf("failed to export IPA: %w", err)
	}
	if _, err := os.Stat(ipaPath); os.IsNotExist(err) {
		return fmt.Errorf("failed to export IPA: file not created at %s", ipaPath)
	}

	log.Println("===> exported IPA successfully:", ipaPath)
	return nil
}

func (cmd *CmdTool) installIosIPA(ipaPath string) error {
	if !*cmd.Args.Install {
		return nil
	}

	log.Println("trying to install IPA to devices...")
	if err := util.ExecCommand(util.CommandOptions{}, "ios-deploy", "--bundle", ipaPath); err != nil {
		return fmt.Errorf("failed to install IPA: %w", err)
	}
	return nil
}

func (cmd *CmdTool) buildIosLibraries() error {
	paths := cmd.iosLibraryPaths()
	if err := prepareIosLibraryDirs(paths); err != nil {
		return err
	}

	fmt.Println("📦 building Go libraries for iOS...")
	if err := cmd.prepareIosHeaders(paths); err != nil {
		return err
	}

	sdkPaths, err := lookupIosSDKPaths()
	if err != nil {
		return err
	}
	if err := cmd.buildIosArchives(paths, sdkPaths); err != nil {
		return err
	}
	if err := createIosSimulatorFatBinary(paths); err != nil {
		return err
	}
	if err := createIosXCFramework(paths); err != nil {
		return err
	}

	fmt.Println("🧹 cleaning up temporary build files...")
	if err := os.RemoveAll(paths.buildDir); err != nil {
		return fmt.Errorf("failed to remove temporary build directory: %w", err)
	}

	fmt.Println("✅ built libgdspx.ios.xcframework successfully")
	fmt.Println("📍 location:", paths.xcframeworkPath)
	return nil
}

func (cmd *CmdTool) iosLibraryPaths() iosLibraryPaths {
	libDir := filepath.Join(cmd.ProjectDir, "lib")
	buildDir := filepath.Join(cmd.ProjectDir, ".godot", "tmp", "gobuild")
	return iosLibraryPaths{
		libDir:          libDir,
		xcframeworkPath: filepath.Join(libDir, "libgdspx.ios.xcframework"),
		buildDir:        buildDir,
		simulatorDir:    filepath.Join(buildDir, "simulator"),
		deviceDir:       filepath.Join(buildDir, "device"),
		headersDir:      filepath.Join(buildDir, "headers"),
		goSrcDir:        filepath.Join(cmd.ProjectDir, "go"),
	}
}

func prepareIosLibraryDirs(paths iosLibraryPaths) error {
	if err := os.RemoveAll(paths.buildDir); err != nil {
		return fmt.Errorf("failed to clean build directory: %w", err)
	}
	if err := os.RemoveAll(paths.xcframeworkPath); err != nil {
		return fmt.Errorf("failed to clean xcframework path: %w", err)
	}
	for _, dir := range []string{paths.simulatorDir, paths.deviceDir, paths.libDir, paths.headersDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (cmd *CmdTool) prepareIosHeaders(paths iosLibraryPaths) error {
	const headerContent = `#ifndef LIBGDSPX_H
#define LIBGDSPX_H

#include <stdlib.h>

// GDExtension entry point.
void GDExtensionInit(void *p_interface, const void *p_library, void *r_initialization);

#endif // LIBGDSPX_H
`
	if err := os.WriteFile(filepath.Join(paths.headersDir, "libgdspx.h"), []byte(headerContent), 0o644); err != nil {
		return fmt.Errorf("failed to create header file: %w", err)
	}

	headerFiles, err := filepath.Glob(filepath.Join(paths.goSrcDir, "*.h"))
	if err != nil {
		return fmt.Errorf("failed to find header files: %w", err)
	}
	for _, headerFile := range headerFiles {
		destFile := filepath.Join(paths.headersDir, filepath.Base(headerFile))
		if err := util.CopyFile(headerFile, destFile); err != nil {
			return fmt.Errorf("failed to copy header file %s: %w", headerFile, err)
		}
	}
	return nil
}

func lookupIosSDKPaths() (iosSDKPaths, error) {
	simulator, err := util.OutputCommand(util.CommandOptions{}, "xcrun", "--sdk", "iphonesimulator", "--show-sdk-path")
	if err != nil {
		return iosSDKPaths{}, fmt.Errorf("failed to get simulator SDK path: %w", err)
	}
	device, err := util.OutputCommand(util.CommandOptions{}, "xcrun", "--sdk", "iphoneos", "--show-sdk-path")
	if err != nil {
		return iosSDKPaths{}, fmt.Errorf("failed to get device SDK path: %w", err)
	}

	simulatorPath := strings.TrimSpace(string(simulator))
	devicePath := strings.TrimSpace(string(device))
	if err := validateSDKPath("simulator", simulatorPath); err != nil {
		return iosSDKPaths{}, err
	}
	if err := validateSDKPath("device", devicePath); err != nil {
		return iosSDKPaths{}, err
	}

	return iosSDKPaths{
		simulator: simulatorPath,
		device:    devicePath,
	}, nil
}

func validateSDKPath(name, sdkPath string) error {
	if strings.ContainsAny(sdkPath, " \t\n") {
		return fmt.Errorf("%s SDK path contains whitespace: %q", name, sdkPath)
	}
	return nil
}

func (cmd *CmdTool) buildIosArchives(paths iosLibraryPaths, sdkPaths iosSDKPaths) error {
	builds := []iosArchiveBuild{
		newIosArchiveBuild(
			"iOS Simulator (x86_64)",
			filepath.Join(paths.simulatorDir, "libgdspx-x86_64.a"),
			sdkPaths.simulator,
			"amd64",
			"-mios-simulator-version-min=12.0 -arch x86_64 -fembed-bitcode",
			"-mios-simulator-version-min=12.0 -arch x86_64",
		),
		newIosArchiveBuild(
			"iOS Simulator (arm64)",
			filepath.Join(paths.simulatorDir, "libgdspx-arm64-sim.a"),
			sdkPaths.simulator,
			"arm64",
			"-mios-simulator-version-min=12.0 -arch arm64 -fembed-bitcode",
			"-mios-simulator-version-min=12.0 -arch arm64",
		),
		newIosArchiveBuild(
			"iOS Device (arm64)",
			filepath.Join(paths.deviceDir, "libgdspx-arm64.a"),
			sdkPaths.device,
			"arm64",
			"-mios-version-min=12.0 -arch arm64 -fembed-bitcode",
			"-mios-version-min=12.0 -arch arm64",
		),
	}

	for _, build := range builds {
		fmt.Printf("🔨 building for %s...\n", build.name)
		if err := util.ExecCommand(util.CommandOptions{Env: build.env, Dir: paths.goSrcDir}, "go",
			"build", "-tags=ios,packmode", "-buildmode=c-archive", "-trimpath", "-ldflags=-w -s",
			"-o", build.outputPath, ".",
		); err != nil {
			return fmt.Errorf("failed to build for %s: %w", build.name, err)
		}
	}
	return nil
}

func newIosArchiveBuild(name, outputPath, sdkPath, goarch, cgoCFlags, cgoLDFlags string) iosArchiveBuild {
	return iosArchiveBuild{
		name:       name,
		outputPath: outputPath,
		env: []string{
			"GODEBUG=cgocheck=0,asyncpreemptoff=1,panicnil=1",
			"CGO_ENABLED=1",
			"GOOS=darwin",
			"GOARCH=" + goarch,
			"CGO_CFLAGS=-isysroot " + sdkPath + " " + cgoCFlags,
			"CGO_LDFLAGS=-isysroot " + sdkPath + " " + cgoLDFlags,
		},
	}
}

func createIosSimulatorFatBinary(paths iosLibraryPaths) error {
	fmt.Println("🔗 creating fat binary for simulator...")
	if err := util.ExecCommand(util.CommandOptions{}, "lipo", "-create", "-output",
		filepath.Join(paths.simulatorDir, "libgdspx.a"),
		filepath.Join(paths.simulatorDir, "libgdspx-x86_64.a"),
		filepath.Join(paths.simulatorDir, "libgdspx-arm64-sim.a"),
	); err != nil {
		return fmt.Errorf("failed to create fat binary for simulator: %w", err)
	}
	return nil
}

func createIosXCFramework(paths iosLibraryPaths) error {
	fmt.Println("🎁 creating XCFramework...")
	if err := util.ExecCommand(util.CommandOptions{}, "xcrun", "xcodebuild", "-create-xcframework",
		"-library", filepath.Join(paths.simulatorDir, "libgdspx.a"), "-headers", paths.headersDir,
		"-library", filepath.Join(paths.deviceDir, "libgdspx-arm64.a"), "-headers", paths.headersDir,
		"-output", paths.xcframeworkPath,
	); err != nil {
		return fmt.Errorf("failed to create XCFramework: %w", err)
	}
	return nil
}
