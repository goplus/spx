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

var (
	reSingleGdArrayParam   = regexp.MustCompile(`^\s*GdArray\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	reRawNativeArrayParams = regexp.MustCompile(`^\s*((?:const\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\*)\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*(int32_t|int)\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	reParamDecl            = regexp.MustCompile(`^\s*(.+?[*\s])([A-Za-z_][A-Za-z0-9_]*)\s*$`)
)

type arrayTransformOverride struct {
	ArrayArgName     string
	OutputCountScale int
}

var arrayTransformOverrides = map[string]arrayTransformOverride{
	// The raw signature does not encode the return length formula, so keep the
	// minimal transform metadata here and let codegen synthesize the high-level
	// GdArray return bridge generically.
	"batch_retrieve_positions": {
		ArrayArgName:     "objs",
		OutputCountScale: 2,
	},
}

func (g *headerCollector) registerNativeArrayBridgeSpecs(baseMethods map[string]classMethodDecl, rawMethods map[string]classMethodDecl) {
	for _, baseMethod := range baseMethods {
		dataType, dataArgName, lenType, lenArgName, goArgType, ptrType, lenGoType, fastArrayType, ok := parseRawNativeArrayParams(baseMethod.Params)
		if !ok {
			continue
		}

		baseFunctionName := "GDExtension" + baseMethod.ClassName + strcase.ToCamel(baseMethod.MethodName)
		g.metadata.NativeArrayBridges[baseFunctionName] = common.NativeArrayBridgeSpec{
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
		}
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

		g.metadata.NativeArrayBridges[baseFunctionName] = common.NativeArrayBridgeSpec{
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
		}
	}
}

func isArrayTransformBridgeMethod(methodName string) bool {
	_, ok := arrayTransformOverrides[methodName]
	return ok
}

func (g *headerCollector) registerArrayTransformBridgeSpecs(baseMethods map[string]classMethodDecl, rawMethods map[string]classMethodDecl) {
	for key, rawMethod := range rawMethods {
		baseMethod, hasBaseMethod := baseMethods[key]
		baseMethodName := strings.TrimSuffix(rawMethod.MethodName, "_raw")
		if rawMethod.ReturnType != "void" {
			continue
		}
		if hasBaseMethod && baseMethod.ReturnType != "GdArray" {
			continue
		}
		if _, _, _, _, _, _, _, _, ok := parseRawNativeArrayParams(rawMethod.Params); ok {
			continue
		}

		params, ok := parseRawExportParams(rawMethod.Params)
		if !ok || len(params) != 4 {
			continue
		}

		override, ok := arrayTransformOverrides[rawMethod.MethodName]
		if !ok {
			continue
		}

		baseArgName := override.ArrayArgName
		if hasBaseMethod {
			baseArgName, ok = parseSingleGdArrayParam(baseMethod.Params)
			if !ok {
				continue
			}
		} else if baseArgName == "" {
			baseArgName = highLevelArrayArgName(params[0].Name)
		}

		inputType, ok := rawArrayFastType(params[0].CType)
		if !ok || !isRawArrayLenType(params[1].CType) {
			continue
		}
		outputType, ok := rawArrayFastType(params[2].CType)
		if !ok || !isRawArrayLenType(params[3].CType) {
			continue
		}

		baseFunctionName := "GDExtension" + rawMethod.ClassName + strcase.ToCamel(baseMethodName)
		spec := common.ArrayTransformBridgeSpec{
			FunctionName:     baseFunctionName,
			ArrayArgName:     baseArgName,
			MethodName:       rawMethod.MethodName,
			Params:           params,
			InputArrayType:   inputType,
			OutputArrayType:  outputType,
			OutputCountScale: override.OutputCountScale,
		}
		g.metadata.ArrayTransformBridges[spec.FunctionName] = spec
	}
}

func (g *headerCollector) appendSyntheticArrayTransformTypedefs(builder *strings.Builder, rawFormat bool, baseMethods map[string]classMethodDecl) {
	existingFunctions := make(map[string]struct{}, len(baseMethods))
	for _, baseMethod := range baseMethods {
		functionName := "GDExtension" + baseMethod.ClassName + strcase.ToCamel(baseMethod.MethodName)
		existingFunctions[functionName] = struct{}{}
	}

	specs := make([]common.ArrayTransformBridgeSpec, 0, len(g.metadata.ArrayTransformBridges))
	for _, spec := range g.metadata.ArrayTransformBridges {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].FunctionName < specs[j].FunctionName })
	for _, spec := range specs {
		if _, ok := existingFunctions[spec.FunctionName]; ok {
			continue
		}
		if rawFormat {
			fmt.Fprintf(builder, "typedef GdArray (*%s)(GdArray %s);\n", spec.FunctionName, spec.ArrayArgName)
			continue
		}
		fmt.Fprintf(builder, "typedef void (*%s)(GdArray %s, GdArray *ret_value);\n", spec.FunctionName, spec.ArrayArgName)
	}
}

func rawArrayFastType(rawType string) (int32, bool) {
	switch strings.TrimPrefix(normalizeRawDataType(rawType), "const ") {
	case "int64_t *":
		return 1, true
	case "float *", "real_t *":
		return 2, true
	case "uint8_t *":
		return 5, true
	case "GdObj *":
		return 6, true
	default:
		return 0, false
	}
}

func isRawArrayLenType(typeName string) bool {
	switch normalizeRawDataType(typeName) {
	case "int", "int32_t":
		return true
	default:
		return false
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

func parseRawExportParams(params string) ([]common.RawExportParam, bool) {
	params = strings.TrimSpace(params)
	if params == "" {
		return nil, true
	}

	parts := strings.Split(params, ",")
	result := make([]common.RawExportParam, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		matches := reParamDecl.FindStringSubmatch(part)
		if len(matches) != 3 {
			return nil, false
		}
		result = append(result, common.RawExportParam{
			CType: normalizeRawDataType(matches[1]),
			Name:  matches[2],
		})
	}
	return result, true
}

func highLevelArrayArgName(rawDataArgName string) string {
	return strings.TrimSuffix(rawDataArgName, "_data")
}
