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

package project

import (
	"bytes"
	"strings"
	"testing"
)

func TestPackedRootPreservesOriginalJSONWithoutSourceFields(t *testing.T) {
	packed := []byte("{ \"bgm\": \"first.wav\", \"bgm\": \"last.wav\" }\n")
	for _, source := range []string{"", "{}", "null"} {
		t.Run(source, func(t *testing.T) {
			index, err := parsePackedConfigIndex(packed, []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(index.projectRaw, packed) || !bytes.Equal(index.raw, packed) {
				t.Fatalf("unmerged packed JSON was rewritten: project=%s, raw=%s", index.projectRaw, index.raw)
			}
		})
	}
}

func TestPackedSectionsPreservePresenceAndErrorOrder(t *testing.T) {
	for _, tt := range []struct {
		name, packed string
		hasFonts     bool
		emptyFonts   bool
		errorPrefix  string
	}{
		{name: "absent", packed: `{}`},
		{name: "null", packed: `{"fonts": null}`, hasFonts: true},
		{name: "empty", packed: `{"fonts": {}}`, hasFonts: true, emptyFonts: true},
		{name: "sprites first", packed: `{"sprites": [], "sounds": false, "fonts": 1}`, errorPrefix: "sprites must be an object:"},
		{name: "sounds second", packed: `{"sprites": null, "sounds": false, "fonts": 1}`, errorPrefix: "sounds must be an object:"},
		{name: "fonts third", packed: `{"sprites": {}, "sounds": null, "fonts": []}`, errorPrefix: "fonts must be an object:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			index, err := parsePackedConfigIndex([]byte(tt.packed), nil)
			if tt.errorPrefix != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.errorPrefix) {
					t.Fatalf("error = %v, want prefix %q", err, tt.errorPrefix)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if index.hasFonts != tt.hasFonts || (index.fonts != nil) != tt.emptyFonts || len(index.fonts) != 0 {
				t.Fatalf("font catalog = (%v, %#v)", index.hasFonts, index.fonts)
			}
		})
	}
}
