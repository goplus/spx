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

package command

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v2/internal/scaffold"
)

const (
	builderAIModule             = "github.com/goplus/builder/tools/ai"
	builderAIModuleVersion      = "v0.0.0-20260429075709-408b30d11459"
	builderAIDescriptionFile    = "builder-ai-description.md"
	builderAIDescriptionFileRaw = "builder-ai-description"
)

// Sync from repository root before building.
//
//go:generate cp ../../../../gox.mod gox.mod
//go:embed gox.mod
var defaultGoxModTemplate string

func (cmd *CmdTool) ensureBuilderAIModuleFiles(projectRoot string) error {
	if !hasBuilderAIDescription(projectRoot) {
		return nil
	}
	if err := ensureBuilderAIGoxMod(projectRoot); err != nil {
		return err
	}
	if err := cmd.ensureBuilderAIGoMod(projectRoot); err != nil {
		return err
	}
	return nil
}

func hasBuilderAIDescription(projectRoot string) bool {
	if projectRoot == "" {
		return false
	}
	for _, name := range []string{builderAIDescriptionFile, builderAIDescriptionFileRaw} {
		info, err := os.Stat(filepath.Join(projectRoot, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func ensureBuilderAIGoxMod(projectRoot string) error {
	goxModPath := filepath.Join(projectRoot, "gox.mod")
	content := defaultGoxModTemplate
	if data, err := os.ReadFile(goxModPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read gox.mod: %w", err)
	}

	updated := addGoxModImport(content, builderAIModule)
	if updated == content {
		return nil
	}
	if err := os.WriteFile(goxModPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write gox.mod: %w", err)
	}
	return nil
}

func (cmd *CmdTool) ensureBuilderAIGoMod(projectRoot string) error {
	goModPath := filepath.Join(projectRoot, "go.mod")
	if cmd.GoModTemplate == "" {
		cmd.GoModTemplate = scaffold.GoMod()
	}
	if err := cmd.createDefaultGoMod(projectRoot, false); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	original := string(data)
	content := original
	content = removeSpxXGoClassRequireComment(content)
	content = addGoModRequire(content, builderAIModule, builderAIModuleVersion)

	if spxRoot := findSpxRootFrom(projectRoot); spxRoot != "" {
		relPath, err := filepath.Rel(projectRoot, spxRoot)
		if err != nil {
			return fmt.Errorf("failed to resolve local spx replace path: %w", err)
		}
		content = ensureSpxModuleReplace(content, filepath.ToSlash(relPath))
	}

	if content == original {
		return nil
	}
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}
	return nil
}

func removeSpxXGoClassRequireComment(content string) string {
	const marker = "//xgo:class"

	lineEnding := configLineEnding(content)
	lines := splitConfigLines(content)
	changed := false
	for i, line := range lines {
		if !strings.Contains(line, marker) {
			continue
		}
		if !strings.Contains(line, "github.com/goplus/spx/v2 ") {
			continue
		}
		idx := strings.Index(line, marker)
		lines[i] = strings.TrimRight(line[:idx], " \t")
		changed = true
	}
	if !changed {
		return content
	}
	return strings.Join(lines, lineEnding)
}

func addGoModRequire(content, modulePath, version string) string {
	requireLine := modulePath + " " + version
	lineEnding := configLineEnding(content)
	lines := splitConfigLines(content)
	inRequireBlock := false
	firstRequireLine := -1
	firstRequireBlockClose := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			if firstRequireLine == -1 {
				firstRequireLine = i
			}
			inRequireBlock = true
		case inRequireBlock && trimmed == ")":
			if firstRequireBlockClose == -1 {
				firstRequireBlockClose = i
			}
			inRequireBlock = false
		case inRequireBlock:
			if isGoModRequireFor(trimmed, modulePath) {
				lines[i] = leadingWhitespace(line) + requireLine
				return strings.Join(lines, lineEnding)
			}
		case strings.HasPrefix(trimmed, "require "):
			if isGoModRequireFor(strings.TrimSpace(strings.TrimPrefix(trimmed, "require ")), modulePath) {
				lines[i] = leadingWhitespace(line) + "require " + requireLine
				return strings.Join(lines, lineEnding)
			}
			if firstRequireLine == -1 {
				firstRequireLine = i
			}
		}
	}

	if firstRequireBlockClose != -1 {
		return strings.Join(insertLines(lines, firstRequireBlockClose, "\t"+requireLine), lineEnding)
	}
	if firstRequireLine != -1 {
		return strings.Join(insertLines(lines, firstRequireLine+1, "require "+requireLine), lineEnding)
	}
	return appendConfigLine(content, "require "+requireLine)
}

func isGoModRequireFor(requireBody, modulePath string) bool {
	fields := strings.Fields(requireBody)
	return len(fields) >= 2 && fields[0] == modulePath
}

func leadingWhitespace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
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
