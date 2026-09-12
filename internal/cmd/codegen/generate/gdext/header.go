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

package gdext

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"
	"github.com/iancoleman/strcase"
)

var (
	reSpaceComma = regexp.MustCompile(`\s+,`)
	reSpaceParen = regexp.MustCompile(`\s+\)`)
	reCommaSpace = regexp.MustCompile(`,\s*`)

	reClassDefinition = regexp.MustCompile(`class\s+(\w+)\s*:\s*(?:public\s+)?(?:SpxBaseMgr|SpxObjectMgr<\w+>)(?:\s*,\s*[^\{]+)?\s*\{`)

	// Only methods explicitly marked with
	// SPX_API or SPX_BIND become part of the cross-language ABI.
	reMethod = regexp.MustCompile(`^\s*(?:SPX_API|SPX_BIND)\s+(\w+)\s+(\w+)\((.*)\);`)
)

type classMethodDecl struct {
	ClassName  string
	ReturnType string
	MethodName string
	Params     string
	Binding    bindingOptions
}

// Headers holds both ABI spellings and the metadata collected from one input.
type Headers struct {
	Raw      string
	Standard string
	Metadata common.GenerationMetadata
}

type headerCollector struct {
	metadata common.GenerationMetadata
	methods  []classMethodDecl
}

func PrepareHeaders(dir string) (Headers, error) {
	merged, err := mergeManagerHeader(dir)
	if err != nil {
		return Headers{}, err
	}
	h := parseManagerHeader(merged)
	raw, err := h.renderHeader(true)
	if err != nil {
		return Headers{}, err
	}
	standard, err := h.renderHeader(false)
	if err != nil {
		return Headers{}, err
	}
	return Headers{Raw: raw, Standard: standard, Metadata: h.metadata}, nil
}

func mergeManagerHeader(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "spx*mgr.h"))
	if err != nil {
		return "", fmt.Errorf("find SPX manager headers: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("#include \"gdextension_interface.h\"\n")
	builder.WriteString("#include \"gdextension_spx_mgr_pre_define.h\"\n")

	for _, file := range files {
		if strings.Contains(file, "spx_base_mgr.h") || strings.Contains(file, "spx_object_mgr.h") {
			continue
		}

		f, err := os.Open(file)
		if err != nil {
			return "", fmt.Errorf("open SPX manager header %q: %w", file, err)
		}

		var buffer bytes.Buffer
		scanner := bufio.NewScanner(f)
		className := ""
		inPublicSection := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "*/") {
				continue
			}
			if strings.HasPrefix(line, "};") {
				continue
			}
			// Skip inline function definitions (lines with both { and })
			if strings.Contains(line, "{") && strings.Contains(line, "}") {
				continue
			}

			if className == "" {
				match := reClassDefinition.FindStringSubmatch(line)
				if len(match) > 0 {
					className = match[1]
				} else {
					continue
				}
			}

			if strings.HasPrefix(line, "public:") {
				inPublicSection = true
				buffer.Reset()
				buffer.WriteString("public:\n")
				continue
			}

			if inPublicSection {
				fmt.Fprintf(&buffer, "\t%s\n", line)
			}
		}

		if className != "" {
			fmt.Fprintf(&builder, "class %s {\n", className)
			builder.WriteString(buffer.String())
			builder.WriteString("\n};\n\n")
		}

		scanErr := scanner.Err()
		closeErr := f.Close()
		if scanErr != nil {
			return "", fmt.Errorf("read SPX manager header %q: %w", file, scanErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close SPX manager header %q: %w", file, closeErr)
		}
	}

	return builder.String(), nil
}

// normalizeParams ensures proper spacing in parameter lists
func normalizeParams(params string) string {
	if params == "" {
		return params
	}
	// Remove trailing spaces before commas and closing paren
	params = reSpaceComma.ReplaceAllString(params, ",")
	params = reSpaceParen.ReplaceAllString(params, ")")
	// Ensure single space after commas
	params = reCommaSpace.ReplaceAllString(params, ", ")
	return strings.TrimSpace(params)
}

