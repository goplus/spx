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
	"regexp"
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

var reFixedArrayDecl = regexp.MustCompile(`^\s*(.+?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\[\s*([1-9][0-9]*)\s*\]\s*$`)

var reOutputMarker = regexp.MustCompile(`\bSPX_OUT\b`)

var reParamDecl = regexp.MustCompile(`^\s*(.+?[*\s])([A-Za-z_][A-Za-z0-9_]*)\s*$`)

func parseArrayBridge(method classMethodDecl) (common.ArrayBridge, bool) {
	spec := common.ArrayBridge{
		FunctionName: method.functionName(),
		MethodName:   method.MethodName,
	}
	params, buffers, ok := parseArrayBuffers(method.Params)
	if !ok {
		if strings.ContainsAny(method.Params, "[]") || reOutputMarker.MatchString(method.Params) {
			panic("native arrays require fixed declarations or pointer/length pairs: " + method.MethodName)
		}
		return common.ArrayBridge{}, false
	}
	if len(buffers) == 0 {
		return common.ArrayBridge{}, false
	}
	spec.Buffers, spec.Arguments = buffers, params
	if method.ReturnType != "void" && !(method.ReturnType == "GdBool" && spec.HasOutputOnly()) {
		panic("native array methods require void, or GdBool with SPX_OUT: " + method.MethodName)
	}
	seen := make(map[string]bool)
	for _, buffer := range spec.Buffers {
		if seen[buffer.ArgName()] {
			panic("duplicate high-level array parameter: " + buffer.ArgName())
		}
		seen[buffer.ArgName()] = true
	}
	return spec, true
}

func parseArrayType(rawType string) (common.ArrayType, bool) {
	elementType, ok := strings.CutSuffix(normalizeCType(rawType), "*")
	if !ok {
		return 0, false
	}
	elementType = strings.TrimSpace(elementType)
	elementType = strings.TrimPrefix(elementType, "const ")
	elementType = strings.TrimSuffix(elementType, " const")
	return common.LookupArrayType(elementType)
}

func isArrayLengthType(typeName string) bool {
	switch normalizeCType(typeName) {
	case "int", "int32_t":
		return true
	default:
		return false
	}
}

// Each buffer is either T name[N] or T *name followed by its own length.
// Parse both forms together so fixed and dynamic buffers compose identically.
func parseArrayBuffers(params string) ([]common.CParam, []common.ArrayBuffer, bool) {
	if strings.TrimSpace(params) == "" {
		return nil, nil, true
	}
	parts := strings.Split(params, ",")
	var arguments []common.CParam
	var buffers []common.ArrayBuffer
	for i := 0; i < len(parts); i++ {
		data, buffer, ok := parseArrayParameter(parts[i])
		if !ok {
			return nil, nil, false
		}
		arguments = append(arguments, data)
		if buffer == nil {
			continue
		}
		if buffer.Count == 0 {
			if i+1 == len(parts) {
				panic("native array requires an adjacent length: " + data.Name)
			}
			length, ok := parseCParam(parts[i+1])
			if !ok || !isArrayLengthType(length.CType) || reOutputMarker.MatchString(parts[i+1]) {
				panic("native array requires an int32 length: " + data.Name)
			}
			buffer.Length = length
			arguments = append(arguments, length)
			i++
		}
		buffers = append(buffers, *buffer)
	}
	return arguments, buffers, true
}

// parseArrayParameter resolves element type and direction once. Scalar parameters
// have no buffer; dynamic buffers receive their adjacent length in the caller.
func parseArrayParameter(param string) (common.CParam, *common.ArrayBuffer, bool) {
	param = strings.TrimSpace(param)
	var buffer common.ArrayBuffer
	fields := strings.Fields(param)
	if len(fields) > 0 && fields[0] == "SPX_OUT" {
		buffer.OutputOnly = true
		param = strings.TrimSpace(strings.TrimPrefix(param, "SPX_OUT"))
	}
	if reOutputMarker.MatchString(param) {
		panic("SPX_OUT must appear once at the start of an array parameter")
	}
	if match := reFixedArrayDecl.FindStringSubmatch(param); match != nil {
		buffer.Data = common.CParam{CType: normalizeCType(match[1] + " *"), Name: match[2]}
		buffer.Count = parseFixedArrayCount(match[3])
	} else {
		data, ok := parseCParam(param)
		if !ok {
			return common.CParam{}, nil, false
		}
		buffer.Data = data
	}
	arrayType, array := parseArrayType(buffer.Data.CType)
	if !array {
		if buffer.OutputOnly || buffer.Count != 0 {
			panic("native arrays require a supported element type: " + buffer.Data.Name)
		}
		return buffer.Data, nil, true
	}
	buffer.Type = arrayType
	if buffer.OutputOnly && !buffer.Writable() {
		panic("SPX_OUT cannot annotate a const array: " + buffer.Data.Name)
	}
	return buffer.Data, &buffer, true
}

func normalizeCType(cType string) string {
	cType = strings.Join(strings.Fields(cType), " ")
	if element, pointer := strings.CutSuffix(cType, "*"); pointer {
		if element, suffixConst := strings.CutSuffix(strings.TrimSpace(element), " const"); suffixConst {
			return "const " + element + " *"
		}
	}
	return cType
}

func parseCParam(param string) (common.CParam, bool) {
	match := reParamDecl.FindStringSubmatch(strings.TrimSpace(param))
	if match == nil {
		return common.CParam{}, false
	}
	return common.CParam{CType: normalizeCType(match[1]), Name: match[2]}, true
}

func parseFixedArrayCount(value string) int {
	count, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		panic("fixed array extent requires a positive int32: " + value)
	}
	return int(count)
}
