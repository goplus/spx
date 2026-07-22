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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	spxfs "github.com/goplus/spx/v2/fs"
)

func TestReadDirReturnsDirectChildren(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "fonts.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := archivezip.NewWriter(file)
	for _, name := range []string{
		"fonts/Basic/index.json",
		"fonts/Basic/basic.ttf",
		"fonts/Scratch/index.json",
		"index.json",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	dir, err := Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	entries, err := dir.(spxfs.ReadDirer).ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	want := []spxfs.DirEntry{{Name: "Basic", IsDir: true}, {Name: "Scratch", IsDir: true}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
}
