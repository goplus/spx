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

package pathutil

import "testing"

func TestPortableComponentCharacters(t *testing.T) {
	for _, tt := range []struct {
		name string
		want ComponentProblem
	}{
		{"sprite.png", ComponentOK}, {"精灵.png", ComponentOK}, {"name.", TrailingDotOrSpace}, {"name ", TrailingDotOrSpace}, {"a:b", ReservedCharacter}, {"a?b", ReservedCharacter}, {"a\x00b", ControlCharacter}, {"a\nb", ControlCharacter},
	} {
		if got := CheckComponent(tt.name); got != tt.want {
			t.Errorf("%q: %v, want %v", tt.name, got, tt.want)
		}
	}
}
func TestDeviceBasenames(t *testing.T) {
	for _, name := range []string{"CON", "con", "CLOCK$", "clock$", "COM1", "com9", "LPT¹", "lpt³"} {
		if !IsDeviceBase(name) {
			t.Errorf("accepted device %q", name)
		}
	}
	for _, name := range []string{"", "CONSOLE", "COM0", "COM10", "LPT4x", "CLOCK$", "Con"} {
		if IsDeviceBase(name) {
			t.Errorf("changed caller folding policy for %q", name)
		}
	}
}
