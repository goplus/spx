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

package zip

import (
	"path"
	"sort"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
)

func (zipf *FS) ReadDir(name string) ([]spxfs.DirEntry, error) {
	prefix := strings.Trim(path.Clean(name), "/")
	if prefix != "" && prefix != "." {
		prefix += "/"
	} else {
		prefix = ""
	}

	children := make(map[string]bool)
	for _, file := range zipf.File {
		fileName := strings.TrimPrefix(file.Name, "/")
		if !strings.HasPrefix(fileName, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(fileName, prefix)
		if remainder == "" {
			continue
		}
		parts := strings.SplitN(remainder, "/", 2)
		isDir := len(parts) == 2 || file.FileInfo().IsDir()
		children[parts[0]] = children[parts[0]] || isDir
	}

	names := make([]string, 0, len(children))
	for name := range children {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]spxfs.DirEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, spxfs.DirEntry{Name: name, IsDir: children[name]})
	}
	return entries, nil
}
