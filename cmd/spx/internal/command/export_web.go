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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/cmd/spx/internal/pack"
	"github.com/goplus/spx/v4/cmd/spx/internal/util"
)

const (
	webNormalMode       = "normal"
	webWorkerMode       = "worker"
	webMinigameMode     = "minigame"
	webMiniprogramMode  = "miniprogram"
	webPlatformTemplate = "template/platform/web"
)

type minigamePaths struct {
	workDir   string
	engineDir string
	jsDir     string
	rawWebDir string
}

// ExportTemplateWeb exports the web template project.
func (cmd *CmdTool) ExportTemplateWeb() error {
	targetDir := filepath.Join(cmd.ProjectDir, ".builds", "webi")
	targetPath := filepath.Join(targetDir, "engine.html")
	platformName := "Web"
	os.Mkdir(targetDir, 0o755)
	os.Remove(filepath.Join(cmd.ProjectDir, "gdspx.gdextension"))
	os.Remove(filepath.Join(cmd.ProjectDir, ".godot", "extension_list.cfg"))
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, "--headless", "--quit", "--path", cmd.ProjectDir, "--export-debug", platformName, targetPath)
}

// ExportWeb exports the project for the standard web runtime.
func (cmd *CmdTool) ExportWeb() error {
	if err := cmd.exportWebCommon(webNormalMode); err != nil {
		return err
	}
	return cmd.copyWebOverlay(webNormalMode, cmd.WebDir)
}

// ExportWebWorker exports the project for the worker-based web runtime.
func (cmd *CmdTool) ExportWebWorker() error {
	if err := cmd.exportWebCommon(webWorkerMode); err != nil {
		return err
	}
	extDir := filepath.Join(cmd.WebDir, "__"+webWorkerMode)
	defer func() {
		_ = os.RemoveAll(extDir)
	}()
	if err := cmd.copyWebOverlay(webWorkerMode, extDir); err != nil {
		return err
	}

	insertCode, err := cmd.readWebWorkerBundle(extDir)
	if err != nil {
		return err
	}
	return cmd.patchWebWorkerEngine(insertCode)
}

// ExportMinigame exports the project for the minigame runtime.
func (cmd *CmdTool) ExportMinigame() error {
	if err := cmd.exportWebCommon(webMinigameMode); err != nil {
		return err
	}

	buildMode := *cmd.Args.Build
	paths, err := cmd.prepareMinigameWorkspace()
	if err != nil {
		return err
	}
	if err := cmd.prepareMinigameEngineAssets(paths, buildMode); err != nil {
		return err
	}
	if err := cmd.finalizeMinigameJS(paths, buildMode != "fast"); err != nil {
		return err
	}
	cmd.openWeChatDevTools(paths.workDir)
	return nil
}

// ExportMiniprogram exports the project for the miniprogram runtime.
func (cmd *CmdTool) ExportMiniprogram() error {
	if err := cmd.exportWebCommon(webMiniprogramMode); err != nil {
		return err
	}
	if err := cmd.copyWebOverlay(webMiniprogramMode, cmd.WebDir); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(cmd.WebDir, "lab")); err != nil {
		return fmt.Errorf("failed to remove web lab assets: %w", err)
	}
	return nil
}

func (cmd *CmdTool) copyWebOverlay(mode, destination string) error {
	source := webPlatformTemplate + mode
	if err := util.CopyDir(cmd.PlatformFS, source, destination, true); err != nil {
		return fmt.Errorf("failed to copy web overlay %s: %w", source, err)
	}
	return nil
}

