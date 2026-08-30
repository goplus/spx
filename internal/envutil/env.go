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

// Package envutil contains the small, deterministic environment operations
// shared by SPX's driver and launcher paths.
package envutil

import (
	"os"
	"runtime"
	"strings"
)

// NeutralGOFLAGS overrides persisted GOFLAGS without changing command behavior.
const NeutralGOFLAGS = "-x=false"

// Assignment describes one environment value to append after replacing all
// existing entries with the same key.
type Assignment struct {
	Key   string
	Value string
}

func resolve(env []string) []string {
	if env == nil {
		return os.Environ()
	}
	return env
}

func canonicalKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(key)
	}
	return key
}

// Lookup finds one key and reports whether it occurred more than once.
func Lookup(env []string, key string) (value string, found, duplicate bool) {
	key = canonicalKey(key)
	for _, entry := range env {
		name, current, ok := strings.Cut(entry, "=")
		if !ok || canonicalKey(name) != key {
			continue
		}
		if found {
			return "", true, true
		}
		value, found = current, true
	}
	return value, found, false
}

// HasNonEmpty reports whether any occurrence of key has a non-empty value.
func HasNonEmpty(env []string, key string) bool {
	key = canonicalKey(key)
	for _, entry := range resolve(env) {
		name, value, ok := strings.Cut(entry, "=")
		if ok && canonicalKey(name) == key && value != "" {
			return true
		}
	}
	return false
}

func filter(env []string, reject func(string) bool) []string {
	base := resolve(env)
	filtered := make([]string, 0, len(base))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && reject(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// Without removes entries whose key is one of keys.
func Without(env []string, keys ...string) []string {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[canonicalKey(key)] = struct{}{}
	}
	return filter(env, func(key string) bool {
		_, found := set[canonicalKey(key)]
		return found
	})
}

// WithoutPrefixes removes entries whose key starts with any prefix.
func WithoutPrefixes(env []string, prefixes ...string) []string {
	return filter(env, func(key string) bool {
		key = canonicalKey(key)
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, canonicalKey(prefix)) {
				return true
			}
		}
		return false
	})
}

// SetMany replaces each assignment key and appends its value in declaration
// order. Empty values are intentional and are retained.
func SetMany(env []string, assignments ...Assignment) []string {
	return setManyWithout(env, nil, assignments...)
}

func setManyWithout(env []string, removeKeys []string, assignments ...Assignment) []string {
	last := make(map[string]int, len(removeKeys)+len(assignments))
	for _, key := range removeKeys {
		last[canonicalKey(key)] = -1
	}
	for i, assignment := range assignments {
		last[canonicalKey(assignment.Key)] = i
	}
	result := filter(env, func(key string) bool {
		_, changed := last[canonicalKey(key)]
		return changed
	})
	for i, assignment := range assignments {
		if last[canonicalKey(assignment.Key)] == i {
			result = append(result, assignment.Key+"="+assignment.Value)
		}
	}
	return result
}

// HostGoEnvironment returns the deterministic environment for a host Go
// command. Ambient graph, target, and CGO selection cannot leak into it.
func HostGoEnvironment(env []string, goWork string, cgoEnabled bool, removeKeys ...string) []string {
	cgo := "0"
	if cgoEnabled {
		cgo = "1"
	}
	return setManyWithout(env, removeKeys,
		Assignment{Key: "GOFLAGS", Value: NeutralGOFLAGS},
		Assignment{Key: "GOWORK", Value: goWork},
		Assignment{Key: "GOOS", Value: runtime.GOOS},
		Assignment{Key: "GOARCH", Value: runtime.GOARCH},
		Assignment{Key: "CGO_ENABLED", Value: cgo},
	)
}
