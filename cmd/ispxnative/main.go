//go:build !js || !wasm

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

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	_ "unsafe"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/pkg/ispx"
)

var retainedProjectRoot struct {
	sync.Mutex
	root *os.Root
}

func main() {
	exitCode, err := run(os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ispxnative: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run(env []string) (int, error) {
	roots, err := interpruntime.RootsFromEnv(env)
	if err != nil {
		return 1, err
	}
	if err := projectpolicy.ValidateConfig(roots.ProjectDir); err != nil {
		return 1, err
	}
	if err := validateAssetIndex(roots.AssetDir); err != nil {
		return 1, err
	}
	if err := ispx.ConfigureFilesystemRoots(roots.ProjectDir, roots.AssetDir); err != nil {
		return 1, fmt.Errorf("configure filesystem roots: %w", err)
	}
	if err := ispx.Init(nil); err != nil {
		return 1, fmt.Errorf("initialize interpreter: %w", err)
	}

	if err := buildPinnedProject(roots.ProjectDir, ispx.BuildFS); err != nil {
		return 1, fmt.Errorf("build project: %w", err)
	}

	exitCode, err := ispx.Run()
	if err != nil {
		return exitCode, fmt.Errorf("interpreter exited with code %d: %w", exitCode, err)
	}
	return exitCode, nil
}

// buildPinnedProject retains the project root for the host-process lifetime on
// success. BuildFS borrows the filesystem because project configuration is
// loaded asynchronously after the interpreted entry point returns. Retaining
// the actual Root makes that lifetime explicit instead of relying on a closure
// and its finalizer to keep the descriptor open.
func buildPinnedProject(projectDir string, build func(fs.FS) error) error {
	projectRoot, err := openPinnedProjectRoot(projectDir)
	if err != nil {
		return err
	}
	if err := build(projectRoot.FS()); err != nil {
		projectRoot.Close()
		return err
	}
	retainProjectRoot(projectRoot)
	return nil
}

func retainProjectRoot(root *os.Root) {
	retainedProjectRoot.Lock()
	previous := retainedProjectRoot.root
	retainedProjectRoot.root = root
	retainedProjectRoot.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
}

func releaseProjectRoot() {
	retainedProjectRoot.Lock()
	root := retainedProjectRoot.root
	retainedProjectRoot.root = nil
	retainedProjectRoot.Unlock()
	if root != nil {
		_ = root.Close()
	}
}

func validateAssetIndex(assetDir string) error {
	found := false
	for _, name := range []string{"index_pack.json", "index.json"} {
		indexPath := filepath.Join(assetDir, name)
		exists, err := validateStableRegularFile(indexPath)
		if err != nil {
			return fmt.Errorf("validate asset index %q: %w", indexPath, err)
		}
		found = found || exists
	}
	if !found {
		return fmt.Errorf("validate asset index: neither %q nor %q exists", filepath.Join(assetDir, "index_pack.json"), filepath.Join(assetDir, "index.json"))
	}
	return nil
}

func validateStableRegularFile(name string) (bool, error) {
	before, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return false, fmt.Errorf("must be a regular non-symlink file")
	}
	file, err := os.Open(name)
	if err != nil {
		return false, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return false, err
	}
	read, readErr := io.Copy(io.Discard, file)
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	afterPath, lstatErr := os.Lstat(name)
	if readErr != nil {
		return false, readErr
	}
	if statErr != nil {
		return false, statErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	if lstatErr != nil || afterPath.Mode()&os.ModeSymlink != 0 || !opened.Mode().IsRegular() ||
		!os.SameFile(before, opened) || !os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
		read != opened.Size() || opened.Mode() != afterOpened.Mode() || opened.Size() != afterOpened.Size() || opened.ModTime() != afterOpened.ModTime() {
		return false, fmt.Errorf("changed while it was read")
	}
	return true, nil
}

func openPinnedProjectRoot(projectDir string) (*os.Root, error) {
	before, err := os.Lstat(projectDir)
	if err != nil {
		return nil, fmt.Errorf("inspect project root %q: %w", projectDir, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("project root %q must be a real directory", projectDir)
	}
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return nil, fmt.Errorf("open project root %q: %w", projectDir, err)
	}
	opened, statErr := root.Stat(".")
	after, lstatErr := os.Lstat(projectDir)
	if statErr != nil || lstatErr != nil || !opened.IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		root.Close()
		return nil, fmt.Errorf("project root %q changed while opening", projectDir)
	}
	return root, nil
}
