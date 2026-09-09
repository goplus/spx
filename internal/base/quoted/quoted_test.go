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

package quoted

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	got, err := Split(`-trimpath ' -modfile=module with spaces ' "-buildvcs=false"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-trimpath", " -modfile=module with spaces ", "-buildvcs=false"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
	if _, err := Split(`'-trimpath`); err == nil {
		t.Fatal("Split accepted an unterminated quote")
	}
}

func TestJoinPreservesFields(t *testing.T) {
	for _, fields := range [][]string{
		{""},
		{"-trimpath", "", "-buildvcs=false"},
		{"'leading", `"leading`},
		{"two words", "tab\tfield", "line\nfield"},
		{`-DNAME="value"`, `-DNAME='value'`, `both'"quotes`},
		{`C:\Program Files\SDK`, "中文路径"},
	} {
		joined, err := Join(fields)
		if err != nil {
			t.Fatalf("Join(%q): %v", fields, err)
		}
		got, err := Split(joined)
		if err != nil || !reflect.DeepEqual(got, fields) {
			t.Errorf("Split(Join(%q)) = %q, %v", fields, got, err)
		}
	}
}

func TestJoinRejectsUnrepresentableFields(t *testing.T) {
	for _, field := range []string{`both ' and " quotes`, `'both"`, `"both'`} {
		if _, err := Join([]string{field}); err == nil {
			t.Errorf("Join(%q) accepted a field requiring both quote styles", field)
		}
	}
}