func (cmd *CmdTool) exportWebCommon(mode string) error {
	if err := cmd.Clear(); err != nil {
		return err
	}
	templateDir := filepath.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+"_web"+mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web dir file not found: " + templateDir)
	}

	dstPath := filepath.Join(cmd.ProjectDir, ".builds", "web")
	if err := os.MkdirAll(dstPath, 0o755); err != nil {
		return fmt.Errorf("failed to create web export directory: %w", err)
	}
	if err := util.CopyDir2(templateDir, dstPath); err != nil {
		return fmt.Errorf("failed to copy web runtime from %s: %w", templateDir, err)
	}

	logInfof("Exporting web assets to %s", dstPath)
	if err := util.CopyDir(cmd.ProjectFS, "template/project", cmd.ProjectDir, true); err != nil {
		return fmt.Errorf("failed to copy web project template: %w", err)
	}

	if err := os.Rename(filepath.Join(dstPath, "godot.editor.html"), filepath.Join(dstPath, "index.html")); err != nil {
		return fmt.Errorf("failed to rename web entry page: %w", err)
	}

	ispxWebDir, err := cmd.getIspxWebDir()
	if err != nil {
		return err
	}
	if err := util.CopyDir2(ispxWebDir, cmd.WebDir); err != nil {
		return fmt.Errorf("failed to copy interpreter web runtime from %s: %w", ispxWebDir, err)
	}

	if err := cmd.copyWebOverlay("", cmd.WebDir); err != nil {
		return err
	}

	output, err := util.OutputCommand(util.CommandOptions{Dir: cmd.TargetDir}, "go", "env", "GOROOT")
	if err != nil {
		return fmt.Errorf("failed to get GOROOT: %w", err)
	}
	goroot := strings.TrimSpace(string(output))
	wasmExecPath := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	if err := util.CopyFile(wasmExecPath, filepath.Join(cmd.WebDir, "go.wasm.exec.js")); err != nil {
		return fmt.Errorf("failed to copy wasm_exec.js: %w", err)
	}
	if err := pack.PackProject(cmd.TargetDir, filepath.Join(cmd.WebDir, "game.zip")); err != nil {
		return err
	}
	return cmd.writeWebLogicAssets(mode)
}

func (cmd *CmdTool) writeWebLogicAssets(mode string) error {
	wasmDstPath := filepath.Join(cmd.WebDir, "ispx.wasm")
	wasmPath := filepath.Join(cmd.GoBinPath, "ispx.wasm")
	wasmBrPath := wasmPath + ".br"
	if err := util.CopyFile(wasmPath, wasmDstPath); err != nil {
		return fmt.Errorf("failed to copy interpreter wasm from %s: %w", wasmPath, err)
	}
	// Minigame exports either use raw wasm or recompress it during preparation.
	if mode != webMinigameMode && util.IsFileExist(wasmBrPath) {
		if err := util.CopyFile(wasmBrPath, wasmDstPath+".br"); err != nil {
			return fmt.Errorf("failed to copy compressed ispx wasm from %s: %w", wasmBrPath, err)
		}
	} else if err := os.Remove(wasmDstPath + ".br"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale compressed wasm at %s: %w", wasmDstPath+".br", err)
	}
	return nil
}

func (cmd *CmdTool) readWebWorkerBundle(extDir string) (string, error) {
	if err := os.Rename(filepath.Join(cmd.WebDir, "go.wasm.exec.js"), filepath.Join(extDir, "go.wasm.exec.js")); err != nil {
		return "", fmt.Errorf("failed to move go.wasm.exec.js: %w", err)
	}

	entries, err := os.ReadDir(extDir)
	if err != nil {
		return "", fmt.Errorf("failed to read worker extension directory: %w", err)
	}

	var bundle strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}

		jsFile := filepath.Join(extDir, entry.Name())
		content, err := os.ReadFile(jsFile)
		if err != nil {
			return "", err
		}
		bundle.WriteString("\n\n\n")
		bundle.Write(content)
	}
	return bundle.String(), nil
}

func (cmd *CmdTool) patchWebWorkerEngine(insertCode string) error {
	enginePath := filepath.Join(cmd.WebDir, "engine.js")
	engineBytes, err := os.ReadFile(enginePath)
	if err != nil {
		return fmt.Errorf("failed to read engine.js: %w", err)
	}
	engineStr := string(engineBytes)

	keyStr := "{if(initializedJS){checkMailbox()}}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js missing worker hook anchor: %s", keyStr)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameRunnerMessageId) {handleGameRunnerMessage(e.data);}")

	keyStr = ";throw ex}}self.onmessage=handleMessage}"
	if !strings.Contains(engineStr, keyStr) {
		return fmt.Errorf("engine.js missing worker bundle anchor: %s", keyStr)
	}
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+insertCode)
	if err := os.WriteFile(enginePath, []byte(engineStr), 0o644); err != nil {
		return fmt.Errorf("failed to write engine.js: %w", err)
	}
	return nil
}

