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

package spxproject

import (
	"reflect"
	"testing"

	"github.com/goplus/mod/modfile"
)

func TestNewProjectsAreIndependent(t *testing.T) {
	want := &modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v4", "math"},
	}
	base, customized := New(), New()
	if !reflect.DeepEqual(base, want) {
		t.Fatalf("base project = %+v, want %+v", base, want)
	}

	customized.Ext = ".custom"
	customized.Class = "CustomGame"
	customized.Works[0].Class = "CustomSprite"
	customized.Works[0].Embedded = !base.Works[0].Embedded
	customized.Works[0] = &modfile.Class{Class: "ReplacementSprite"}
	customized.PkgPaths[0] = "example.com/custom"
	customized.Import = []*modfile.Import{{Name: "ai", Path: "example.com/ai"}}

	if !reflect.DeepEqual(base, want) {
		t.Fatalf("customization changed an existing base project: %+v", base)
	}
	if fresh := New(); !reflect.DeepEqual(fresh, want) {
		t.Fatalf("customization changed subsequent base projects: %+v", fresh)
	}
}
