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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var macOSCGOFlagVariables = []string{
	"CGO_CFLAGS",
	"CGO_CPPFLAGS",
	"CGO_CXXFLAGS",
	"CGO_LDFLAGS",
}

type macOSXcrunResolver func(args ...string) (string, error)
type macOSPathPredicate func(path string) bool

// configureMacOSGoToolchainEnv replaces stale Darwin SDK/compiler inputs in a
// child environment. Work happens on a copy so a failed xcrun lookup never
// leaves callers with a partially rewritten environment.
func configureMacOSGoToolchainEnv(
	goos string,
	env map[string]string,
	xcrun macOSXcrunResolver,
	isDirectory macOSPathPredicate,
	isExecutable macOSPathPredicate,
) error {
	if goos != "darwin" {
		return nil
	}
	if env == nil {
		return fmt.Errorf("macOS Go toolchain environment must not be nil")
	}

	next := cloneStringMap(env)
	sdkRoot := next["SDKROOT"]
	if !macOSSDKDirectoryIsUsable(sdkRoot, isDirectory) {
		resolved, err := resolveMacOSXcrunPath(xcrun, isDirectory, "SDK", "--show-sdk-path")
		if err != nil {
			return err
		}
		sdkRoot = resolved
		next["SDKROOT"] = resolved
	}

	for _, compiler := range []struct {
		environment string
		tool        string
		description string
	}{
		{environment: "CC", tool: "clang", description: "C compiler"},
		{environment: "CXX", tool: "clang++", description: "C++ compiler"},
	} {
		if macOSCommandIsUsable(next[compiler.environment], next, isExecutable) {
			continue
		}
		resolved, err := resolveMacOSXcrunPath(xcrun, isExecutable, compiler.description, "--find", compiler.tool)
		if err != nil {
			return err
		}
		next[compiler.environment] = resolved
	}

	for _, name := range macOSCGOFlagVariables {
		value, ok := next[name]
		if !ok || value == "" {
			continue
		}
		normalized, err := normalizeMacOSCGOFlags(value, sdkRoot, isDirectory)
		if err != nil {
			return fmt.Errorf("normalize %s: %w", name, err)
		}
		next[name] = normalized
	}

	for key := range env {
		delete(env, key)
	}
	for key, value := range next {
		env[key] = value
	}
	return nil
}

func configureCurrentMacOSGoToolchainEnv(env map[string]string) error {
	return configureMacOSGoToolchainEnv(
		runtime.GOOS,
		env,
		func(args ...string) (string, error) { return runMacOSXcrun(env, args...) },
		pathIsDirectory,
		pathIsExecutable,
	)
}

