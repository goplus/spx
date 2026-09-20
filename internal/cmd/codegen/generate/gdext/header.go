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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/iancoleman/strcase"
)

var (
	// Braces in comments and literals do not change the surrounding C++ scope.
	reNonCode = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`)

	reSpaceComma = regexp.MustCompile(`\s+,`)
	reSpaceParen = regexp.MustCompile(`\s+\)`)
	reCommaSpace = regexp.MustCompile(`,\s*`)

	// Manager identity comes from its name and explicit exports, not its implementation base.
	reClassDefinition = regexp.MustCompile(`^class\s+(Spx\w+Mgr)(?:\s+final)?\s*(?::[^\{]+)?\s*\{`)

	// SPX_BIND exports one C++ declaration; static is ordinary C++ semantics.
	reMethod = regexp.MustCompile(`^SPX_BIND\s+(static\s+)?(\w+)\s+(\w+)\((.*)\);$`)
)

type classMethodDecl struct {
	ClassName  string
	ReturnType string
	MethodName string
	Params     string
	Static     bool
}

func (m classMethodDecl) functionName() string {
	return "GDExtension" + m.ClassName + strcase.ToCamel(m.MethodName)
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

func (g *headerCollector) render(rawFormat bool) string {
	var builder strings.Builder
	for _, method := range g.methods {
		if method.MethodName == "" {
			fmt.Fprintf(&builder, "// %s\n", method.ClassName)
			continue
		}
		returnType, params := method.ReturnType, method.Params
		functionName := method.functionName()
		if !rawFormat && returnType != "void" {
			if params != "" {
				params += ", "
			}
			result := g.metadata.ReturnParameters[functionName]
			params += result.CType + " *" + result.Name
			returnType = "void"
		}
		fmt.Fprintf(&builder, "typedef %s (*%s)(%s);\n", returnType, functionName, params)
	}
	return builder.String()
}

func (g *headerCollector) renderHeader(raw bool) (string, error) {
	text := strings.ReplaceAll(gdSpxExtH, "###MANAGER_FUNC_DEFINE", g.render(raw))
	output, err := common.RenderTemplate(template.FuncMap{"arrayTypes": common.ArrayTypes}, "gdextension_spx_ext.h", text, nil)
	return string(output), err
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
		switch filepath.Base(file) {
		case "spx_object_mgr.h":
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read SPX manager header %q: %w", file, err)
		}
		appendPublicDeclarations(&builder, string(source))
	}

	return builder.String(), nil
}

// appendPublicDeclarations collects direct public exports from manager classes.
// Nested types and inline bodies cannot change the manager's access section.
func appendPublicDeclarations(builder *strings.Builder, source string) {
	source = reNonCode.ReplaceAllStringFunc(source, func(text string) string {
		return " " + strings.Repeat("\n", strings.Count(text, "\n"))
	})
	depth, public := 0, false
	for line := range strings.SplitSeq(source, "\n") {
		line = strings.TrimSpace(line)
		if depth == 0 {
			if match := reClassDefinition.FindStringSubmatch(line); match != nil {
				fmt.Fprintf(builder, "class %s {\npublic:\n", match[1])
				depth = strings.Count(line, "{") - strings.Count(line, "}")
				public = false
			}
			continue
		}
		previousDepth := depth
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth == 0 {
			builder.WriteString("\n};\n\n")
			continue
		}
		if previousDepth != 1 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "public:"):
			public = true
		case strings.HasPrefix(line, "private:"), strings.HasPrefix(line, "protected:"):
			public = false
		case public && strings.HasPrefix(line, "SPX_BIND"):
			fmt.Fprintf(builder, "\t%s\n", line)
		}
	}
}

// normalizeParams normalizes whitespace around parameter separators.
func normalizeParams(params string) string {
	if params == "" {
		return params
	}
	// Remove whitespace before commas and closing parentheses.
	params = reSpaceComma.ReplaceAllString(params, ",")
	params = reSpaceParen.ReplaceAllString(params, ")")
	// Use one space after each comma.
	params = reCommaSpace.ReplaceAllString(params, ", ")
	return strings.TrimSpace(params)
}

func parseManagerHeader(input string) *headerCollector {
	g := &headerCollector{metadata: common.GenerationMetadata{
		ArrayBridges:     make(map[string]common.ArrayBridge),
		StaticMethods:    make(map[string]string),
		ReturnParameters: make(map[string]common.CParam),
	}}
	var currentClassName string

	for line := range strings.SplitSeq(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
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
			params := normalizeParams(matches[4])
			methodDecl := classMethodDecl{
				ClassName:  currentClassName,
				ReturnType: matches[2],
				MethodName: matches[3],
				Params:     params,
				Static:     matches[1] != "",
			}
			spec, arrayBridge := parseArrayBridge(methodDecl)
			functionName := methodDecl.functionName()
			if methodDecl.ReturnType != "void" {
				name := "ret_value"
				for n := 2; regexp.MustCompile(`\b` + name + `\b`).MatchString(methodDecl.Params); n++ {
					name = fmt.Sprintf("ret_value_%d", n)
				}
				g.metadata.ReturnParameters[functionName] = common.CParam{CType: methodDecl.ReturnType, Name: name}
			}
			if methodDecl.Static {
				g.metadata.StaticMethods[functionName] = currentClassName + "Mgr::" + methodDecl.MethodName
			}
			if arrayBridge {
				g.metadata.ArrayBridges[spec.FunctionName] = spec
				// Lower every fixed array to its pointer ABI after preserving its extent.
				var params []string
				for _, param := range spec.Params() {
					params = append(params, param.Declaration())
				}
				methodDecl.Params = strings.Join(params, ", ")
			}
			g.methods = append(g.methods, methodDecl)
		} else if strings.HasPrefix(line, "SPX_BIND") {
			panic("invalid SPX_BIND declaration: " + line)
		}
	}

	return g
}
