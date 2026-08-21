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

package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func validatePackDestination(name string) error {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect pack destination %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pack destination %q must not be a symlink", name)
	}
	if info.IsDir() {
		return fmt.Errorf("pack destination %q must be a file", name)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pack destination %q must be a regular file", name)
	}
	return nil
}

func createPackOutput(name string) (string, *os.File, error) {
	dir := filepath.Dir(name)
	prefix := "." + filepath.Base(name) + ".tmp-"
	file, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return "", nil, fmt.Errorf("create temporary pack output in %q: %w", dir, err)
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", nil, fmt.Errorf("set pack output mode in %q: %w", dir, err)
	}
	return file.Name(), file, nil
}

func closePackOutput(writer interface{ Close() error }, file *os.File, err error) error {
	if closeErr := writer.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	return err
}

func publishPackOutput(tempName, destination string) error {
	if err := os.Rename(tempName, destination); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return fmt.Errorf("publish pack output %q: %w", destination, err)
	}

	info, err := os.Lstat(destination)
	if err != nil {
		return fmt.Errorf("publish pack output %q: %w", destination, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("pack destination %q changed while publishing", destination)
	}
	if err := os.Remove(destination); err != nil {
		return fmt.Errorf("replace pack output %q: %w", destination, err)
	}
	if err := os.Rename(tempName, destination); err != nil {
		return fmt.Errorf("publish pack output %q: %w", destination, err)
	}
	return nil
}
