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
	archivezip "archive/zip"
	"bytes"
	"io/fs"
	"reflect"
	"testing"

	spxfs "github.com/goplus/spx/v4/fs"
)

func TestReadDirReturnsDirectChildren(t *testing.T) {
	var contents bytes.Buffer
	writer := archivezip.NewWriter(&contents)
	for _, name := range []string{
		"other.txt",
		"fonts/Scratch/index.json",
		"fonts/Basic/index.json",
		"fonts/Basic/basic.ttf",
		"fonts/Empty/",
		"fonts/",
		"index.json",
		"fontscript/unrelated",
		"mode-dir",
	} {
		header := &archivezip.FileHeader{Name: name, Method: archivezip.Store}
		if name == "mode-dir" {
			header.SetMode(fs.ModeDir | 0o755)
		}
		if _, err := writer.CreateHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := archivezip.NewReader(bytes.NewReader(contents.Bytes()), int64(contents.Len()))
	if err != nil {
		t.Fatal(err)
	}
	dir := &FS{Reader: reader}
	root := []spxfs.DirEntry{
		{Name: "fonts", IsDir: true},
		{Name: "fontscript", IsDir: true},
		{Name: "index.json"},
		{Name: "mode-dir", IsDir: true},
		{Name: "other.txt"},
	}
	fonts := []spxfs.DirEntry{{Name: "Basic", IsDir: true}, {Name: "Empty", IsDir: true}, {Name: "Scratch", IsDir: true}}
	for _, tt := range []struct {
		name string
		want []spxfs.DirEntry
	}{
		{name: "", want: root},
		{name: ".", want: root},
		{name: "/", want: root},
		{name: "./fonts/..", want: root},
		{name: "fonts", want: fonts},
		{name: "/fonts//", want: fonts},
		{name: "fonts/./Basic/", want: []spxfs.DirEntry{{Name: "basic.ttf"}, {Name: "index.json"}}},
		{name: "fonts/Basic/../Scratch", want: []spxfs.DirEntry{{Name: "index.json"}}},
		{name: "fonts/Empty", want: []spxfs.DirEntry{}},
		{name: "mode-dir", want: []spxfs.DirEntry{}},
		{name: "index.json", want: []spxfs.DirEntry{}},
		{name: "absent", want: []spxfs.DirEntry{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := dir.ReadDir(tt.name)
			if err != nil || !reflect.DeepEqual(entries, tt.want) {
				t.Fatalf("ReadDir(%q) = %#v, %v, want %#v, nil", tt.name, entries, err, tt.want)
			}
		})
	}
}

func TestReadDirEmptyArchive(t *testing.T) {
	dir := &FS{Reader: &archivezip.Reader{}}
	entries, err := dir.ReadDir(".")
	if err != nil || len(entries) != 0 {
		t.Fatalf("ReadDir = %#v, %v, want no entries or error", entries, err)
	}
}
