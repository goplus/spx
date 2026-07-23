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
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/goplus/spx/v3/internal/scaffold"
)

const (
	ModulePath         = "github.com/goplus/builder/tools/ai"
	DescriptionFile    = "builder-ai-description.md"
	DescriptionFileRaw = "builder-ai-description"
	spxModulePath      = "github.com/goplus/spx/v3"
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
	if err := EnsureGoMod(projectRoot, opts); err != nil {
		return err
	}
	return nil
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
	goModTemplate := opts.GoModTemplate
	if goModTemplate == "" {
		goModTemplate = scaffold.GoMod()
	}
	if err := createDefaultGoMod(projectRoot, goModTemplate, false); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	original := string(data)

	file, err := parseGoModFile(goModPath, data)
	if err != nil {
		return err
	}
	removeSpxXGoClassRequireComment(file)
	if err := file.AddRequire(ModulePath, Version); err != nil {
		return fmt.Errorf("failed to add builder ai require: %w", err)
	}
	if opts.FindSpxRoot != nil {
		if spxRoot := opts.FindSpxRoot(projectRoot); spxRoot != "" {
			relPath, err := filepath.Rel(projectRoot, spxRoot)
			if err != nil {
				return fmt.Errorf("failed to resolve local spx replace path: %w", err)
			}
			if err := ensureGoModReplace(file, spxModulePath, filepath.ToSlash(relPath)); err != nil {
				return err
			}
		}
	}
	file.Cleanup()

	content, err := formatGoModFile(file, original)
	if err != nil {
		return err
	}
	if content == original {
		return nil
	}
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}
	return nil
}

func createDefaultGoMod(dir, content string, forceWrite bool) error {
	goModPath := filepath.Join(dir, "go.mod")
	_, err := os.Stat(goModPath)
	if os.IsNotExist(err) || forceWrite {
		if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func parseGoModFile(path string, data []byte) (*modfile.File, error) {
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}
	return file, nil
}

func removeSpxXGoClassRequireComment(file *modfile.File) {
	const marker = "//xgo:class"

	for _, req := range file.Require {
		if req.Mod.Path != spxModulePath || req.Syntax == nil {
			continue
		}
		suffix := req.Syntax.Suffix[:0]
		for _, comment := range req.Syntax.Suffix {
			if strings.Contains(comment.Token, marker) {
				continue
			}
			suffix = append(suffix, comment)
		}
		req.Syntax.Suffix = suffix
	}
}

func ensureGoModReplace(file *modfile.File, modulePath, relPath string) error {
	for _, repl := range append([]*modfile.Replace(nil), file.Replace...) {
		if repl.Old.Path != modulePath {
			continue
		}
		if err := file.DropReplace(repl.Old.Path, repl.Old.Version); err != nil {
			return fmt.Errorf("failed to drop existing replace for %s: %w", modulePath, err)
		}
	}
	if err := file.AddReplace(modulePath, "", relPath, ""); err != nil {
		return fmt.Errorf("failed to add replace for %s: %w", modulePath, err)
	}
	return nil
}

func formatGoModFile(file *modfile.File, original string) (string, error) {
	content, err := file.Format()
	if err != nil {
		return "", fmt.Errorf("failed to format go.mod: %w", err)
	}
	if configLineEnding(original) == "\r\n" {
		return strings.ReplaceAll(string(content), "\n", "\r\n"), nil
	}
	return string(content), nil
}

func addGoxModImport(content, modulePath string) string {
	importLine := `import "` + modulePath + `"`
	if hasConfigLine(content, importLine) {
		return content
	}

	lineEnding := configLineEnding(content)
	lines := splitConfigLines(content)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "project ") {
			return strings.Join(insertLines(lines, i+1, "", importLine), lineEnding)
		}
	}
	return appendConfigLine(content, importLine)
}

func hasConfigLine(content, want string) bool {
	for _, line := range splitConfigLines(content) {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
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

func insertLines(lines []string, index int, values ...string) []string {
	out := make([]string, 0, len(lines)+len(values))
	out = append(out, lines[:index]...)
	out = append(out, values...)
	out = append(out, lines[index:]...)
	return out
}

func appendConfigLine(content, line string) string {
	lineEnding := configLineEnding(content)
	switch {
	case content == "":
		return line + lineEnding
	case strings.HasSuffix(content, lineEnding+lineEnding):
		return content + line + lineEnding
	case strings.HasSuffix(content, lineEnding):
		return content + lineEnding + line + lineEnding
	default:
		return content + lineEnding + lineEnding + line + lineEnding
	}
}