func runMacOSXcrun(env map[string]string, args ...string) (string, error) {
	xcrunPath, err := resolveCommandPath("xcrun", env)
	if err != nil {
		return "", fmt.Errorf("resolve xcrun: %w", err)
	}

	commandEnv := cloneStringMap(env)
	delete(commandEnv, "SDKROOT")
	cmd := exec.Command(xcrunPath, append([]string{"--sdk", "macosx"}, args...)...)
	cmd.Env = envMapToSlice(commandEnv)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return "", fmt.Errorf("xcrun %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return "", fmt.Errorf("xcrun %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func resolveMacOSXcrunPath(xcrun macOSXcrunResolver, valid macOSPathPredicate, description string, args ...string) (string, error) {
	resolved, err := xcrun(args...)
	if err != nil {
		return "", fmt.Errorf("resolve macOS %s: %w", description, err)
	}
	if resolved == "" || strings.ContainsAny(resolved, "\x00\r\n") || !filepath.IsAbs(resolved) || !valid(resolved) {
		return "", fmt.Errorf("xcrun returned invalid macOS %s %q", description, resolved)
	}
	return resolved, nil
}

func macOSCommandIsUsable(value string, env map[string]string, isExecutable macOSPathPredicate) bool {
	fields, err := splitQuotedFields(value)
	if err != nil || len(fields) == 0 {
		return false
	}
	command := fields[0]
	if filepath.IsAbs(command) {
		return isExecutable(command)
	}
	if strings.ContainsRune(command, filepath.Separator) {
		return false
	}
	for _, directory := range filepath.SplitList(pathEnvValue(env)) {
		if directory != "" && isExecutable(filepath.Join(directory, command)) {
			return true
		}
	}
	return false
}

func normalizeMacOSCGOFlags(value, sdkRoot string, isDirectory macOSPathPredicate) (string, error) {
	fields, err := splitQuotedFields(value)
	if err != nil {
		return "", err
	}
	changed := false
	normalized := make([]string, 0, len(fields)+1)
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		switch field {
		case "-isysroot", "--sysroot":
			normalized = append(normalized, field)
			if index+1 >= len(fields) || strings.HasPrefix(fields[index+1], "-") {
				normalized = append(normalized, sdkRoot)
				changed = true
				continue
			}
			index++
			root := fields[index]
			if !macOSSDKDirectoryIsUsable(root, isDirectory) {
				root = sdkRoot
				changed = true
			}
			normalized = append(normalized, root)
			continue
		}

		if replacement, ok := normalizeAttachedSysroot(field, sdkRoot, isDirectory); ok {
			normalized = append(normalized, replacement)
			changed = changed || replacement != field
			continue
		}
		replacement := replaceMissingSDKPaths(field, sdkRoot, isDirectory)
		normalized = append(normalized, replacement)
		changed = changed || replacement != field
	}
	if !changed {
		return value, nil
	}
	return joinQuotedFields(normalized)
}

func normalizeAttachedSysroot(field, sdkRoot string, isDirectory macOSPathPredicate) (string, bool) {
	for _, prefix := range []string{"-isysroot=", "--sysroot=", "-isysroot", "--sysroot"} {
		if strings.HasPrefix(field, prefix) {
			root := strings.TrimPrefix(field, prefix)
			if root == "" {
				continue
			}
			if !macOSSDKDirectoryIsUsable(root, isDirectory) {
				root = sdkRoot
			}
			return prefix + root, true
		}
	}
	return "", false
}

func macOSSDKDirectoryIsUsable(path string, isDirectory macOSPathPredicate) bool {
	return filepath.IsAbs(path) && isDirectory(path)
}

// replaceMissingSDKPaths handles SDK references embedded in flags such as
// -I.../MacOSX.sdk/usr/include and -Wl,-syslibroot,.../MacOSX.sdk.
func replaceMissingSDKPaths(field, sdkRoot string, isDirectory macOSPathPredicate) string {
	searchFrom := 0
	for {
		lower := strings.ToLower(field[searchFrom:])
		relativeEnd := strings.Index(lower, ".sdk")
		if relativeEnd < 0 {
			return field
		}
		end := searchFrom + relativeEnd + len(".sdk")
		prefix := field[:end]
		segmentStart := strings.LastIndexAny(prefix, "=,") + 1
		relativeSlash := strings.Index(prefix[segmentStart:], "/")
		if relativeSlash < 0 {
			searchFrom = end
			continue
		}
		start := segmentStart + relativeSlash
		candidate := field[start:end]
		if filepath.IsAbs(candidate) && !isDirectory(candidate) {
			field = field[:start] + sdkRoot + field[end:]
			searchFrom = start + len(sdkRoot)
			continue
		}
		searchFrom = end
	}
}

// splitQuotedFields and joinQuotedFields match the quoting rules used by Go
// for CGO_* environment variables: quotes may surround a whole field and are
// not shell-evaluated or unescaped.
func splitQuotedFields(value string) ([]string, error) {
	var fields []string
	for len(value) > 0 {
		for len(value) > 0 && isQuotedFieldSpace(value[0]) {
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
		for end < len(value) && !isQuotedFieldSpace(value[end]) {
			end++
		}
		if end == len(value) {
			fields = append(fields, value)
			break
		}
		fields = append(fields, value[:end])
		value = value[end:]
	}
	return fields, nil
}

func joinQuotedFields(fields []string) (string, error) {
	quoted := make([]string, len(fields))
	for index, field := range fields {
		hasSpace := false
		hasSingle := strings.ContainsRune(field, '\'')
		hasDouble := strings.ContainsRune(field, '"')
		for offset := 0; offset < len(field); offset++ {
			if isQuotedFieldSpace(field[offset]) {
				hasSpace = true
				break
			}
		}
		switch {
		case !hasSpace:
			quoted[index] = field
		case !hasSingle:
			quoted[index] = "'" + field + "'"
		case !hasDouble:
			quoted[index] = `"` + field + `"`
		default:
			return "", fmt.Errorf("field %q contains whitespace and both quote characters", field)
		}
	}
	return strings.Join(quoted, " "), nil
}

func isQuotedFieldSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func pathIsDirectory(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathIsExecutable(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
