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

package fs

// FileVerifier checks resources without requiring their contents to be read.
// A directory may implement it for binary assets stored outside its text source.
type FileVerifier interface {
	VerifyFile(name string) error
}

// DirectoryLister lists direct child directories without claiming the complete
// file and directory listing provided by ReadDirer.
type DirectoryLister interface {
	ListDirectories(name string) ([]string, error)
}

// ResourcePathValidator validates a configuration resource against its physical
// asset root. Virtual directories already enforce their own namespace.
type ResourcePathValidator interface {
	ValidateResourcePath(name string) error
}

// NativeResourcePaths supplies the filename capabilities of a native build.
type NativeResourcePaths struct{}

// VerifyFile uses a directory's optional verification capability, or opens and
// closes the file. It returns the original verification, open, or close error.
func VerifyFile(dir Dir, name string) error {
	if verifier, ok := dir.(FileVerifier); ok {
		return verifier.VerifyFile(name)
	}
	file, err := dir.Open(name)
	if err != nil {
		return err
	}
	return file.Close()
}

// ListDirectories prefers directory-only discovery, otherwise filters ReadDir
// while preserving its order. Directories without either capability return nil.
func ListDirectories(dir Dir, name string) ([]string, error) {
	if lister, ok := dir.(DirectoryLister); ok {
		return lister.ListDirectories(name)
	}
	reader, ok := dir.(ReadDirer)
	if !ok {
		return nil, nil
	}
	entries, err := reader.ReadDir(name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir {
			names = append(names, entry.Name)
		}
	}
	return names, nil
}

// ValidateResourcePath checks a normalized, nonempty configuration reference
// when the directory exposes a physical asset root.
func ValidateResourcePath(dir Dir, name string) error {
	if name == "" {
		return nil
	}
	if validator, ok := dir.(ResourcePathValidator); ok {
		return validator.ValidateResourcePath(name)
	}
	return nil
}

// AllowsLiteralResPrefix reports whether configuration paths in this source
// can use a bare res: prefix as part of a filename.
func AllowsLiteralResPrefix(source any) bool {
	v, ok := source.(interface{ AllowsLiteralResPrefix() bool })
	return ok && v.AllowsLiteralResPrefix()
}
