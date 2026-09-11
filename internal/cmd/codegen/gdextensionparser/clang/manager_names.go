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

package clang

import (
	"sort"
	"strings"
	"unicode"
)

// ManagerNames resolves the longest known prefix without sorting during lookup.
// Its zero value uses the legacy two-byte-prefix fallback.
type ManagerNames struct{ longestFirst []string }

func NewManagerNames(names []string) ManagerNames {
	result := ManagerNames{longestFirst: append([]string(nil), names...)}
	for i, name := range result.longestFirst {
		result.longestFirst[i] = strings.ToLower(name)
	}
	sort.Slice(result.longestFirst, func(i, j int) bool {
		return len(result.longestFirst[i]) > len(result.longestFirst[j])
	})
	return result
}

func (m ManagerNames) Resolve(name string) string {
	return m.resolve(name, unicode.IsUpper)
}

// resolveASCII retains the parser's ASCII fallback. Parsed C identifiers are ASCII;
// the template helpers historically also accept manually constructed Unicode names.
func (m ManagerNames) resolveASCII(name string) string {
	return m.resolve(name, func(r rune) bool { return r >= 'A' && r <= 'Z' })
}

func (m ManagerNames) resolve(name string, isUpper func(rune) bool) string {
	suffix := name[len("GDExtensionSpx"):]
	lower := strings.ToLower(suffix)
	for _, manager := range m.longestFirst {
		if strings.HasPrefix(lower, manager) {
			return manager
		}
	}
	chars := []rune{rune(suffix[0]), rune(suffix[1])}
	for _, r := range suffix[2:] {
		if isUpper(r) {
			break
		}
		chars = append(chars, r)
	}
	return strings.ToLower(string(chars))
}
