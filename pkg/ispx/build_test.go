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
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
)

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
