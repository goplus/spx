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

package scratchcompat_test

import (
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

func TestKeyFromStringScratchKeyNames(t *testing.T) {
	for _, tt := range []struct {
		name string
		want spx.Key
	}{
		{"any", spx.KeyAny},
		{"space", spx.KeySpace},
		{"enter", spx.KeyEnter},
		{"up arrow", spx.KeyUp},
		{"down arrow", spx.KeyDown},
		{"left arrow", spx.KeyLeft},
		{"right arrow", spx.KeyRight},
		{"backspace", spx.KeyBackspace},
		{"delete", spx.KeyDelete},
		{"shift", spx.KeyShift},
		{"caps lock", spx.KeyCapsLock},
		{"scroll lock", spx.KeyScrollLock},
		{"control", spx.KeyControl},
		{"escape", spx.KeyEscape},
		{"insert", spx.KeyInsert},
		{"home", spx.KeyHome},
		{"end", spx.KeyEnd},
		{"page up", spx.KeyPageUp},
		{"page down", spx.KeyPageDown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := scratchcompat.KeyFromString(tt.name); got != tt.want {
				t.Errorf("scratchcompat.KeyFromString(%q) = %v, want %v", tt.name, got, tt.want)
			}
			if got := spx.KeyFromString(tt.name); got != spx.KeyMax {
				t.Errorf("spx.KeyFromString(%q) = %v, want KeyMax", tt.name, got)
			}
		})
	}
}

func TestKeyFromStringSPXNames(t *testing.T) {
	for _, tt := range []struct {
		name string
		want spx.Key
	}{
		{"Any", spx.KeyAny},
		{"Enter", spx.KeyEnter},
		{"Return", spx.KeyEnter},
		{"Space", spx.KeySpace},
		{"Up", spx.KeyUp},
		{"Escape", spx.KeyEscape},
		{"Ctrl", spx.KeyControl},
		{"a", spx.KeyA},
		{"A", spx.KeyA},
		{"0", spx.Key0},
		{"!", spx.KeyExclam},
		{"", spx.KeyMax},
		{"not a keyboard key", spx.KeyMax},
		{"ENTER", spx.KeyMax},
		{"Up Arrow", spx.KeyMax},
		{" up arrow", spx.KeyMax},
		{"up arrow ", spx.KeyMax},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := scratchcompat.KeyFromString(tt.name); got != tt.want {
				t.Errorf("scratchcompat.KeyFromString(%q) = %v, want %v", tt.name, got, tt.want)
			}
			if got := spx.KeyFromString(tt.name); got != tt.want {
				t.Errorf("spx.KeyFromString(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
