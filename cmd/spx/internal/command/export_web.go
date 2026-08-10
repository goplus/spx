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
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/cmd/spx/internal/pack"
	"github.com/goplus/spx/v3/cmd/spx/internal/util"
)

const (
	webNormalMode      = "normal"
	webWorkerMode      = "worker"
	webMinigameMode    = "minigame"
	webMiniprogramMode = "miniprogram"
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
	util.CopyDir(cmd.PlatformFS, "template/platform/web"+webNormalMode, cmd.WebDir, true)
	return nil
}

// ExportWebWorker exports the project for the worker-based web runtime.
func (cmd *CmdTool) ExportWebWorker() error {
	if err := cmd.exportWebCommon(webWorkerMode); err != nil {
		return err
	}
	extDir := filepath.Join(cmd.WebDir, "__"+webWorkerMode)
	util.CopyDir(cmd.PlatformFS, "template/platform/web"+webWorkerMode, extDir, true)
	defer func() {
		_ = os.RemoveAll(extDir)
	}()

	insertCode, err := cmd.readWebWorkerBundle(extDir)
	if err != nil {
		return err
	}
	if err := cmd.patchWebWorkerEngine(insertCode); err != nil {
		return err
	}
	return nil
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
	util.CopyDir(cmd.PlatformFS, "template/platform/web"+webMiniprogramMode, cmd.WebDir, true)
	if err := os.RemoveAll(filepath.Join(cmd.WebDir, "lab")); err != nil {
		return fmt.Errorf("failed to remove web lab assets: %w", err)
	}
	return nil
}

