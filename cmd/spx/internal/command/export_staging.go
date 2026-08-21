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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/cmd/spx/internal/util"
)

func (cmd *CmdTool) prepareExport() error {
	if cmd.TargetAbsDir == "" {
		return fmt.Errorf("stage project-local resources: logical project directory is empty")
	}
	source := filepath.Join(cmd.TargetAbsDir, "assets")
	destination := filepath.Join(cmd.ProjectDir, "assets")
	if err := validateExportStage(source, destination); err != nil {
		return err
	}
	if err := util.CopyDir2(source, destination); err != nil {
		return fmt.Errorf("stage project-local resources from %s: %w", source, err)
	}
	return nil
}

func validateExportStage(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect project assets %q: %w", source, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("project assets %q must be a real directory", source)
	}

	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	if source == destination || pathWithin(destination, source) {
		return fmt.Errorf("stage project assets: destination %q is inside source %q", destination, source)
	}
	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("stage project assets: destination %q must not be a symlink", destination)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect staged project assets %q: %w", destination, err)
	}

	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("project asset %q must not be a symlink", name)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("project asset %q must be a regular file", name)
		}
		return nil
	})
}

func pathWithin(name, root string) bool {
	rel, err := filepath.Rel(root, name)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
