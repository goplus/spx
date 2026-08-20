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

package interpruntime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/goplus/spx/v3/internal/scaffold"
)

const (
	runtimeExtensionFile = "runtime.gdextension"
	projectExtensionFile = "gdspx.gdextension"
	extensionListFile    = "extension_list.cfg"
	bridgeDirectory      = "lib"
)

var scaffoldTempSequence atomic.Uint64

// SessionConfig describes the files needed to load the interpreter bridge in
// an Engine session. BridgePath is copied by base name into Roots.SessionDir.
type SessionConfig struct {
	Roots      Roots
	BridgePath string
}

// PrepareSession creates the session scaffold without changing global cwd or
// environment.
func PrepareSession(cfg SessionConfig) error {
	if err := validateAbsoluteCleanPath("ProjectDir", cfg.Roots.ProjectDir); err != nil {
		return err
	}
	if err := validateAbsoluteCleanPath("AssetDir", cfg.Roots.AssetDir); err != nil {
		return err
	}
	if err := validateAbsoluteCleanPath("SessionDir", cfg.Roots.SessionDir); err != nil {
		return err
	}
	for _, item := range []struct {
		name          string
		path          string
		rejectSymlink bool
	}{
		{name: "ProjectDir", path: cfg.Roots.ProjectDir},
		{name: "AssetDir", path: cfg.Roots.AssetDir, rejectSymlink: true},
	} {
		if err := validateDirectory(item.name, item.path, item.rejectSymlink); err != nil {
			return err
		}
	}
	bridge, bridgeInfo, err := openPinnedRegularFile("BridgePath", cfg.BridgePath)
	if err != nil {
		return err
	}
	defer bridge.Close()

	if err := ensureDirectoryNoSymlink("SessionDir", cfg.Roots.SessionDir, 0o700); err != nil {
		return err
	}
	if err := cfg.Roots.Validate(); err != nil {
		return err
	}

	session, err := openPinnedRoot("SessionDir", cfg.Roots.SessionDir)
	if err != nil {
		return err
	}
	defer session.Close()

	bridgeName := filepath.Base(cfg.BridgePath)
	if bridgeName == "." || bridgeName == string(filepath.Separator) || bridgeName == runtimeExtensionFile || bridgeName == ".godot" {
		return fmt.Errorf("interpruntime: unsafe bridge base name %q", bridgeName)
	}
	if relative, relErr := filepath.Rel(cfg.Roots.SessionDir, cfg.BridgePath); relErr == nil &&
		(relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))) {
		return fmt.Errorf("interpruntime: BridgePath must be outside SessionDir")
	}
	bridgeRoot, err := ensurePinnedSubdirectory(session, bridgeDirectory, 0o700)
	if err != nil {
		return err
	}
	defer bridgeRoot.Close()
	if err := replaceRootFile(bridgeRoot, bridgeName, bridge, 0o700); err != nil {
		return fmt.Errorf("interpruntime: copy bridge %q: %w", cfg.BridgePath, err)
	}
	if copied, err := bridge.Seek(0, io.SeekCurrent); err != nil {
		return fmt.Errorf("interpruntime: inspect copied BridgePath %q: %w", cfg.BridgePath, err)
	} else if copied != bridgeInfo.Size() {
		return fmt.Errorf("interpruntime: BridgePath %q changed size while it was copied", cfg.BridgePath)
	}
	if after, err := bridge.Stat(); err != nil {
		return fmt.Errorf("interpruntime: re-stat BridgePath %q: %w", cfg.BridgePath, err)
	} else if !os.SameFile(bridgeInfo, after) || bridgeInfo.Mode() != after.Mode() || bridgeInfo.Size() != after.Size() || !bridgeInfo.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("interpruntime: BridgePath %q changed while it was copied", cfg.BridgePath)
	}
	if err := replaceRootFile(session, runtimeExtensionFile, bytes.NewReader([]byte(scaffold.SessionRuntimeGDExtension())), 0o600); err != nil {
		return fmt.Errorf("interpruntime: write %s: %w", runtimeExtensionFile, err)
	}
	if err := replaceRootFile(session, projectExtensionFile, bytes.NewReader([]byte(scaffold.ProjectGDExtension())), 0o600); err != nil {
		return fmt.Errorf("interpruntime: write %s: %w", projectExtensionFile, err)
	}
	if err := verifyPinnedSubdirectory(session, bridgeDirectory, bridgeRoot); err != nil {
		return err
	}
	projectData, err := ensurePinnedSubdirectory(session, ".godot", 0o700)
	if err != nil {
		return err
	}
	defer projectData.Close()
	if err := replaceRootFile(projectData, extensionListFile, bytes.NewReader([]byte(scaffold.SessionExtensionList())), 0o600); err != nil {
		return fmt.Errorf("interpruntime: write %s: %w", extensionListFile, err)
	}
	if err := verifyPinnedSubdirectory(session, ".godot", projectData); err != nil {
		return err
	}
	if err := verifyPinnedRootPath("SessionDir", cfg.Roots.SessionDir, session); err != nil {
		return err
	}
	return nil
}

