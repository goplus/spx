/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
)

type commandSetup uint8

const (
	noSetup commandSetup = iota
	pathSetup
	projectSetup
	interpretedSetup
)

type commandBuild uint8

const (
	noBuild commandBuild = iota
	dllBuild
	wasmBuild
	tinyGoBuild
)

type commandSpec struct {
	name, group, summary string
	setup                commandSetup
	build                commandBuild
	runtime, hidden      bool
	run                  func(*CmdTool) error
	unavailable          bool
}

var commandSpecs = []commandSpec{
	{name: "help", group: "Basic Commands", summary: "Display help information"},
	{name: "version", group: "Basic Commands", summary: "Display version information"},
	{name: "init", group: "Project Management", summary: "Create a project in the current directory", run: (*CmdTool).Init},
	{name: "clear", group: "Project Management", summary: "Clear the project", setup: pathSetup, run: (*CmdTool).Clear},
	{name: "clearbuild", group: "Project Management", summary: "Clear build artifacts", setup: pathSetup, run: (*CmdTool).ClearBuild},
	{name: "build", group: "Development & Building", summary: "Build the dynamic library", setup: projectSetup, build: dllBuild},
	{name: "buildlauncher", group: "Development & Building", summary: "Build a launcher without Godot or an XGo driver", run: (*CmdTool).runBuildLauncher},
	{name: "buildtinygo", group: "Development & Building", summary: "Build a TinyGo static library for ESP32", setup: projectSetup, build: tinyGoBuild},
	{name: "run", group: "Development & Building", summary: "Run in interpreted mode", setup: interpretedSetup, run: (*CmdTool).handleInterpretedRunCommand},
	{name: "runnative", group: "Development & Building", summary: "Run with the native PC runtime", setup: projectSetup, build: dllBuild, runtime: true, run: (*CmdTool).executeRunNative},
	{name: "editor", group: "Development & Building", summary: "Open the project in editor mode", setup: projectSetup, build: dllBuild, run: (*CmdTool).executeEditor},
	{name: "rune", group: "Development & Building", summary: "Run with engine after importing assets", setup: projectSetup, build: dllBuild, run: (*CmdTool).executeRune},
	{name: "buildweb", group: "Web Development", summary: "Build for WebAssembly (WASM)", setup: projectSetup, build: wasmBuild},
	{name: "runweb", group: "Web Development", summary: "Launch the web server", setup: projectSetup, build: wasmBuild, runtime: true, run: (*CmdTool).RunWeb},
	{name: "runwebworker", group: "Web Development", summary: "Run with a web worker", setup: projectSetup, build: wasmBuild, runtime: true, run: (*CmdTool).RunWebWorker},
	{name: "exportweb", group: "Web Development", summary: "Export the web package", setup: projectSetup, build: wasmBuild, run: (*CmdTool).ExportWeb},
	{name: "exportwebworker", group: "Web Development", summary: "Export the web worker package", setup: projectSetup, run: (*CmdTool).ExportWebWorker},
	{name: "exporttemplateweb", group: "Web Development", summary: "Export the web template project", setup: projectSetup, run: (*CmdTool).ExportTemplateWeb},
	{name: "stopweb", group: "Web Development", summary: "Stop the web server", setup: pathSetup, run: (*CmdTool).StopWeb},
	{name: "export", group: "Export & Distribution", summary: "Export the PC package", setup: projectSetup, build: dllBuild, run: (*CmdTool).Export},
	{name: "exportpack", group: "Export & Distribution", summary: "Export project data for the desktop runtime", setup: projectSetup, build: dllBuild, hidden: true, run: (*CmdTool).ExportPack},
	{name: "exportapk", group: "Export & Distribution", summary: "Export an Android APK", setup: projectSetup, run: (*CmdTool).ExportApk},
	{name: "exportios", group: "Export & Distribution", summary: "Export an iOS package", setup: projectSetup, run: (*CmdTool).ExportIos},
	{name: "exportminigame", group: "Export & Distribution", summary: "Export a minigame package", setup: projectSetup, run: (*CmdTool).ExportMinigame},
	{name: "exportminiprogram", group: "Export & Distribution", summary: "Export a mini program package", setup: projectSetup, run: (*CmdTool).ExportMiniprogram},
	{name: "exportbot", group: "Export & Distribution", summary: "Reserved; not implemented", unavailable: true},
	{name: "runm", group: "Multiplayer", summary: "Reserved; not implemented", unavailable: true},
}

func findCommand(name string) (commandSpec, bool) {
	for _, spec := range commandSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return commandSpec{}, false
}

func (cmd *CmdTool) dispatch(spec commandSpec, fsRelDir, dstRelDir string) error {
	if spec.unavailable {
		return fmt.Errorf("command %q is not implemented", spec.name)
	}
	if spec.name == "help" || spec.name == "version" {
		cmd.ShowHelpInfo()
		return nil
	}
	if err := cmd.prepareCommand(spec, fsRelDir, dstRelDir); err != nil {
		return err
	}
	if err := cmd.buildCommand(spec.build); err != nil {
		return err
	}
	if spec.run == nil {
		return nil
	}
	return spec.run(cmd)
}

func (cmd *CmdTool) prepareCommand(spec commandSpec, fsRelDir, dstRelDir string) error {
	switch spec.setup {
	case noSetup:
		return nil
	case interpretedSetup:
		return cmd.setupInterpretedPaths(dstRelDir)
	case pathSetup, projectSetup:
		if err := cmd.setupPaths(dstRelDir); err != nil {
			return err
		}
		if spec.setup == pathSetup {
			return nil
		}
	default:
		return fmt.Errorf("command %q has an invalid setup", spec.name)
	}
	cmd.RuntimeMode = spec.runtime
	if err := cmd.CheckEnv(); err != nil {
		return err
	}
	// Work around goplus/spx#619.
	os.Setenv("GODEBUG", "asyncpreemptoff=1")
	cmd.WebDir, _ = filepath.Abs(filepath.Join(cmd.ProjectDir, ".builds", "web"))
	return cmd.SetupEnv(cmd.Version, cmd.ProjectFS, fsRelDir, dstRelDir)
}

func (cmd *CmdTool) buildCommand(build commandBuild) error {
	switch build {
	case noBuild:
		return nil
	case dllBuild:
		if cmd.usesPureEngine() {
			return nil
		}
		return cmd.BuildDll()
	case wasmBuild:
		return cmd.BuildWasm()
	case tinyGoBuild:
		return cmd.BuildTinyGoLib()
	default:
		return fmt.Errorf("invalid command build phase %d", build)
	}
}
