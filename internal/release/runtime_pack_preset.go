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

package release

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	runtimePackExportPresets    = "cmd/spx/template/project/export_presets.cfg"
	runtimePackExportPresetName = "Linux"
)

func projectGodotPreset(content []byte, name string) ([]byte, error) {
	sections := make(map[string][]string)
	section := ""
	for _, rawLine := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section == "" || line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		sections[section] = append(sections[section], line)
	}

	key := ""
	for candidate, lines := range sections {
		if strings.Contains(candidate, ".options") || !strings.HasPrefix(candidate, "preset.") {
			continue
		}
		for _, line := range lines {
			value, ok := godotConfigString(line, "name")
			if !ok || value != name {
				continue
			}
			if key != "" {
				return nil, fmt.Errorf("duplicate preset %q", name)
			}
			key = candidate
		}
	}
	if key == "" {
		return nil, fmt.Errorf("preset %q not found", name)
	}
	options, ok := sections[key+".options"]
	if !ok {
		return nil, fmt.Errorf("preset %q has no options", name)
	}

	var projected strings.Builder
	projected.WriteString("[preset]\n")
	for _, line := range sections[key] {
		projected.WriteString(line)
		projected.WriteByte('\n')
	}
	projected.WriteString("[preset.options]\n")
	for _, line := range options {
		projected.WriteString(line)
		projected.WriteByte('\n')
	}
	return []byte(projected.String()), nil
}

func godotConfigString(line, key string) (string, bool) {
	left, right, ok := strings.Cut(line, "=")
	if !ok || strings.TrimSpace(left) != key {
		return "", false
	}
	value, err := strconv.Unquote(strings.TrimSpace(right))
	return value, err == nil
}
