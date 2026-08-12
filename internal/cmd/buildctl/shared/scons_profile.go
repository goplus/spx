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

package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	SConsProfileFilename = "spx_scons_profile.json"
	sconsProfileSchema   = 1
)

var orchestrationSConsKeys = map[string]struct{}{
	"angle":            {},
	"arch":             {},
	"custom_modules":   {},
	"dev_build":        {},
	"generate_bundle":  {},
	"ios_simulator":    {},
	"linker":           {},
	"platform":         {},
	"proxy_to_pthread": {},
	"target":           {},
	"tests":            {},
	"threads":          {},
	"use_llvm":         {},
	"vsproj":           {},
}

// sconsProfile is the module-owned, ordered set of SCons arguments used to
// build an external SPX module.
type sconsProfile struct {
	Schema          int
	Common          []string
	EditorRelease   []string
	TemplateRelease []string
}

func loadSConsProfile(spxModuleSource string) (sconsProfile, error) {
	path := filepath.Join(spxModuleSource, SConsProfileFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return sconsProfile{}, fmt.Errorf("read SCons profile %s: %w", path, err)
	}
	profile, err := parseSConsProfile(data)
	if err != nil {
		return sconsProfile{}, fmt.Errorf("parse SCons profile %s: %w", path, err)
	}
	return profile, nil
}

func parseSConsProfile(data []byte) (sconsProfile, error) {
	fields, err := decodeSConsProfileFields(data)
	if err != nil {
		return sconsProfile{}, err
	}

	for _, name := range []string{"schema", "common", "editor_release", "template_release"} {
		if _, ok := fields[name]; !ok {
			return sconsProfile{}, fmt.Errorf("missing SCons profile field %q", name)
		}
	}

	var profile sconsProfile
	if err := json.Unmarshal(fields["schema"], &profile.Schema); err != nil {
		return sconsProfile{}, fmt.Errorf("invalid SCons profile schema: %w", err)
	}
	if profile.Schema != sconsProfileSchema {
		return sconsProfile{}, fmt.Errorf("unsupported SCons profile schema %d, want %d", profile.Schema, sconsProfileSchema)
	}

	groups := []struct {
		name string
		dst  *[]string
	}{
		{name: "common", dst: &profile.Common},
		{name: "editor_release", dst: &profile.EditorRelease},
		{name: "template_release", dst: &profile.TemplateRelease},
	}
	keysByGroup := make(map[string]map[string]struct{}, len(groups))
	for _, group := range groups {
		if bytes.Equal(bytes.TrimSpace(fields[group.name]), []byte("null")) {
			return sconsProfile{}, fmt.Errorf("SCons profile field %q must be an array", group.name)
		}
		if err := json.Unmarshal(fields[group.name], group.dst); err != nil {
			return sconsProfile{}, fmt.Errorf("invalid SCons profile field %q: %w", group.name, err)
		}
		keys, err := validateSConsProfileArgs(group.name, *group.dst)
		if err != nil {
			return sconsProfile{}, err
		}
		keysByGroup[group.name] = keys
	}

	for _, targetGroup := range []string{"editor_release", "template_release"} {
		for key := range keysByGroup[targetGroup] {
			if _, duplicate := keysByGroup["common"][key]; duplicate {
				return sconsProfile{}, fmt.Errorf("duplicate SCons key %q across %q and %q", key, "common", targetGroup)
			}
		}
	}

	return profile, nil
}

func decodeSConsProfileFields(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("invalid SCons profile JSON: %w", err)
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("invalid SCons profile JSON: top-level value must be an object")
	}

	knownFields := map[string]struct{}{
		"schema": {}, "common": {}, "editor_release": {}, "template_release": {},
	}
	fields := make(map[string]json.RawMessage, len(knownFields))
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid SCons profile JSON: %w", err)
		}
		name, ok := nameToken.(string)
		if !ok {
			return nil, fmt.Errorf("invalid SCons profile JSON: object field name must be a string")
		}
		if _, ok := knownFields[name]; !ok {
			return nil, fmt.Errorf("unknown SCons profile field %q", name)
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, fmt.Errorf("duplicate SCons profile field %q", name)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("invalid SCons profile field %q: %w", name, err)
		}
		fields[name] = raw
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("invalid SCons profile JSON: %w", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("invalid SCons profile JSON: trailing value")
		}
		return nil, fmt.Errorf("invalid SCons profile JSON: %w", err)
	}
	return fields, nil
}

func validateSConsProfileArgs(group string, args []string) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(args))
	for index, arg := range args {
		if arg == "" {
			return nil, fmt.Errorf("SCons profile field %q entry %d is empty", group, index)
		}
		if strings.IndexFunc(arg, unicode.IsSpace) >= 0 {
			return nil, fmt.Errorf("SCons profile field %q entry %d contains whitespace: %q", group, index, arg)
		}
		if strings.IndexFunc(arg, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("SCons profile field %q entry %d contains a control character: %q", group, index, arg)
		}
		key, value, ok := strings.Cut(arg, "=")
		if !ok || strings.Count(arg, "=") != 1 || !validSConsKey(key) || value == "" {
			return nil, fmt.Errorf("SCons profile field %q entry %d must be a non-empty key=value string: %q", group, index, arg)
		}
		if _, reserved := orchestrationSConsKeys[key]; reserved {
			return nil, fmt.Errorf("SCons key %q in profile field %q is owned by build orchestration", key, group)
		}
		if _, duplicate := keys[key]; duplicate {
			return nil, fmt.Errorf("duplicate SCons key %q in profile field %q", key, group)
		}
		keys[key] = struct{}{}
	}
	return keys, nil
}

func validSConsKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

func composeSConsBuildArgs(moduleSource string, buildArgs []string, profileGroups ...[]string) []string {
	length := len(buildArgs) + 1
	for _, group := range profileGroups {
		length += len(group)
	}
	args := make([]string, 0, length)
	for _, group := range profileGroups {
		args = append(args, group...)
	}
	args = append(args, buildArgs...)
	args = append(args, "custom_modules="+moduleSource)
	return args
}