func (cmd *CmdTool) prepareMinigameWorkspace() (minigamePaths, error) {
	paths := minigamePaths{
		workDir:   cmd.WebDir,
		engineDir: filepath.Join(cmd.WebDir, "engine"),
		jsDir:     filepath.Join(cmd.WebDir, "js"),
		rawWebDir: cmd.WebDir + "_bck",
	}
	if err := os.Rename(cmd.WebDir, paths.rawWebDir); err != nil {
		return minigamePaths{}, fmt.Errorf("failed to backup web directory: %w", err)
	}

	if err := cmd.copyWebOverlay(webMinigameMode, cmd.WebDir); err != nil {
		return minigamePaths{}, err
	}
	for _, dir := range []string{paths.engineDir, paths.jsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return minigamePaths{}, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	return paths, nil
}

func (cmd *CmdTool) prepareMinigameEngineAssets(paths minigamePaths, buildMode string) error {
	wasmNames := []string{"engine.wasm", "ispx.wasm"}
	if buildMode == "fast" {
		for _, name := range wasmNames {
			source := filepath.Join(paths.rawWebDir, name)
			if err := os.Rename(source, filepath.Join(paths.engineDir, name)); err != nil {
				return fmt.Errorf("failed to move %s: %w", source, err)
			}
		}
	} else {
		if _, err := exec.LookPath("brotli"); err != nil {
			return fmt.Errorf("error: brotli is not installed")
		}

		for _, name := range wasmNames {
			source := filepath.Join(paths.rawWebDir, name)
			logInfof("Compressing %s", source)
			if err := exec.Command("brotli", "-f", "-q", "11", source).Run(); err != nil {
				return fmt.Errorf("failed to compress %s: %w", source, err)
			}
		}
		if err := cmd.moveFilesByPattern(paths.rawWebDir, paths.engineDir, "*.br"); err != nil {
			return fmt.Errorf("failed to move br files: %w", err)
		}
	}

	if err := cmd.moveFilesByPattern(paths.rawWebDir, paths.engineDir, "*.zip"); err != nil {
		return fmt.Errorf("failed to move zip files: %w", err)
	}
	return nil
}

func (cmd *CmdTool) finalizeMinigameJS(paths minigamePaths, isCompressed bool) error {
	if err := cmd.moveFilesByPattern(paths.rawWebDir, paths.jsDir, "*.js"); err != nil {
		return fmt.Errorf("failed to move js files: %w", err)
	}
	if err := cmd.mergeJSFiles(paths.jsDir, isCompressed); err != nil {
		return fmt.Errorf("failed to merge JS files: %w", err)
	}
	if err := os.RemoveAll(paths.rawWebDir); err != nil {
		return fmt.Errorf("failed to remove raw web directory: %w", err)
	}
	return nil
}

func (cmd *CmdTool) openWeChatDevTools(workDir string) {
	if wechatDevTools := os.Getenv("WECHAT_DEV_TOOLS"); wechatDevTools != "" {
		projectDir := cmd.TargetDir
		if !filepath.IsAbs(wechatDevTools) {
			wechatDevTools = filepath.Join(projectDir, wechatDevTools)
		}
		logInfof("Opening WeChat DevTools for %s", workDir)
		execCmd := exec.Command(filepath.Join(wechatDevTools, "cli"), "open", "--project", workDir, "-y")
		execCmd.Dir = projectDir
		execCmd.Run()
		return
	}
	logWarnf("WECHAT_DEV_TOOLS is not set; open the project manually: %s", workDir)
}

// moveFilesByPattern moves matching files.
func (cmd *CmdTool) moveFilesByPattern(srcDir, dstDir, pattern string) error {
	if _, err := filepath.Match(pattern, ""); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		matched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		if err := os.Rename(filepath.Join(srcDir, entry.Name()), filepath.Join(dstDir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

// mergeJSFiles merges JavaScript files.
func (cmd *CmdTool) mergeJSFiles(jsDir string, isCompressed bool) (err error) {
	jsFiles := []string{"header.js", "engine.js", "go.wasm.exec.js", "worker.message.manager.js", "game.js"}
	outputFile := filepath.Join(jsDir, "engine_new.js")

	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer func() {
		if output == nil {
			return
		}
		err = errors.Join(err, output.Close())
	}()

	if _, err := fmt.Fprintf(output, "globalThis['FFI'] = null;\nconst isWasmCompressed = %t;\n\n", isCompressed); err != nil {
		return fmt.Errorf("write merged JS output %s: %w", outputFile, err)
	}

	for _, jsFile := range jsFiles {
		filePath := filepath.Join(jsDir, jsFile)
		file, err := os.Open(filePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(output, file)
		if err := errors.Join(copyErr, file.Close()); err != nil {
			return err
		}

		if err := os.Remove(filePath); err != nil {
			return err
		}
	}

	if err = output.Close(); err != nil {
		output = nil
		return fmt.Errorf("closing merged JS output: %w", err)
	}
	output = nil
	return os.Rename(outputFile, filepath.Join(jsDir, "engine.js"))
}
