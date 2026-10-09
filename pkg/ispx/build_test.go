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

package ispx

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
)

func TestReadmeTutorialExamplesMatchSource(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	// Ignore presentation indentation, but preserve the source text on each line.
	normalize := func(source string) string {
		lines := strings.Split(strings.TrimSpace(source), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		return strings.Join(lines, "\n")
	}
	examples := make(map[string]bool)
	for _, block := range strings.Split(string(readme), "```coffee\n")[1:] {
		code, _, ok := strings.Cut(block, "\n```")
		if !ok {
			t.Fatal("unclosed README code block")
		}
		examples[normalize(code)] = true
	}
	// These examples are presented as complete source files in the README.
	for _, path := range []string{
		"02-Dragon/Dragon.spx",
		"03-Clone/main.spx",
		"03-Clone/Arrow.spx",
		"04-Bullet/MyAircraft.spx",
		"04-Bullet/Bullet.spx",
	} {
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile("../../tutorial/" + path)
			if err != nil {
				t.Fatal(err)
			}
			if !examples[normalize(string(source))] {
				t.Errorf("README must include the complete current source of tutorial/%s", path)
			}
		})
	}
}

func TestBuildAutoClosureConditions(t *testing.T) {
	ctx := ixgo.NewContext(xgobuild.StaticLoad)
	for _, pkg := range defaultPackagesToImport {
		if _, err := ctx.Loader.Import(pkg); err != nil {
			t.Fatalf("import %q: %v", pkg, err)
		}
	}

	source, err := xgobuild.BuildFSDir(ctx, newXGoParserFS(fstest.MapFS{
		"main.spx": {Data: []byte(`n := 0
repeatUntil n > 0, => {
	n++
}
waitUntil n > 1
`)},
		"SpEvent.spx": {Data: []byte(`var score int
onCond score >= 3, => {
	score++
}
`)},
	}), ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "spx.RepeatUntil(func() bool") {
		t.Fatalf("repeatUntil condition was not compiled as a closure:\n%s", source)
	}
	if !strings.Contains(string(source), "spx.WaitUntil(func() bool") {
		t.Fatalf("waitUntil condition was not compiled as a closure:\n%s", source)
	}
	if !strings.Contains(string(source), ".OnCond(func() bool") {
		t.Fatalf("onCond condition was not compiled as a closure:\n%s", source)
	}
}
