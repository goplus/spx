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

package assetindex

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMergeRootPrecedence(t *testing.T) {
	source := map[string]json.RawMessage{
		"bgm":       json.RawMessage(`"source.wav"`),
		"backdrops": json.RawMessage(`[{"path":"source.png"}]`),
	}
	packed := map[string]json.RawMessage{"bgm": json.RawMessage(`"packed.wav"`)}
	var config struct {
		Bgm       string `json:"bgm"`
		Backdrops []struct {
			Path string `json:"path"`
		} `json:"backdrops"`
	}
	if err := DecodeRoot(Merge(source, packed), &config); err != nil {
		t.Fatal(err)
	}
	if config.Bgm != "packed.wav" || len(config.Backdrops) != 1 || config.Backdrops[0].Path != "source.png" {
		t.Fatalf("merged config = %+v", config)
	}
	if string(source["bgm"]) != `"source.wav"` {
		t.Fatalf("source root changed: %s", source["bgm"])
	}
}

func TestMergeOptionalRoots(t *testing.T) {
	packed := map[string]json.RawMessage{"bgm": json.RawMessage(`"packed.wav"`)}
	for _, tt := range []struct {
		name   string
		source map[string]json.RawMessage
		packed map[string]json.RawMessage
		want   map[string]json.RawMessage
	}{
		{name: "both absent"},
		{name: "nil source", packed: packed, want: packed},
		{name: "empty source", source: map[string]json.RawMessage{}, packed: packed, want: packed},
		{name: "source only", source: packed, want: packed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Merge(tt.source, tt.packed); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeRoot(t *testing.T) {
	for _, tt := range []struct {
		name    string
		root    map[string]json.RawMessage
		want    string
		wantErr bool
	}{
		{name: "nil root", want: "existing.wav"},
		{name: "empty root", root: map[string]json.RawMessage{}, want: "existing.wav"},
		{name: "valid field", root: map[string]json.RawMessage{"bgm": json.RawMessage(`"packed.wav"`)}, want: "packed.wav"},
		{name: "malformed field", root: map[string]json.RawMessage{"bgm": json.RawMessage(`"unfinished`)}, want: "existing.wav", wantErr: true},
		{name: "wrong field type", root: map[string]json.RawMessage{"bgm": json.RawMessage(`42`)}, want: "existing.wav", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := struct {
				Bgm string `json:"bgm"`
			}{Bgm: "existing.wav"}
			if err := DecodeRoot(tt.root, &config); (err != nil) != tt.wantErr {
				t.Fatalf("DecodeRoot() error = %v, want error %v", err, tt.wantErr)
			}
			if config.Bgm != tt.want {
				t.Fatalf("decoded bgm = %q, want %q", config.Bgm, tt.want)
			}
		})
	}
}

func TestParseEntriesPresence(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`)} {
		entries, err := ParseEntries(raw)
		if err != nil || entries != nil {
			t.Fatalf("ParseEntries(%s) = %v, %v; want nil", raw, entries, err)
		}
	}
	entries, err := ParseEntries(json.RawMessage(`{}`))
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("ParseEntries({}) = %v, %v; want empty object", entries, err)
	}
	if _, err := ParseEntries(json.RawMessage(`[]`)); err == nil {
		t.Fatal("ParseEntries accepted an array section")
	}
}
