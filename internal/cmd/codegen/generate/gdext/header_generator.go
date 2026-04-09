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
	"strings"

	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"
	spxlog "github.com/goplus/spx/v2/internal/cmd/codegen/internal/log"
	"github.com/iancoleman/strcase"
)

// Pre-compiled regular expressions for better performance
var (
	// For normalizeParams function
	reSpaceComma = regexp.MustCompile(`\s+,`)
	reSpaceParen = regexp.MustCompile(`\s+\)`)
	reCommaSpace = regexp.MustCompile(`,\s*`)

	// For mergeManagerHeader function
	reClassDefinition = regexp.MustCompile(`class\s+(\w+)\s*:\s*(?:public\s+)?(?:SpxBaseMgr|SpxObjectMgr<\w+>)\s*{`)

	// For generateManagerHeader function
	reMethodVoid           = regexp.MustCompile(`\s*void\s+(\w+)\((.*)\);`)
	reMethodReturn         = regexp.MustCompile(`\s*(\w+)\s+(\w+)\((.*)\);`)
	reSingleGdArrayParam   = regexp.MustCompile(`^\s*GdArray\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	reRawNativeArrayParams = regexp.MustCompile(`^\s*((?:const\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\*)\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*(int32_t|int)\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)
)

func shouldSkipGeneratedMethod(methodName string) bool {
	// Raw helpers are web-only entry points and should not leak into the shared
	// gdextension interface consumed by native ffi/codegen.
	return strings.HasSuffix(methodName, "_raw")
}

type classMethodDecl struct {
	ClassName  string
	ReturnType string
	MethodName string
	Params     string
}

func generateSpxExtHeader(dir, outputFile string, isRawFormat bool) {
	mergedStr := mergeManagerHeader(dir)
	mergedHeaderFuncStr := generateManagerHeader(mergedStr, isRawFormat)
	finalHeader := strings.Replace(gdSpxExtH, "###MANAGER_FUNC_DEFINE", mergedHeaderFuncStr, -1)
	// Write the final header file
	f, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	f.Write([]byte(finalHeader))
	f.Close()
}

func mergeManagerHeader(dir string) string {
	files, err := filepath.Glob(filepath.Join(dir, "spx*mgr.h"))
	if err != nil {
		spxlog.Error("Error finding files: %v", err)
		return ""
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
			spxlog.Error("Error opening file: %v", err)
			continue
		}
		defer f.Close()

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
				buffer.WriteString("\t" + line + "\n")
			}
		}

		if className != "" {
			builder.WriteString(fmt.Sprintf("class %s {\n", className))
			builder.WriteString(buffer.String())
			builder.WriteString("\n};\n\n")
		}

		if err := scanner.Err(); err != nil {
			spxlog.Error("Error reading file: %v", err)
		}
	}

	return builder.String()
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
	// Trim any leading/trailing whitespace
	return strings.TrimSpace(params)
}

func generateManagerHeader(input string, rawFormat bool) string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	var currentClassName string

	var builder strings.Builder

	// Clear the previous list of manager names
	common.ClearKnownManagerNames()
	common.ClearNativeArrayBridgeSpecs()

	baseMethods := map[string]classMethodDecl{}
	rawMethods := map[string]classMethodDecl{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "class") {
			parts := strings.Fields(line)
			currentClassName = parts[1]
			currentClassName = currentClassName[:len(currentClassName)-3]
			// Register manager name (remove "Spx" prefix)
			if strings.HasPrefix(currentClassName, "Spx") {
				managerName := currentClassName[3:] // Remove "Spx" prefix
				common.RegisterManagerName(managerName)
			}
			builder.WriteString("// " + currentClassName + "\n")
			continue
		}
		if reMethodVoid.MatchString(line) {
			matches := reMethodVoid.FindStringSubmatch(line)
			params := normalizeParams(matches[2])
			methodDecl := classMethodDecl{
				ClassName:  currentClassName,
				ReturnType: "void",
				MethodName: matches[1],
				Params:     params,
			}
			if shouldSkipGeneratedMethod(matches[1]) {
				rawMethods[currentClassName+"::"+strings.TrimSuffix(matches[1], "_raw")] = methodDecl
				continue
			}
			baseMethods[currentClassName+"::"+matches[1]] = methodDecl
			methodName := strcase.ToCamel(matches[1])
			builder.WriteString(fmt.Sprintf("typedef void (*GDExtension%s%s)(%s);\n", currentClassName, methodName, params))
		} else if reMethodReturn.MatchString(line) {
			matches := reMethodReturn.FindStringSubmatch(line)
			params := normalizeParams(matches[3])
			methodDecl := classMethodDecl{
				ClassName:  currentClassName,
				ReturnType: matches[1],
				MethodName: matches[2],
				Params:     params,
			}
			if shouldSkipGeneratedMethod(matches[2]) {
				rawMethods[currentClassName+"::"+strings.TrimSuffix(matches[2], "_raw")] = methodDecl
				continue
			}
			baseMethods[currentClassName+"::"+matches[2]] = methodDecl
			returnType := matches[1]
			methodName := strcase.ToCamel(matches[2])
			if rawFormat {
				builder.WriteString(fmt.Sprintf("typedef %s (*GDExtension%s%s)(%s);\n", returnType, currentClassName, methodName, params))
			} else {
				if len(params) > 0 {
					returnType = ", " + returnType
				}
				builder.WriteString(fmt.Sprintf("typedef void (*GDExtension%s%s)(%s%s *ret_value);\n", currentClassName, methodName, params, returnType))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		spxlog.Error("Error reading string: %v", err)
	}

	registerNativeArrayBridgeSpecs(baseMethods, rawMethods)

	return builder.String()
}