func (cmd *CmdTool) prepareMinigameWorkspace() (minigamePaths, error) {
	backupDir := cmd.WebDir + "_bck"
	if err := os.Rename(cmd.WebDir, backupDir); err != nil {
		return minigamePaths{}, fmt.Errorf("failed to backup web directory: %w", err)
	}
	if err := os.MkdirAll(cmd.WebDir, 0o755); err != nil {
		return minigamePaths{}, fmt.Errorf("failed to create minigame directory: %w", err)
	}

	paths := minigamePaths{
		workDir:   cmd.WebDir,
		engineDir: filepath.Join(cmd.WebDir, "engine"),
		jsDir:     filepath.Join(cmd.WebDir, "js"),
		rawWebDir: filepath.Join(cmd.WebDir, "rawWeb"),
	}
	if err := os.Rename(backupDir, paths.rawWebDir); err != nil {
		return minigamePaths{}, fmt.Errorf("failed to move raw web assets: %w", err)
	}

	util.CopyDir(cmd.PlatformFS, "template/platform/web"+webMinigameMode, cmd.WebDir, true)
	for _, dir := range []string{paths.engineDir, paths.jsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return minigamePaths{}, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	return paths, nil
}

func (cmd *CmdTool) prepareMinigameEngineAssets(paths minigamePaths, buildMode string) error {
	godotEditorWasm := filepath.Join(paths.rawWebDir, "engine.wasm")
	ispxWasm := filepath.Join(paths.rawWebDir, "ispx.wasm")

	if buildMode == "fast" {
		if err := cmd.moveFile(godotEditorWasm, filepath.Join(paths.engineDir, "engine.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", godotEditorWasm, err)
		}
		if err := cmd.moveFile(ispxWasm, filepath.Join(paths.engineDir, "ispx.wasm")); err != nil {
			return fmt.Errorf("failed to move %s: %w", ispxWasm, err)
		}
	} else {
		if _, err := exec.LookPath("brotli"); err != nil {
			return fmt.Errorf("error: brotli is not installed")
		}

		logInfof("Compressing %s", godotEditorWasm)
		if err := cmd.compressBrotli(godotEditorWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", godotEditorWasm, err)
		}
		logInfof("Compressing %s", ispxWasm)
		if err := cmd.compressBrotli(ispxWasm); err != nil {
			return fmt.Errorf("failed to compress %s: %w", ispxWasm, err)
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
		logInfof("Opening WeChat DevTools for %s", workDir)
		execCmd := exec.Command(filepath.Join(wechatDevTools, "cli"), "open", "--project", workDir, "-y")
		execCmd.Run()
		return
	}
	logWarnf("WECHAT_DEV_TOOLS is not set; open the project manually: %s", workDir)
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
	engineStr = strings.ReplaceAll(engineStr, keyStr, keyStr+"else if(e.data._gameAppMessageId) {handleGameAppMessage(e.data);}")

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

func (cmd *CmdTool) exportWebCommon(mode string) error {
	if err := cmd.Clear(); err != nil {
		return err
	}
	templateDir := filepath.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+"_web"+mode)
	if !util.IsFileExist(templateDir) {
		return errors.New("web dir file not found: " + templateDir)
	}

	dstPath := filepath.Join(cmd.ProjectDir, ".builds", "web")
	os.MkdirAll(dstPath, 0o755)
	util.CopyDir2(templateDir, dstPath)

	logInfof("Exporting web assets to %s", dstPath)
	util.CopyDir(cmd.ProjectFS, "template/project", cmd.ProjectDir, true)

	os.Rename(filepath.Join(dstPath, "godot.editor.html"), filepath.Join(dstPath, "index.html"))

	ispxWebDir, err := cmd.getIspxWebDir()
	if err != nil {
		return err
	}
	util.CopyDir2(ispxWebDir, cmd.WebDir)

	util.CopyDir(cmd.PlatformFS, "template/platform/web", cmd.WebDir, true)

	output, err := util.OutputCommand(util.CommandOptions{}, "go", "env", "GOROOT")
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
	return cmd.writeWebLogicAssets()
}

func (cmd *CmdTool) writeWebLogicAssets() error {
	wasmDstPath := filepath.Join(cmd.WebDir, "ispx.wasm")
	wasmPath := filepath.Join(cmd.GoBinPath, "ispx.wasm")
	wasmBrPath := wasmPath + ".br"
	if !util.IsFileExist(wasmBrPath) {
		wasmBrPath = ""
	}
	if err := util.CopyFile(wasmPath, wasmDstPath); err != nil {
		return fmt.Errorf("failed to copy interpreter wasm from %s: %w", wasmPath, err)
	}
	if wasmBrPath != "" {
		if err := util.CopyFile(wasmBrPath, wasmDstPath+".br"); err != nil {
			return fmt.Errorf("failed to copy compressed ispx wasm from %s: %w", wasmBrPath, err)
		}
	} else if err := os.Remove(wasmDstPath + ".br"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale compressed wasm at %s: %w", wasmDstPath+".br", err)
	}
	return nil
}

// compressBrotli runs brotli on a file.
func (cmd *CmdTool) compressBrotli(filePath string) error {
	execCmd := exec.Command("brotli", "-f", "-q", "11", filePath)
	return execCmd.Run()
}

// moveFile moves one file.
func (cmd *CmdTool) moveFile(srcFile, dstFile string) error {
	return os.Rename(srcFile, dstFile)
}

// moveFilesByPattern moves matching files.
func (cmd *CmdTool) moveFilesByPattern(srcDir, dstDir, pattern string) error {
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
		if closeErr := output.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("closing merged JS output: %w", closeErr)
		}
	}()

	writer := bufio.NewWriter(output)

	compressionFlag := fmt.Sprintf("globalThis['FFI'] = null;\nconst isWasmCompressed = %t;\n\n", isCompressed)
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
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		if err := os.Remove(filePath); err != nil {
			return err
		}
	}

	if err = writer.Flush(); err != nil {
		return fmt.Errorf("flushing merged JS output: %w", err)
	}
	if err = output.Close(); err != nil {
		output = nil
		return fmt.Errorf("closing merged JS output: %w", err)
	}
	output = nil
	return os.Rename(outputFile, filepath.Join(jsDir, "engine.js"))
}
