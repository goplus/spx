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

package projectpolicy

import (
	"fmt"
	"os"

	"github.com/goplus/spx/v4/internal/base/stablefile"
)

func readStableRegularFile(name string) ([]byte, os.FileInfo, error) {
	return readStableFile(name, stablefile.Reader{})
}

func readStableRegularRootFile(root *os.Root, name string) ([]byte, os.FileInfo, error) {
	return readStableFile(name, stablefile.Reader{Lstat: root.Lstat, Open: root.Open})
}

func readStableFile(name string, reader stablefile.Reader) ([]byte, os.FileInfo, error) {
	reader.MaxBytes = maxPortableConfigBytes
	reader.AllowMissing = true
	data, info, err := reader.Read(name)
	if err == stablefile.ErrTooLarge {
		return nil, nil, fmt.Errorf("exceeds %d bytes", maxPortableConfigBytes)
	}
	return data, info, err
}
