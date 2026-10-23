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

package builderai

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/goplus/spx/v4/internal/scaffold"
)

const (
	ModulePath         = "github.com/goplus/builder/tools/ai"
	DescriptionFile    = "builder-ai-description.md"
	DescriptionFileRaw = "builder-ai-description"
	spxModulePath      = "github.com/goplus/spx/v4"
)

type ProjectOptions struct {
	GoModTemplate string
	FindSpxRoot   func(startDir string) string
}

func EnsureProjectFiles(projectRoot string, opts ProjectOptions) error {
	if !HasDescription(projectRoot) {
		return nil
	}
	if err := EnsureGoxMod(projectRoot); err != nil {
		return err
	}
	return EnsureGoMod(projectRoot, opts)
}

func HasDescription(projectRoot string) bool {
	if projectRoot == "" {
		return false
	}
	for _, name := range []string{DescriptionFile, DescriptionFileRaw} {
		info, err := os.Stat(filepath.Join(projectRoot, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func EnsureGoxMod(projectRoot string) error {
	goxModPath := filepath.Join(projectRoot, "gox.mod")
	content := defaultGoxModTemplate
	if data, err := os.ReadFile(goxModPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read gox.mod: %w", err)
	}

	updated := addGoxModImport(content, ModulePath)
	if updated == content {
		return nil
	}
	if err := os.WriteFile(goxModPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write gox.mod: %w", err)
	}
	return nil
}

func EnsureGoMod(projectRoot string, opts ProjectOptions) error {
	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	missing := os.IsNotExist(err)
	if missing {
		template := opts.GoModTemplate
		if template == "" {
			template = scaffold.GoMod()
		}
		data = []byte(template)
	} else if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	original := string(data)

	file, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}
	removeSpxXGoClassRequireComment(file)
	// AddRequire in pinned x/mod v0.42.0 only edits syntax and returns nil.
	_ = file.AddRequire(ModulePath, Version)
	if ReplacementPath != "" {
		ensureGoModReplace(file, ModulePath, ReplacementPath, ReplacementVersion)
	}
	if opts.FindSpxRoot != nil {
		if spxRoot := opts.FindSpxRoot(projectRoot); spxRoot != "" {
			relPath, err := filepath.Rel(projectRoot, spxRoot)
			if err != nil {
				return fmt.Errorf("failed to resolve local spx replace path: %w", err)
			}
			ensureGoModReplace(file, spxModulePath, filepath.ToSlash(relPath), "")
		}
	}
	file.Cleanup()

	content := formatGoModFile(file, original)
	if !missing && content == original {
		return nil
	}
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}
	return nil
}

func removeSpxXGoClassRequireComment(file *modfile.File) {
	const marker = "//xgo:class"

	for _, req := range file.Require {
		if req.Mod.Path != spxModulePath || req.Syntax == nil {
			continue
		}
		req.Syntax.Suffix = slices.DeleteFunc(req.Syntax.Suffix, func(comment modfile.Comment) bool {
			return strings.Contains(comment.Token, marker)
		})
	}
}

func ensureGoModReplace(file *modfile.File, modulePath, replacementPath, replacementVersion string) {
	// The pinned x/mod v0.42.0 replacement edits mutate syntax and always return nil.
	for _, repl := range file.Replace {
		if repl.Old.Path != modulePath || repl.Old.Version == "" {
			continue
		}
		_ = file.DropReplace(repl.Old.Path, repl.Old.Version)
	}
	_ = file.AddReplace(modulePath, "", replacementPath, replacementVersion)
}

func formatGoModFile(file *modfile.File, original string) string {
	content := modfile.Format(file.Syntax)
	if configLineEnding(original) == "\r\n" {
		return strings.ReplaceAll(string(content), "\n", "\r\n")
	}
	return string(content)
}

func addGoxModImport(content, modulePath string) string {
	importLine := `import "` + modulePath + `"`
	lines := splitConfigLines(content)
	for _, line := range lines {
		if strings.TrimSpace(line) == importLine {
			return content
		}
	}

	lineEnding := configLineEnding(content)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "project ") {
			return strings.Join(slices.Insert(lines, i+1, "", importLine), lineEnding)
		}
	}
	return appendConfigLine(content, importLine)
}

func configLineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func splitConfigLines(content string) []string {
	return strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
}

func appendConfigLine(content, line string) string {
	lineEnding := configLineEnding(content)
	switch {
	case content == "" || strings.HasSuffix(content, lineEnding+lineEnding):
		return content + line + lineEnding
	case strings.HasSuffix(content, lineEnding):
		return content + lineEnding + line + lineEnding
	default:
		return content + lineEnding + lineEnding + line + lineEnding
	}
}
