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

package runtimevariant

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSpineBin(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	enabled := []string{"SPX_SPINE=1", "GOPATH=" + root}
	if _, _, err := LocalSpineBin(enabled, bin); err == nil {
		t.Fatal("accepted unmarked default runtime")
	}
	if err := os.WriteFile(filepath.Join(bin, SpineMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, selected, err := LocalSpineBin(enabled, "")
	if err != nil || !selected || got != bin {
		t.Fatalf("selected = %q, %v, %v", got, selected, err)
	}
	for _, env := range [][]string{{"GOPATH=" + root}, {"SPX_SPINE=0", "GOPATH=" + root}, {"SPX_SPINE=bad"}, {"SPX_SPINE=1", "SPX_SPINE=0"}} {
		if _, _, err := LocalSpineBin(env, bin); err == nil {
			t.Fatalf("accepted unsafe selection %v", env)
		}
	}
	if _, _, err := LocalSpineBin(enabled, t.TempDir()); err == nil {
		t.Fatal("accepted unrelated binary directory")
	}
	if _, selected, err := LocalSpineBin(nil, t.TempDir()); selected || err != nil {
		t.Fatalf("default behavior changed: %v, %v", selected, err)
	}
}
