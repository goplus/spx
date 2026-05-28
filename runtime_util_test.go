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

package spx

import (
	"testing"
)

func TestCharAt(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		idx      int
		expected string
	}{
		{
			name:     "first character",
			s:        "hello",
			idx:      0,
			expected: "h",
		},
		{
			name:     "last character",
			s:        "hello",
			idx:      4,
			expected: "o",
		},
		{
			name:     "middle character",
			s:        "hello",
			idx:      2,
			expected: "l",
		},
		{
			name:     "index equals length returns empty string",
			s:        "hello",
			idx:      5,
			expected: "",
		},
		{
			name:     "negative index returns empty string",
			s:        "hello",
			idx:      -1,
			expected: "",
		},
		{
			name:     "index exceeds length returns empty string",
			s:        "hello",
			idx:      6,
			expected: "",
		},
		{
			name:     "empty string returns empty string",
			s:        "",
			idx:      0,
			expected: "",
		},
		{
			name:     "unicode multibyte character",
			s:        "你好世界",
			idx:      0,
			expected: "你",
		},
		{
			name:     "unicode last character",
			s:        "你好世界",
			idx:      3,
			expected: "界",
		},
		{
			name:     "unicode index out of range",
			s:        "你好世界",
			idx:      4,
			expected: "",
		},
		{
			name:     "single character string",
			s:        "a",
			idx:      0,
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CharAt(tt.s, tt.idx)
			if got != tt.expected {
				t.Errorf("CharAt(%q, %d) = %q, want %q", tt.s, tt.idx, got, tt.expected)
			}
		})
	}
}