func registerNativeArrayBridgeSpecs(baseMethods map[string]classMethodDecl, rawMethods map[string]classMethodDecl) {
	for _, baseMethod := range baseMethods {
		dataType, dataArgName, lenType, lenArgName, goArgType, ptrType, lenGoType, fastArrayType, ok := parseRawNativeArrayParams(baseMethod.Params)
		if !ok {
			continue
		}

		baseFunctionName := "GDExtension" + baseMethod.ClassName + strcase.ToCamel(baseMethod.MethodName)
		common.RegisterNativeArrayBridgeSpec(common.NativeArrayBridgeSpec{
			BaseFunctionName: baseFunctionName,
			BaseArgName:      highLevelArrayArgName(dataArgName),
			DataArgName:      dataArgName,
			DataArgGoType:    goArgType,
			DataArgPtrType:   ptrType,
			LenArgName:       lenArgName,
			LenArgGoType:     lenGoType,
			GoArgType:        goArgType,
			RawFunctionName:  baseFunctionName,
			RawMethodName:    baseMethod.MethodName,
			RawDataArgName:   dataArgName,
			RawDataCType:     dataType,
			RawLenArgName:    lenArgName,
			RawLenCType:      lenType,
			FastArrayType:    fastArrayType,
		})
	}

	for key, rawMethod := range rawMethods {
		baseMethod, ok := baseMethods[key]
		if !ok || baseMethod.ReturnType != rawMethod.ReturnType {
			continue
		}

		baseArgName, ok := parseSingleGdArrayParam(baseMethod.Params)
		if !ok {
			continue
		}

		rawDataType, rawDataArgName, rawLenType, rawLenArgName, goArgType, ptrType, lenGoType, fastArrayType, ok := parseRawNativeArrayParams(rawMethod.Params)
		if !ok {
			continue
		}

		baseFunctionName := "GDExtension" + rawMethod.ClassName + strcase.ToCamel(baseMethod.MethodName)
		rawFunctionName := "GDExtension" + rawMethod.ClassName + strcase.ToCamel(rawMethod.MethodName)

		common.RegisterNativeArrayBridgeSpec(common.NativeArrayBridgeSpec{
			BaseFunctionName: baseFunctionName,
			BaseArgName:      baseArgName,
			DataArgName:      baseArgName,
			DataArgGoType:    goArgType,
			DataArgPtrType:   ptrType,
			LenArgName:       rawLenArgName,
			LenArgGoType:     lenGoType,
			GoArgType:        goArgType,
			RawFunctionName:  rawFunctionName,
			RawMethodName:    rawMethod.MethodName,
			RawDataArgName:   rawDataArgName,
			RawDataCType:     rawDataType,
			RawLenArgName:    rawLenArgName,
			RawLenCType:      rawLenType,
			FastArrayType:    fastArrayType,
		})
	}
}

func parseSingleGdArrayParam(params string) (string, bool) {
	matches := reSingleGdArrayParam.FindStringSubmatch(params)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func parseRawNativeArrayParams(params string) (rawDataType string, rawDataArgName string, rawLenType string, rawLenArgName string, goArgType string, ptrType string, lenGoType string, fastArrayType int32, ok bool) {
	matches := reRawNativeArrayParams.FindStringSubmatch(params)
	if len(matches) != 5 {
		return "", "", "", "", "", "", "", 0, false
	}

	rawDataType = normalizeRawDataType(matches[1])
	rawDataArgName = matches[2]
	rawLenType = matches[3]
	rawLenArgName = matches[4]

	switch strings.TrimPrefix(rawDataType, "const ") {
	case "float *":
		return rawDataType, rawDataArgName, rawLenType, rawLenArgName, "[]float32", "*float32", "int32", 2, true
	case "real_t *":
		return rawDataType, rawDataArgName, rawLenType, rawLenArgName, "[]float32", "*float32", "int32", 2, true
	case "int64_t *":
		return rawDataType, rawDataArgName, rawLenType, rawLenArgName, "[]int64", "*int64", "int32", 1, true
	case "uint8_t *":
		return rawDataType, rawDataArgName, rawLenType, rawLenArgName, "[]byte", "*uint8", "int32", 5, true
	default:
		return "", "", "", "", "", "", "", 0, false
	}
}

func normalizeRawDataType(rawDataType string) string {
	rawDataType = strings.Join(strings.Fields(rawDataType), " ")
	rawDataType = strings.ReplaceAll(rawDataType, " *", " *")
	return rawDataType
}

func highLevelArrayArgName(rawDataArgName string) string {
	if strings.HasSuffix(rawDataArgName, "_data") {
		return strings.TrimSuffix(rawDataArgName, "_data")
	}
	return rawDataArgName
}