func ensureDirectoryNoSymlink(name, path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, mode); err != nil {
			return fmt.Errorf("interpruntime: create %s %q: %w", name, path, err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("interpruntime: stat %s %q: %w", name, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("interpruntime: %s %q must not be a symlink", name, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("interpruntime: %s %q is not a directory", name, path)
	}
	return nil
}

func openPinnedRoot(name, path string) (*os.Root, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("interpruntime: lstat %s %q: %w", name, path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("interpruntime: %s %q is not a real directory", name, path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("interpruntime: open %s root %q: %w", name, path, err)
	}
	after, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("interpruntime: stat opened %s root %q: %w", name, path, err)
	}
	if !after.IsDir() || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("interpruntime: %s %q changed while it was opened", name, path)
	}
	if err := verifyPinnedRootPath(name, path, root); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func verifyPinnedRootPath(name, path string, root *os.Root) error {
	opened, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("interpruntime: stat pinned %s %q: %w", name, path, err)
	}
	current, err := os.Lstat(path)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("interpruntime: %s %q changed after it was opened", name, path)
	}
	return nil
}

func ensurePinnedSubdirectory(parent *os.Root, name string, mode os.FileMode) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		if err := parent.Mkdir(name, mode); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("interpruntime: create session directory %q: %w", name, err)
		}
		before, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("interpruntime: inspect session directory %q: %w", name, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("interpruntime: session directory %q is not a real directory", name)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("interpruntime: open session directory %q: %w", name, err)
	}
	after, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	if !after.IsDir() || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("interpruntime: session directory %q changed while it was opened", name)
	}
	if err := verifyPinnedSubdirectory(parent, name, root); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func verifyPinnedSubdirectory(parent *os.Root, name string, root *os.Root) error {
	opened, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("interpruntime: stat pinned session directory %q: %w", name, err)
	}
	current, err := parent.Lstat(name)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("interpruntime: session directory %q changed after it was opened", name)
	}
	return nil
}

func replaceRootFile(root *os.Root, name string, input io.Reader, mode os.FileMode) (err error) {
	if name == "" || name == "." || filepath.Base(name) != name {
		return fmt.Errorf("interpruntime: invalid scaffold file name %q", name)
	}
	if info, statErr := root.Lstat(name); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("interpruntime: scaffold target %q is not a regular non-symlink file", name)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("interpruntime: inspect scaffold target %q: %w", name, statErr)
	}

	var tempName string
	var output *os.File
	for attempt := 0; attempt < 100; attempt++ {
		tempName = fmt.Sprintf(".%s.tmp-%d-%d", name, os.Getpid(), scaffoldTempSequence.Add(1))
		output, err = root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("interpruntime: create temporary scaffold %q: %w", name, err)
		}
	}
	if output == nil {
		return fmt.Errorf("interpruntime: could not allocate temporary scaffold for %q", name)
	}
	defer func() {
		_ = root.Remove(tempName)
	}()
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpruntime: write temporary scaffold %q: %w", name, err)
	}
	if err := output.Chmod(mode); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpruntime: chmod temporary scaffold %q: %w", name, err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpruntime: sync temporary scaffold %q: %w", name, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("interpruntime: close temporary scaffold %q: %w", name, err)
	}
	if runtime.GOOS == "windows" {
		// Windows rename does not consistently replace an existing regular file.
		// Root.Remove removes the directory entry itself and never follows it.
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("interpruntime: replace existing scaffold %q: %w", name, err)
		}
	}
	if err := root.Rename(tempName, name); err != nil {
		return fmt.Errorf("interpruntime: publish scaffold %q: %w", name, err)
	}
	return nil
}

