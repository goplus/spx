/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package command

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/cmd/spx/internal/util"
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

type iosExportOutput struct {
	buildDir         string
	ipaPath          string
	xcodeProjectPath string
}

const (
	iosExportPresetName     = "iOS"
	iosExportProjectOnlyKey = "application/export_project_only"
)

// ExportIos exports the current project for iOS.
func (cmd *CmdTool) ExportIos() error {
	logInfof("Starting iOS export process")

	if err := cmd.prepareExport(); err != nil {
		return err
	}
	cmd.BuildDll()
	if err := cmd.renameIosArtifacts(); err != nil {
		return err
	}

	logInfof("Building iOS libraries")
	if err := cmd.buildIosLibraries(); err != nil {
		return fmt.Errorf("failed to build iOS libraries: %w", err)
	}
	logInfof("Built iOS libraries successfully")

	exportProjectOnly, err := cmd.isIosExportProjectOnly()
	if err != nil {
		return err
	}
	if exportProjectOnly {
		logInfof("iOS export preset is configured for Xcode project-only output")
	}

	output, err := cmd.prepareIosOutput(exportProjectOnly)
	if err != nil {
		return err
	}

	if err := cmd.validateIosExportInputs(); err != nil {
		return err
	}
	cmd.logIosExportTemplates()
	cmd.importIosProjectResources()

	if err := cmd.exportIos(output, exportProjectOnly); err != nil {
		return err
	}
	return cmd.installIosIPA(output.ipaPath, exportProjectOnly)
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

func (cmd *CmdTool) prepareIosOutput(exportProjectOnly bool) (iosExportOutput, error) {
	output := iosExportOutput{
		buildDir:         filepath.Join(cmd.ProjectDir, ".builds", "ios"),
		ipaPath:          filepath.Join(cmd.ProjectDir, ".builds", "ios", "Game.ipa"),
		xcodeProjectPath: filepath.Join(cmd.ProjectDir, ".builds", "ios", "Game.xcodeproj"),
	}
	logInfof("iOS build output directory: %s", output.buildDir)
	if exportProjectOnly {
		logInfof("Expected Xcode project path: %s", output.xcodeProjectPath)
	} else {
		logInfof("Expected IPA path: %s", output.ipaPath)
	}

	if err := os.MkdirAll(output.buildDir, 0o755); err != nil {
		return iosExportOutput{}, fmt.Errorf("failed to create build directory: %w", err)
	}
	logInfof("Build directory created: %s", output.buildDir)
	return output, nil
}

func (cmd *CmdTool) validateIosExportInputs() error {
	if _, err := os.Stat(cmd.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("godot binary not found at %s", cmd.CmdPath)
	}
	logInfof("Found Godot binary at: %s", cmd.CmdPath)

	projectFilePath := filepath.Join(cmd.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("godot project file not found at %s", projectFilePath)
	}
	logInfof("Found Godot project file at: %s", projectFilePath)
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

	logInfof("Checking export templates at: %s", templateDir)
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		logWarnf("Could not read export template directory: %v", err)
		return
	}

	logInfof("Available export template versions")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(templateDir, entry.Name())
		logInfof("Export template version: %s", entry.Name())

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
			logInfof("Templates for iOS in %s: %v", entry.Name(), iosFiles)
		}
	}
}

func (cmd *CmdTool) importIosProjectResources() {
	logInfof("Importing project resources")
	if err := util.ExecCommand(
		util.CommandOptions{},
		cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--editor", "--quit",
	); err != nil {
		logWarnf("Project import had issues: %v", err)
	}
}

func (cmd *CmdTool) isIosExportProjectOnly() (bool, error) {
	exportPresetsPath := filepath.Join(cmd.ProjectDir, "export_presets.cfg")
	content, err := os.ReadFile(exportPresetsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read export presets %s: %w", exportPresetsPath, err)
	}
	return parseIosExportProjectOnly(content), nil
}

