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

package project

import (
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fontBoundaryDir struct {
	localDir
	opened []string
}

func (d *fontBoundaryDir) Open(name string) (io.ReadCloser, error) {
	d.opened = append(d.opened, name)
	return d.localDir.Open(name)
}

func TestPackedFontNamesRejectUnsafePathsBeforeOpeningFamily(t *testing.T) {
	for _, name := range []string{"", ".", "..", "nested/Font", "../Sibling", "../../outside", `..\..\outside`, "Font\x00Other"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			raw, err := json.Marshal(map[string]any{"fonts": map[string]any{name: nil}})
			if err != nil {
				t.Fatal(err)
			}
			writeProjectFile(t, dir, packedIndexJSON, string(raw))
			resource := &fontBoundaryDir{localDir: localDir{base: dir}}
			_, err = OpenBuilderResources(resource, nil)
			if err == nil || !strings.Contains(err.Error(), "font family name") {
				t.Fatalf("error = %v, want invalid family name", err)
			}
			if want := []string{packedIndexJSON, "index.json"}; !reflect.DeepEqual(resource.opened, want) {
				t.Fatalf("opened = %q, want only project configs %q", resource.opened, want)
			}
		})
	}
}

func TestPackedFontNameCannotLoadFilesOutsideProject(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	writeProjectFile(t, projectDir, packedIndexJSON, `{"fonts":{"../../outside":{"faces":[{"path":"ignored.ttf"}]}}}`)
	writeProjectFile(t, root, "outside/index.json", `{"faces":[{"path":"outside.ttf"}]}`)
	writeProjectFile(t, root, "outside/outside.ttf", "external-font")
	opened, err := OpenBuilderResources(projectDir, nil)
	if err == nil {
		defer opened.FS.Close()
		t.Fatalf("accepted family traversal; loaded external config/font: %#v", opened.Fonts.Families)
	}
	if !strings.Contains(err.Error(), "font family name") {
		t.Fatalf("error = %v, want invalid family name", err)
	}
}
