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

package keycode

import "testing"

func TestParseRecognizesAliases(t *testing.T) {
	tests := []struct {
		name string
		want int64
	}{
		{name: "A", want: KeyA},
		{name: "a", want: KeyA},
		{name: "Enter", want: KeyEnter},
		{name: "Return", want: KeyEnter},
		{name: "Ctrl", want: KeyControl},
		{name: "!", want: KeyExclam},
		{name: "KP/", want: KeyKPDivide},
		{name: " ", want: KeySpace},
	}

	for _, tt := range tests {
		got, ok := Parse(tt.name)
		if !ok {
			t.Fatalf("Parse(%q) reported unknown key", tt.name)
		}
		if got != tt.want {
			t.Fatalf("Parse(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	if _, ok := Parse("NotAKey"); ok {
		t.Fatal("Parse accepted an unknown key")
	}
}
