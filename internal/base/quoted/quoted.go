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

// Package quoted handles fields in Go toolchain environment variables.
package quoted

import (
	"fmt"
	"strings"
)

// Split separates fields using Go's quoting rules: quotes may surround a whole
// field; neither quotes within unquoted fields nor backslashes are interpreted.
func Split(value string) ([]string, error) {
	var fields []string
	for len(value) > 0 {
		for len(value) > 0 && isSpace(value[0]) {
			value = value[1:]
		}
		if value == "" {
			break
		}
		if value[0] == '\'' || value[0] == '"' {
			quote := value[0]
			value = value[1:]
			end := strings.IndexByte(value, quote)
			if end < 0 {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			fields = append(fields, value[:end])
			value = value[end+1:]
			continue
		}
		end := 0
		for end < len(value) && !isSpace(value[end]) {
			end++
		}
		fields = append(fields, value[:end])
		value = value[end:]
	}
	return fields, nil
}

// Join preserves fields for Split, quoting empty fields, whitespace, and leading
// quotes with an available quote style.
func Join(fields []string) (string, error) {
	quoted := make([]string, len(fields))
	for index, field := range fields {
		hasSpace := false
		hasSingle := strings.ContainsRune(field, '\'')
		hasDouble := strings.ContainsRune(field, '"')
		for offset := 0; offset < len(field); offset++ {
			if isSpace(field[offset]) {
				hasSpace = true
				break
			}
		}
		switch {
		case field != "" && !hasSpace && field[0] != '\'' && field[0] != '"':
			quoted[index] = field
		case !hasSingle:
			quoted[index] = "'" + field + "'"
		case !hasDouble:
			quoted[index] = `"` + field + `"`
		default:
			return "", fmt.Errorf("field %q needs quoting but contains both quote characters", field)
		}
	}
	return strings.Join(quoted, " "), nil
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}
