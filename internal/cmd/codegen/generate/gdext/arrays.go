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
	"regexp"
	"sort"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/iancoleman/strcase"
)

var reParamDecl = regexp.MustCompile(`^\s*(.+?[*\s])([A-Za-z_][A-Za-z0-9_]*)\s*$`)

func parseArrayBridge(method classMethodDecl) (common.ArrayBridge, bool) {
	spec := common.ArrayBridge{
		FunctionName: "GDExtension" + method.ClassName + strcase.ToCamel(method.MethodName),
		MethodName:   method.MethodName,
	}
	if method.Binding.returnsArray() {
		params, ok := parseCParams(method.Params)
		if method.ReturnType != "void" || !ok || len(params) != 4 {
			panic("SPX_BINDING array transform requires void(input pointer, count, output pointer, capacity): " + method.MethodName)
		}
		input, inputOK := parseArrayBuffer(params[:2])
		output, outputOK := parseArrayBuffer(params[2:])
		if !inputOK || !outputOK || strings.HasPrefix(output.Data.CType, "const ") {
			panic("invalid SPX_BINDING array transform buffers: " + method.MethodName)
		}
		output.ElementsPerInput = method.Binding.ElementsPerInput
		spec.ArgName = method.Binding.ArrayArg
		spec.Input, spec.Output, spec.ReturnArray = &input, &output, true
	} else {
		buffer, ok := parseCallerBuffer(method.Params)
		if !ok {
			return common.ArrayBridge{}, false
		}
		spec.ArgName = strings.TrimSuffix(buffer.Data.Name, "_data")
		if strings.HasPrefix(buffer.Data.CType, "const ") {
			spec.Input = &buffer
		} else {
			buffer.Count = method.Binding.OutputCount
			spec.Output = &buffer
		}
	}
	return spec, true
}

func (g *headerCollector) writeArrayTypedefs(builder *strings.Builder, rawFormat bool) {
	specs := make([]common.ArrayBridge, 0, len(g.metadata.ArrayBridges))
	for _, spec := range g.metadata.ArrayBridges {
		if spec.ReturnArray {
			specs = append(specs, spec)
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].FunctionName < specs[j].FunctionName })
	for _, spec := range specs {
		if rawFormat {
			fmt.Fprintf(builder, "typedef GdArray (*%s)(GdArray %s);\n", spec.FunctionName, spec.ArgName)
			continue
		}
		fmt.Fprintf(builder, "typedef void (*%s)(GdArray %s, GdArray *ret_value);\n", spec.FunctionName, spec.ArgName)
	}
}

func parseArrayType(rawType string) (common.ArrayType, bool) {
	elementType, ok := strings.CutSuffix(normalizeCType(rawType), "*")
	if !ok {
		return 0, false
	}
	return common.LookupArrayType(strings.TrimSpace(strings.TrimPrefix(elementType, "const ")))
}

func isArrayLengthType(typeName string) bool {
	switch normalizeCType(typeName) {
	case "int", "int32_t":
		return true
	default:
		return false
	}
}

func parseArrayBuffer(params []common.CParam) (common.ArrayBuffer, bool) {
	if len(params) != 2 || !isArrayLengthType(params[1].CType) {
		return common.ArrayBuffer{}, false
	}
	arrayType, ok := parseArrayType(params[0].CType)
	return common.ArrayBuffer{Data: params[0], Length: params[1], Type: arrayType}, ok
}

func parseCallerBuffer(params string) (common.ArrayBuffer, bool) {
	parsed, ok := parseCParams(params)
	if !ok {
		return common.ArrayBuffer{}, false
	}
	buffer, ok := parseArrayBuffer(parsed)
	// GdObj buffers are exposed through GdArray transforms, not direct Go slices.
	return buffer, ok && buffer.Type != common.ArrayObject
}

func normalizeCType(cType string) string {
	return strings.Join(strings.Fields(cType), " ")
}

func parseCParams(params string) ([]common.CParam, bool) {
	params = strings.TrimSpace(params)
	if params == "" {
		return nil, true
	}

	parts := strings.Split(params, ",")
	result := make([]common.CParam, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		matches := reParamDecl.FindStringSubmatch(part)
		if len(matches) != 3 {
			return nil, false
		}
		result = append(result, common.CParam{
			CType: normalizeCType(matches[1]),
			Name:  matches[2],
		})
	}
	return result, true
}