// CommandConfig describes a prepared Engine child. Env is the complete base
// environment, rather than an overlay; root variables are replaced before the
// command is returned.
type CommandConfig struct {
	Roots      Roots
	Executable string
	Args       []string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	PathPolicy PathPolicy
}

// PathPolicy controls how pre-existing Engine --path options are handled.
// RejectPath is the driver-safe default. ReplacePath exists only for the
// legacy spx adapter, whose own --path flag selects ProjectDir.
type PathPolicy uint8

const (
	RejectPath PathPolicy = iota
	ReplacePath
)

// PrepareCommand validates cfg and returns an Engine command rooted at the
// explicit session directory. It has no process-wide side effects.
//
// Executable validation is an early fail-fast check: os/exec opens the path
// again in Start and offers no portable descriptor-bound exec operation.
// Security-sensitive callers must therefore source Executable from a verified,
// private runtime bundle whose directory cannot be replaced by an untrusted
// process; this function does not turn an ambient writable path into one.
func PrepareCommand(ctx context.Context, cfg CommandConfig) (*exec.Cmd, error) {
	if ctx == nil {
		return nil, fmt.Errorf("interpruntime: nil context")
	}
	if err := cfg.Roots.Validate(); err != nil {
		return nil, err
	}
	if err := validateRegularFile("Executable", cfg.Executable); err != nil {
		return nil, err
	}
	env, err := cfg.Roots.Environment(cfg.Env)
	if err != nil {
		return nil, err
	}

	args, err := engineArgs(cfg.Args, cfg.Roots.SessionDir, cfg.PathPolicy)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, cfg.Executable, args...)
	cmd.Dir = cfg.Roots.SessionDir
	cmd.Env = env
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	return cmd, nil
}

func validateRegularFile(name, path string) error {
	file, _, err := openPinnedRegularFile(name, path)
	if err != nil {
		return err
	}
	return file.Close()
}

func openPinnedRegularFile(name, path string) (*os.File, os.FileInfo, error) {
	if err := validateAbsoluteCleanPath(name, path); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("interpruntime: lstat %s %q: %w", name, path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("interpruntime: %s %q is not a regular non-symlink file", name, path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("interpruntime: open %s %q: %w", name, path, err)
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("interpruntime: stat opened %s %q: %w", name, path, err)
	}
	after, err := os.Lstat(path)
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		file.Close()
		return nil, nil, fmt.Errorf("interpruntime: %s %q changed while it was opened", name, path)
	}
	return file, opened, nil
}

// engineArgs makes SessionDir authoritative. RejectPath fails closed; the
// explicit ReplacePath compatibility policy removes both accepted spellings.
func engineArgs(input []string, sessionDir string, policy PathPolicy) ([]string, error) {
	if policy != RejectPath && policy != ReplacePath {
		return nil, fmt.Errorf("interpruntime: unknown Engine path policy %d", policy)
	}
	args := make([]string, 0, len(input)+3)
	for i := 0; i < len(input); i++ {
		if input[i] == "--path" || strings.HasPrefix(input[i], "--path=") {
			if policy == RejectPath {
				return nil, fmt.Errorf("interpruntime: Engine --path is reserved for SessionDir")
			}
			if input[i] == "--path" {
				if i+1 < len(input) {
					i++
				}
			}
			continue
		}
		args = append(args, input[i])
	}
	args = append(args, "--path", sessionDir, "--no-header")
	return args, nil
}
