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

package strictjson

import (
	"reflect"
	"strings"
	"testing"
)

type testDocument struct {
	Name   string `json:"name"`
	Nested struct {
		Enabled bool `json:"enabled"`
	} `json:"nested"`
	Items []struct {
		Value int `json:"value"`
	} `json:"items"`
}

func TestDecodeRejectsAmbiguousDocuments(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "duplicate nested key",
			data: `{"name":"test","nested":{"enabled":true,"enabled":false}}`,
			want: `duplicate JSON key "enabled" at $.nested`,
		},
		{
			name: "duplicate key in array object",
			data: `{"name":"test","items":[{"value":1,"value":2}]}`,
			want: `duplicate JSON key "value" at $.items[0]`,
		},
		{
			name: "unknown field",
			data: `{"name":"test","unexpected":true}`,
			want: `unknown field "unexpected"`,
		},
		{
			name: "second value",
			data: `{"name":"first"} {"name":"second"}`,
			want: "multiple JSON values",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var document testDocument
			err := Decode([]byte(test.data), &document)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Decode error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestDecodeLetsDestinationDetermineTopLevelType(t *testing.T) {
	var numbers []int
	if err := Decode([]byte(`[1, 2, 3]`), &numbers); err != nil {
		t.Fatalf("Decode array: %v", err)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(numbers, want) {
		t.Fatalf("Decode array = %v, want %v", numbers, want)
	}

	var number int
	if err := Decode([]byte(`7`), &number); err != nil {
		t.Fatalf("Decode scalar: %v", err)
	}
	if number != 7 {
		t.Fatalf("Decode scalar = %d, want 7", number)
	}

	var document testDocument
	if err := Decode([]byte(`[1, 2, 3]`), &document); err == nil {
		t.Fatal("Decode accepted an array for a struct destination")
	}
}
