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

package licenseheader

import "testing"

func TestAddToGoSourceWithoutBuildTags(t *testing.T) {
	src := []byte("package demo\n")

	got := string(AddToGoSource(src))

	if got != Text+"package demo\n" {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestAddToGoSourceWithBuildTags(t *testing.T) {
	src := []byte("//go:build js\n// +build js\n/* generated */\npackage demo\n")

	got := string(AddToGoSource(src))
	want := "//go:build js\n// +build js\n\n" + Text + "/* generated */\npackage demo\n"

	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestAddToGoSourcePreservesExistingHeader(t *testing.T) {
	src := []byte(Text + "package demo\n")

	got := string(AddToGoSource(src))

	if got != string(src) {
		t.Fatalf("header duplicated:\n%s", got)
	}
}

func TestAddToGoSourcePreservesUTF8BOM(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("package demo\n")...)

	got := AddToGoSource(src)
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte(Text+"package demo\n")...)

	if string(got) != string(want) {
		t.Fatalf("unexpected output:\n%s", string(got))
	}
}
