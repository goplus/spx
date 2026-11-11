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

package project

import (
	"path"
	"strings"
)

// SpineAtlasPages lists the texture paths in a Spine text atlas. Each page
// starts a nonempty block; subsequent lines describe its metadata and regions.
func SpineAtlasPages(atlas string) []string {
	var pages []string
	pageStart := true
	header := true
	headerStarted := false
	for _, line := range strings.Split(strings.TrimPrefix(atlas, "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			// Leading blank lines precede the header. Once header entries have
			// started, a blank line ends them, as in spine::Atlas::load.
			if headerStarted {
				header = false
			}
			pageStart = true
			continue
		}
		if header && isSpineAtlasHeader(line) {
			headerStarted = true
			continue
		}
		header = false
		if pageStart {
			pages = append(pages, line)
			pageStart = false
		}
	}
	return pages
}

// ValidSpineAtlasPage confines texture pages to the atlas directory and its
// descendants. Keep this consistent with the native Spine resource loader.
func ValidSpineAtlasPage(page string) bool {
	if page == "" || path.IsAbs(page) || strings.ContainsAny(page, "\\\x00:") {
		return false
	}
	for _, part := range strings.Split(page, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// Header entries precede the first page only. Do not mistake URI or drive
// paths for entries merely because they contain a colon.
func isSpineAtlasHeader(line string) bool {
	key, value, found := strings.Cut(line, ":")
	if !found || key == "" || value == "" || strings.ContainsAny(line, "/\\\x00") {
		return false
	}
	for _, ch := range strings.TrimSpace(key) {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return false
		}
	}
	// Exported entries separate textual values with whitespace. Numeric
	// entries such as scale:1 may omit that space.
	ch := value[0]
	return ch == ' ' || ch == '\t' || ch >= '0' && ch <= '9' || ch == '-' || ch == '+' || ch == '.'
}
