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

package projectbundle

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func validateRelativePath(name, field string) (string, error) {
	if name == "" || name == "." || !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("%w: %s %q", ErrInvalidPath, field, name)
	}
	if strings.ContainsRune(name, '\\') || path.IsAbs(name) || filepath.IsAbs(name) || looksLikeWindowsAbsolutePath(name) {
		return "", fmt.Errorf("%w: %s %q must be slash-separated and relative", ErrInvalidPath, field, name)
	}
	if path.Clean(name) != name {
		return "", fmt.Errorf("%w: %s %q is not clean", ErrInvalidPath, field, name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("%w: %s %q contains an invalid component", ErrInvalidPath, field, name)
		}
		if err := validatePortableComponent(component); err != nil {
			return "", fmt.Errorf("%w: %s %q: %v", ErrInvalidPath, field, name, err)
		}
	}
	return name, nil
}

func validatePortableComponent(component string) error {
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return errors.New("component ends in a dot or space")
	}
	if strings.ContainsAny(component, `<>:"|?*`) {
		return errors.New("component contains a Windows-reserved character")
	}
	for _, character := range component {
		if character < 0x20 {
			return errors.New("component contains a control character")
		}
	}

	base := component
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	base = strings.ToUpper(strings.TrimRight(base, " ."))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
		return errors.New("component uses a reserved DOS device name")
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("component uses a reserved DOS device name")
	}
	for _, prefix := range []string{"COM", "LPT"} {
		for _, digit := range []string{"¹", "²", "³"} {
			if base == prefix+digit {
				return errors.New("component uses a reserved DOS device name")
			}
		}
	}
	return nil
}

func looksLikeWindowsAbsolutePath(name string) bool {
	if strings.HasPrefix(name, "//") || strings.HasPrefix(name, `\\`) {
		return true
	}
	return len(name) >= 3 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':' && (name[2] == '/' || name[2] == '\\')
}

func validateOutputs(projectDir, output, finalOutput, packRoot string) error {
	var roots []string
	if packRoot != "" {
		roots = append(roots, packRoot)
	}

	canonicalRoots := make([]string, len(roots))
	for i, root := range roots {
		canonical, err := canonicalFilesystemPath(root)
		if err != nil {
			return fmt.Errorf("projectbundle: canonicalize input root %q: %w", root, err)
		}
		canonicalRoots[i] = canonical
	}
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "Output", value: output},
		{field: "FinalOutput", value: finalOutput},
	} {
		if item.value == "" {
			continue
		}
		candidate := item.value
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(projectDir, candidate)
		}
		canonical, err := canonicalFilesystemPath(candidate)
		if err != nil {
			return fmt.Errorf("projectbundle: canonicalize %s %q: %w", item.field, item.value, err)
		}
		for i, root := range canonicalRoots {
			if isSameOrWithin(canonical, root) {
				return fmt.Errorf("%w: %s %q is within collected root %q", ErrInvalidPath, item.field, item.value, roots[i])
			}
		}
	}
	return nil
}

func canonicalFilesystemPath(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	current := absolute
	var suffix []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

type observedRoot struct {
	path string
	info os.FileInfo
}

func observeRoot(name string) (observedRoot, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return observedRoot{}, err
	}
	absolute = filepath.Clean(absolute)
	before, err := os.Stat(absolute)
	if err != nil {
		return observedRoot{}, err
	}
	if !before.IsDir() {
		return observedRoot{}, fmt.Errorf("%w: root %q is not a directory", ErrUnsafeFile, name)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return observedRoot{}, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return observedRoot{}, err
	}
	canonical = filepath.Clean(canonical)
	after, err := os.Lstat(canonical)
	if err != nil {
		return observedRoot{}, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, after) {
		return observedRoot{}, fmt.Errorf("%w: root %q changed while it was canonicalized", ErrUnsafeFile, name)
	}
	return observedRoot{path: canonical, info: before}, nil
}

func isSameOrWithin(name, root string) bool {
	relative, err := filepath.Rel(root, name)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
