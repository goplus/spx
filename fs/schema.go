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

package fs

import (
	"errors"
	"io"
	iofs "io/fs"
	"strings"
)

// -------------------------------------------------------------------------------------

type Dir interface {
	Open(file string) (io.ReadCloser, error)
	Close() error
}

// DirEntry describes a direct child returned by ReadDirer.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ReadDirer is an optional capability for resource directories that can list
// direct children. Callers must continue to work with Dir implementations that
// only support opening known paths.
type ReadDirer interface {
	ReadDir(dir string) ([]DirEntry, error)
}

// DirEntriesFromFS converts standard library directory entries while
// preserving their order.
func DirEntriesFromFS(entries []iofs.DirEntry) []DirEntry {
	result := make([]DirEntry, len(entries))
	for i, entry := range entries {
		result[i] = DirEntry{Name: entry.Name(), IsDir: entry.IsDir()}
	}
	return result
}

type GdDir interface {
	GetPath() string
}

func SplitSchema(path string) (schema, file string) {
	idx := strings.IndexAny(path, ":/\\ ")
	if idx < 0 || path[idx] != ':' {
		return "", path
	}
	schema, file = path[:idx], path[idx+1:]
	file = strings.TrimPrefix(file, "//")
	return
}

// -------------------------------------------------------------------------------------

type OpenFunc = func(file string) (Dir, error)

var (
	openSchemas = map[string]OpenFunc{}
)

func RegisterSchema(schema string, open OpenFunc) {
	openSchemas[schema] = open
}

func Open(path string) (Dir, error) {
	schema, file := SplitSchema(path)
	if open, ok := openSchemas[schema]; ok {
		return open(file)
	}
	return nil, errors.New("fs.Open: unsupported schema - " + schema)
}

// -------------------------------------------------------------------------------------
