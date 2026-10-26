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

// Package gdextensionparser prepares SPX headers for binding generation.
package gdextensionparser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/preprocessor"
)

// ParseGDExtensionInterface expands the parser headers with an in-memory source
// ABI. The published header contains a lowered ABI and must never be used as
// scratch space while preparing its replacement.
func ParseGDExtensionInterface(nativeDir, extensionHeader string) (clang.CHeaderFileAST, error) {
	source, err := readHeaders(nativeDir, extensionHeader)
	if err != nil {
		return clang.CHeaderFileAST{}, err
	}
	preprocFile, err := preprocessor.ParsePreprocessorString(source)
	if err != nil {
		return clang.CHeaderFileAST{}, fmt.Errorf("error preprocessing %s: %w", nativeDir, err)
	}
	ast, err := clang.ParseCString(preprocFile.Eval(false))
	if err != nil {
		return clang.CHeaderFileAST{}, fmt.Errorf("error parsing %s: %w", nativeDir, err)
	}
	return ast, nil
}

func readHeaders(dir, extensionHeader string) (string, error) {
	var allLines []string
	lines, err := readLines(filepath.Join(dir, "gdextension_spx_codegen_header.h"))
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "#include \"") {
			includePath := strings.ReplaceAll(strings.ReplaceAll(line, "#include \"", ""), "\"", "")
			var includeLines []string
			if includePath == "gdextension_spx_ext.h" {
				includeLines = strings.Split(extensionHeader, "\n")
			} else {
				includeLines, err = readLines(filepath.Join(dir, includePath))
				if err != nil {
					return "", err
				}
			}
			for _, inLine := range includeLines {
				if !strings.HasPrefix(inLine, "#include \"") {
					allLines = append(allLines, inLine)
				}
			}
		} else {
			allLines = append(allLines, line)
		}
	}

	var sb strings.Builder
	for _, line := range allLines {
		// Drop decorative separator lines from the concatenated headers.
		if strings.Contains(line, "/*******") {
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	source := sb.String()
	source = strings.ReplaceAll(source, "\r", "")
	return source, nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
