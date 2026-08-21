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

package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const maxPackJSONSize = 16 << 20

func readJSONFile(name string, value any) error {
	before, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular non-symlink file", name)
	}
	if before.Size() > maxPackJSONSize {
		return fmt.Errorf("%s exceeds the JSON size limit", name)
	}

	file, err := os.Open(name)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	current, lstatErr := os.Lstat(name)
	if statErr != nil || lstatErr != nil || current.Mode()&os.ModeSymlink != 0 ||
		!opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, current) {
		_ = file.Close()
		return fmt.Errorf("%s changed while it was opened", name)
	}

	data, readErr := io.ReadAll(io.LimitReader(file, maxPackJSONSize+1))
	afterOpened, statErr := file.Stat()
	afterPath, lstatErr := os.Lstat(name)
	closeErr := file.Close()
	if readErr != nil {
		return readErr
	}
	if statErr != nil || lstatErr != nil || afterPath.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
		!sameStableFileMetadata(opened, afterOpened) || int64(len(data)) != opened.Size() {
		return fmt.Errorf("%s changed while it was read", name)
	}
	if len(data) > maxPackJSONSize {
		return fmt.Errorf("%s exceeds the JSON size limit", name)
	}
	if closeErr != nil {
		return closeErr
	}
	return json.Unmarshal(data, value)
}
