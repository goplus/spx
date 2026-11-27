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

package fileutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes a temporary file in dst's directory and closes it.
// If verify is provided, the closed file must pass verification before it replaces dst.
func WriteFileAtomic(dst string, write func(*os.File) error, verify func(string) error) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	path := file.Name()
	committed := false
	defer func() {
		if !committed {
			file.Close()
			os.Remove(path)
		}
	}()

	if err := write(file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if verify != nil {
		if err := verify(path); err != nil {
			return err
		}
	}
	if err := ReplaceFile(path, dst); err != nil {
		return err
	}
	committed = true
	return nil
}