func parseManagerHeader(input string) *headerCollector {
	g := &headerCollector{metadata: common.GenerationMetadata{
		ArrayBridges: make(map[string]common.ArrayBridge),
		WebBindings:  make(map[string]common.WebBindingMode),
	}}
	scanner := bufio.NewScanner(strings.NewReader(input))
	var currentClassName string
	var pendingBinding *bindingOptions

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		options, declaration := parseBinding(line)
		if options != nil {
			if pendingBinding != nil {
				panic("duplicate SPX_BINDING annotation: " + line)
			}
			if declaration == "" {
				pendingBinding = options
				continue
			}
			line = declaration
		} else if pendingBinding != nil {
			options, pendingBinding = pendingBinding, nil
		}
		if options != nil && !reMethod.MatchString(line) {
			panic("SPX_BINDING must annotate an SPX_API or SPX_BIND method: " + line)
		}
		if strings.HasPrefix(line, "class ") {
			parts := strings.Fields(line)
			currentClassName = parts[1]
			currentClassName = currentClassName[:len(currentClassName)-3]
			if strings.HasPrefix(currentClassName, "Spx") {
				managerName := strings.ToLower(currentClassName[3:])
				if !slices.Contains(g.metadata.ManagerNames, managerName) {
					g.metadata.ManagerNames = append(g.metadata.ManagerNames, managerName)
				}
			}
			g.methods = append(g.methods, classMethodDecl{ClassName: currentClassName})
			continue
		}
		if matches := reMethod.FindStringSubmatch(line); matches != nil {
			params := normalizeParams(matches[3])
			methodDecl := classMethodDecl{
				ClassName:  currentClassName,
				ReturnType: matches[1],
				MethodName: matches[2],
				Params:     params,
			}
			if options != nil {
				methodDecl.Binding = *options
			}
			methodDecl.Binding.validate(methodDecl)
			functionName := "GDExtension" + currentClassName + strcase.ToCamel(methodDecl.MethodName)
			if mode := methodDecl.Binding.Web; mode != common.WebBindingDefault {
				g.metadata.WebBindings[functionName] = mode
			}
			if spec, ok := parseArrayBridge(methodDecl); ok {
				g.metadata.ArrayBridges[spec.FunctionName] = spec
			}
			if methodDecl.Binding.returnsArray() {
				continue
			}
			g.methods = append(g.methods, methodDecl)
		}
	}

	if err := scanner.Err(); err != nil {
		spxlog.Error("Error reading string: %v", err)
	}

	if pendingBinding != nil {
		panic("SPX_BINDING has no method declaration")
	}
	return g
}

func (g *headerCollector) render(rawFormat bool) string {
	var builder strings.Builder
	for _, method := range g.methods {
		if method.MethodName == "" {
			fmt.Fprintf(&builder, "// %s\n", method.ClassName)
			continue
		}
		returnType, params := method.ReturnType, method.Params
		name := strcase.ToCamel(method.MethodName)
		if returnType == "void" || rawFormat {
			fmt.Fprintf(&builder, "typedef %s (*GDExtension%s%s)(%s);\n", returnType, method.ClassName, name, params)
		} else {
			if len(params) > 0 {
				returnType = ", " + returnType
			}
			fmt.Fprintf(&builder, "typedef void (*GDExtension%s%s)(%s%s *ret_value);\n", method.ClassName, name, params, returnType)
		}
	}
	g.writeArrayTypedefs(&builder, rawFormat)
	return builder.String()
}

func (g *headerCollector) renderHeader(raw bool) (string, error) {
	text := strings.ReplaceAll(gdSpxExtH, "###MANAGER_FUNC_DEFINE", g.render(raw))
	output, err := common.RenderTemplate(template.FuncMap{"arrayTypes": common.ArrayTypes}, "gdextension_spx_ext.h", text, nil)
	return string(output), err
}