func parseIosExportProjectOnly(content []byte) bool {
	presetNames := make(map[string]string)
	projectOnlyValues := make(map[string]string)
	seenPresets := make(map[string]bool)
	var presetOrder []string
	var currentPreset string
	var currentOptions bool

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			preset, options, ok := parseExportPresetSection(line)
			if !ok {
				currentPreset = ""
				currentOptions = false
				continue
			}
			currentPreset = preset
			currentOptions = options
			if !options && !seenPresets[preset] {
				seenPresets[preset] = true
				presetOrder = append(presetOrder, preset)
			}
			continue
		}

		key, value, ok := parseExportPresetAssignment(line)
		if !ok || currentPreset == "" {
			continue
		}
		value = strings.Trim(value, "\"")

		if currentOptions {
			if key == iosExportProjectOnlyKey {
				projectOnlyValues[currentPreset] = value
			}
			continue
		}
		if key == "name" {
			presetNames[currentPreset] = value
		}
	}

	for _, preset := range presetOrder {
		if presetNames[preset] != iosExportPresetName {
			continue
		}
		return strings.EqualFold(projectOnlyValues[preset], "true")
	}
	return false
}

func parseExportPresetSection(line string) (preset string, options bool, ok bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false, false
	}

	section := strings.TrimSpace(line[1 : len(line)-1])
	if !strings.HasPrefix(section, "preset.") {
		return "", false, false
	}

	preset = strings.TrimPrefix(section, "preset.")
	if strings.HasSuffix(preset, ".options") {
		preset = strings.TrimSuffix(preset, ".options")
		options = true
	}
	if preset == "" || strings.Contains(preset, ".") {
		return "", false, false
	}
	return preset, options, true
}

func parseExportPresetAssignment(line string) (key string, value string, ok bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func (cmd *CmdTool) exportIos(output iosExportOutput, exportProjectOnly bool) error {
	logInfof("Exporting Godot project for iOS")
	logInfof("Export command: %s --headless --path %s --export-debug %s %s",
		cmd.CmdPath, cmd.ProjectDir, iosExportPresetName, output.ipaPath)

	if err := util.ExecCommand(
		util.CommandOptions{},
		cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--export-debug", iosExportPresetName, output.ipaPath,
	); err != nil {
		return fmt.Errorf("failed to export iOS build: %w", err)
	}

	return validateIosExportOutput(output, exportProjectOnly)
}

func validateIosExportOutput(output iosExportOutput, exportProjectOnly bool) error {
	if exportProjectOnly {
		if _, err := os.Stat(output.xcodeProjectPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("failed to export Xcode project: file not created at %s", output.xcodeProjectPath)
			}
			return fmt.Errorf("failed to check Xcode project output %s: %w", output.xcodeProjectPath, err)
		}
		logInfof("Exported Xcode project successfully: %s", output.xcodeProjectPath)
		return nil
	}

	if _, err := os.Stat(output.ipaPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to export IPA: file not created at %s", output.ipaPath)
		}
		return fmt.Errorf("failed to check IPA output %s: %w", output.ipaPath, err)
	}

	logInfof("Exported IPA successfully: %s", output.ipaPath)
	return nil
}

func (cmd *CmdTool) installIosIPA(ipaPath string, exportProjectOnly bool) error {
	if !*cmd.Args.Install {
		return nil
	}
	if exportProjectOnly {
		return fmt.Errorf("cannot install IPA when application/export_project_only=true")
	}

	logInfof("Installing IPA on connected devices")
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

	logInfof("Building Go libraries for iOS")
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

	logInfof("Cleaning up temporary build files")
	if err := os.RemoveAll(paths.buildDir); err != nil {
		return fmt.Errorf("failed to remove temporary build directory: %w", err)
	}

	logInfof("Built libgdspx.ios.xcframework successfully")
	logInfof("XCFramework location: %s", paths.xcframeworkPath)
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
		logInfof("Building iOS archive for %s", build.name)
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
	logInfof("Creating fat binary for simulator")
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
	logInfof("Creating XCFramework")
	if err := util.ExecCommand(util.CommandOptions{}, "xcrun", "xcodebuild", "-create-xcframework",
		"-library", filepath.Join(paths.simulatorDir, "libgdspx.a"), "-headers", paths.headersDir,
		"-library", filepath.Join(paths.deviceDir, "libgdspx-arm64.a"), "-headers", paths.headersDir,
		"-output", paths.xcframeworkPath,
	); err != nil {
		return fmt.Errorf("failed to create XCFramework: %w", err)
	}
	return nil
}
