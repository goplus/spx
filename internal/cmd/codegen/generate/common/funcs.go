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

package common

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/internal/licenseheader"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"

	"github.com/iancoleman/strcase"
)

const (
	NativeRelDir       = "../../gdengine/binding/native"
	GdengineImplRelDir = "../../gdengine/impl"
	EnginewrapRelDir   = "../../enginewrap"
	EnginePkgRelDir    = "../../../pkg/spx/pkg/engine"
)

type NativeArrayBridgeSpec struct {
	BaseFunctionName string
	BaseArgName      string

	DataArgName    string
	DataArgGoType  string
	DataArgPtrType string
	LenArgName     string
	LenArgGoType   string

	RawFunctionName string
	RawMethodName   string
	RawDataArgName  string
	RawDataCType    string
	RawLenArgName   string
	RawLenCType     string

	GoArgType     string
	FastArrayType int32
}

type RawExportParam struct {
	CType string
	Name  string
}

type ArrayTransformBridgeSpec struct {
	FunctionName     string
	ArrayArgName     string
	MethodName       string
	Params           []RawExportParam
	InputArrayType   int32
	OutputArrayType  int32
	OutputCountScale int
}

func Add(a int, b int) int {
	return a + b
}

func Sub(a int, b int) int {
	return a - b
}

func GoArgumentType(t clang.PrimativeType, name string) string {
	n := strings.TrimSpace(t.Name)

	hasReturnPrefix := strings.HasPrefix(name, "r_")

	switch n {
	case "void":
		if t.IsPointer {
			return "unsafe.Pointer"
		}
		return ""
	case "float", "real_t":
		if t.IsPointer {
			return "*float32"
		}
		return "float32"
	case "size_t":
		if t.IsPointer {
			panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
		}
		return "uint64"
	case "char":
		if t.IsPointer {
			if hasReturnPrefix {
				return "*Char"
			} else {
				return "string"
			}
		}
		panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
	case "int32_t":
		if t.IsPointer {
			panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
		}
		return "int32"
	case "char16_t":
		if t.IsPointer {
			return "*Char16T"
		}
		panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
	case "char32_t":
		if t.IsPointer {
			return "*Char32T"
		}
		return "Char32T"
	case "wchar_t":
		if t.IsPointer {
			return "*WcharT"
		}
		panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
	case "uint8_t":
		if t.IsPointer {
			return "*Uint8T"
		}
		return "Uint8T"
	case "int":
		if t.IsPointer {
			return "*int32"
		}
		return "int32"
	case "uint32_t":
		if t.IsPointer {
			return "*Uint32T"
		}
		return "Uint32T"
	case "uint64_t":
		if t.IsPointer {
			return "*Uint64T"
		}
		return "Uint64T"
	case "GdArray":
		if t.IsPointer {
			return "*GdArray"
		}
		return "GdArray"
	default:
		if t.IsPointer {
			return fmt.Sprintf("*%s", n)
		}
		return n
	}
}

func GoReturnType(t clang.PrimativeType) string {
	n := strings.TrimSpace(t.Name)

	switch n {
	case "float", "real_t":
		if t.IsPointer {
			return "*float32"
		} else {
			return "float32"
		}
	case "double":
		if t.IsPointer {
			return "*float32"
		} else {
			return "float32"
		}
	case "int32_t":
		if t.IsPointer {
			return "*int32"
		} else {
			return "int32"
		}
	case "int64_t":
		if t.IsPointer {
			return "*int64"
		} else {
			return "int64"
		}
	case "uint64_t":
		if t.IsPointer {
			return "*uint64"
		} else {
			return "uint64"
		}
	case "uint8_t":
		if t.IsPointer {
			return "*uint8"
		} else {
			return "uint8"
		}
	case "uint32_t":
		if t.IsPointer {
			return "*uint32"
		} else {
			return "uint32"
		}
	case "char16_t":
		if t.IsPointer {
			return "*Char16T"
		} else {
			return "Char16T"
		}
	case "char32_t":
		if t.IsPointer {
			return "*Char32T"
		} else {
			return "Char32T"
		}
	case "void":
		if t.IsPointer {
			return "unsafe.Pointer"
		} else {
			return ""
		}
	case "GdArray":
		if t.IsPointer {
			return "*GdArray"
		} else {
			return "GdArray"
		}
	default:
		if t.IsPointer {
			return fmt.Sprintf("*%s", n)
		} else {
			return n
		}
	}
}

func GoEnumValue(v clang.EnumValue, index int) string {
	if v.IntValue != nil {
		return strconv.Itoa(*v.IntValue)
	} else if v.ConstRefValue != nil {
		return *v.ConstRefValue
	} else if index == 0 {
		return "iota"
	} else {
		return ""
	}
}

func CgoCastArgument(a clang.Argument, defaultName string) string {
	if a.Type.Primative != nil {
		t := a.Type.Primative

		n := strings.TrimSpace(t.Name)

		var goVarName string

		if a.Name != "" {
			goVarName = a.Name
		} else {
			goVarName = defaultName
		}

		hasReturnPrefix := strings.HasPrefix(a.Name, "r_")

		switch n {
		case "void":
			if t.IsPointer {
				return fmt.Sprintf("unsafe.Pointer(%s)", goVarName)
			} else {
				panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
			}
		case "char":
			if t.IsPointer {
				if hasReturnPrefix {
					return fmt.Sprintf("(*C.char)(%s)", goVarName)
				} else {
					return fmt.Sprintf("C.CString(%s)", goVarName)
				}
			} else {
				panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
			}
		case "GdArray":
			if t.IsPointer {
				return fmt.Sprintf("(*C.GdArray)(%s)", goVarName)
			} else {
				return fmt.Sprintf("(C.GdArray)(%s)", goVarName)
			}
		default:
			if t.IsPointer {
				return fmt.Sprintf("(*C.%s)(%s)", n, goVarName)
			} else {
				return fmt.Sprintf("(C.%s)(%s)", n, goVarName)
			}
		}
	} else if a.Type.Function != nil {
		return fmt.Sprintf("(*[0]byte)(%s)", a.Type.Function.Name)
	}

	panic("unhandled type")
}

func CgoCleanUpArgument(a clang.Argument, index int) string {
	if a.Type.Primative != nil {
		t := a.Type.Primative
		n := strings.TrimSpace(t.Name)

		hasReturnPrefix := strings.HasPrefix(a.Name, "r_")

		switch n {
		case "char":
			if t.IsPointer {
				if !hasReturnPrefix {
					return fmt.Sprintf("C.free(unsafe.Pointer(arg%d))", index)
				}
				return ""

			} else {
				panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
			}
		default:
			return ""
		}
	} else if a.Type.Function != nil {
		return ""
	}

	panic("unhandled type")
}

func CgoCastReturnType(t clang.PrimativeType, argName string) string {
	n := strings.TrimSpace(t.Name)

	switch n {
	case "int32_t":
		if t.IsPointer {
			return fmt.Sprintf("(*int32)(%s)", argName)
		} else {
			return fmt.Sprintf("int32(%s)", argName)
		}
	case "uint32_t":
		if t.IsPointer {
			return fmt.Sprintf("(*uint32)(%s)", argName)
		} else {
			return fmt.Sprintf("uint32(%s)", argName)
		}
	case "int64_t":
		if t.IsPointer {
			return fmt.Sprintf("(*int64)(%s)", argName)
		} else {
			return fmt.Sprintf("int64(%s)", argName)
		}
	case "uint64_t":
		if t.IsPointer {
			return fmt.Sprintf("(*uint64)(%s)", argName)
		} else {
			return fmt.Sprintf("uint64(%s)", argName)
		}
	case "uint8_t":
		if t.IsPointer {
			return fmt.Sprintf("(*uint8)(%s)", argName)
		} else {
			return fmt.Sprintf("uint8(%s)", argName)
		}
	case "char16_t":
		if t.IsPointer {
			return fmt.Sprintf("(*Char16T)(%s)", argName)
		} else {
			panic(fmt.Sprintf("unhandled type: %s, %v", t.CStyleString(), t))
		}
	case "char32_t":
		if t.IsPointer {
			return fmt.Sprintf("(*Char32T)(%s)", argName)
		} else {
			panic(fmt.Sprintf("unhandled type: %s, %v", t.CStyleString(), t))
		}
	case "void":
		if t.IsPointer {
			return fmt.Sprintf("unsafe.Pointer(%s)", argName)
		} else {
			panic(fmt.Sprintf("unhandled type: %s", t.CStyleString()))
		}
	case "float", "real_t":
		if t.IsPointer {
			return fmt.Sprintf("(*float32)(%s)", argName)
		} else {
			return fmt.Sprintf("float32(%s)", argName)
		}
	case "double":
		if t.IsPointer {
			return fmt.Sprintf("(*float32)(%s)", argName)
		} else {
			return fmt.Sprintf("float32(%s)", argName)
		}
	case "GdArray":
		if t.IsPointer {
			return fmt.Sprintf("(*GdArray)(%s)", argName)
		} else {
			return fmt.Sprintf("GdArray(%s)", argName)
		}
	default:
		if t.IsPointer {
			return fmt.Sprintf("(*%s)(%s)", n, argName)
		} else {
			return fmt.Sprintf("(%s)(%s)", n, argName)
		}
	}
}

func GdiVariableName(typeName string) string {
	ret := LoadProcAddressName(typeName)
	ret = strcase.ToCamel(ret)
	ret = strings.Replace(ret, "C32Str", "C32str", 1)
	ret = strings.Replace(ret, "Placeholder", "PlaceHolder", 1)
	return ret
}

func GetManagerFuncName(typeName string) string {
	typeName = strings.Replace(typeName, "GDExtensionSpx", "", 1)
	return strings.Replace(LoadProcAddressName(typeName), "spx", "Call", 1)
}

func LoadProcAddressName(typeName string) string {
	ret := strcase.ToSnake(typeName)
	ret = strings.Replace(ret, "gd_extension_", "", 1)
	ret = strings.Replace(ret, "_latin_1_", "_latin1_", 1)
	ret = strings.Replace(ret, "_utf_8_", "_utf8_", 1)
	ret = strings.Replace(ret, "_utf_16_", "_utf16_", 1)
	ret = strings.Replace(ret, "_utf_32_", "_utf32_", 1)
	ret = strings.Replace(ret, "_c_32_str", "_c32str", 1)
	ret = strings.Replace(ret, "_float_32_", "_float32_", 1)
	ret = strings.Replace(ret, "_float_64_", "_float32_", 1)
	ret = strings.Replace(ret, "_int_16_", "_int16_", 1)
	ret = strings.Replace(ret, "_int_32_", "_int32_", 1)
	ret = strings.Replace(ret, "_int_64_", "_int64_", 1)
	ret = strings.Replace(ret, "_vector_2_", "_vector2_", 1)
	ret = strings.Replace(ret, "_vector_3_", "_vector3_", 1)
	ret = strings.Replace(ret, "_2", "2", 1)
	ret = strings.Replace(ret, "_3", "3", 1)
	ret = strings.Replace(ret, "_4", "4", 1)
	ret = strings.Replace(ret, "place_holder", "placeholder", 1)
	return ret
}

func TrimPrefix(typeName, prefix string) string {
	prefixLen := len(prefix)
	if strings.HasPrefix(typeName, prefix) {
		return typeName[prefixLen:]
	}
	return typeName
}

func (c *GenerationContext) IsNativeArrayBridgeArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	return c.IsNativeArrayDataArg(function, arg)
}

func (c *GenerationContext) IsNativeArrayDataArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.DataArgName || arg.Name == spec.BaseArgName
}

func (c *GenerationContext) IsNativeArrayLenArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.LenArgName && spec.LenArgName != ""
}

func (c *GenerationContext) ShouldSkipHighLevelArgument(function *clang.TypedefFunction, arg clang.Argument) bool {
	return c.IsNativeArrayLenArg(function, arg)
}

func (c *GenerationContext) EffectiveGoArgumentName(function *clang.TypedefFunction, arg clang.Argument) string {
	if c.IsNativeArrayDataArg(function, arg) {
		spec, _ := c.GetNativeArrayBridgeSpec(function.Name)
		return spec.BaseArgName
	}
	return arg.Name
}

func (c *GenerationContext) EffectiveGoArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	if c.IsNativeArrayDataArg(function, arg) {
		spec, _ := c.GetNativeArrayBridgeSpec(function.Name)
		return spec.DataArgGoType
	}
	return c.MustGoTypeForCType(MustPrimitiveTypeName(arg, function.Name), function.Name)
}

func (c *GenerationContext) EffectiveGdxArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	typeName := c.EffectiveGoArgumentType(function, arg)
	switch typeName {
	case "Object":
		return "gdx.Object"
	case "Array":
		return "gdx.Array"
	default:
		return typeName
	}
}

func (c *GenerationContext) NativeArrayLenExpr(function *clang.TypedefFunction, argName string) string {
	spec, _ := c.GetNativeArrayBridgeSpec(function.Name)
	return spec.LenArgGoType + "(len(" + argName + "))"
}

func EffectiveArguments(function *clang.TypedefFunction) []clang.Argument {
	args := function.Arguments
	if function.ReturnType.Name == "void" && len(args) > 0 && args[len(args)-1].Name == "ret_value" {
		return args[:len(args)-1]
	}
	return args
}

func (c *GenerationContext) HighLevelArguments(function *clang.TypedefFunction) []clang.Argument {
	args := EffectiveArguments(function)
	result := make([]clang.Argument, 0, len(args))
	for _, arg := range args {
		if c.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		cloned := arg
		cloned.Name = c.EffectiveGoArgumentName(function, arg)
		result = append(result, cloned)
	}
	return result
}

func EffectiveRawReturnType(function *clang.TypedefFunction) string {
	if function.ReturnType.Name != "void" {
		return function.ReturnType.Name
	}
	if len(function.Arguments) > 0 {
		last := function.Arguments[len(function.Arguments)-1]
		if last.Name == "ret_value" {
			if last.Type.Primative != nil {
				return last.Type.Primative.Name
			}
			panic(fmt.Sprintf("unsupported synthetic ret_value type in %s: %s", function.Name, last.Type.CStyleString()))
		}
	}
	return ""
}

func (c *GenerationContext) EffectiveGoReturnType(function *clang.TypedefFunction) string {
	rawType := EffectiveRawReturnType(function)
	if rawType == "" {
		return ""
	}
	return c.MustGoTypeForCType(rawType, function.Name)
}

func HasEffectiveReturn(function *clang.TypedefFunction) bool {
	return EffectiveRawReturnType(function) != ""
}

func (c *GenerationContext) GetFuncParamTypeString(typeName string) string {
	return c.cppType2Go[typeName]
}

func (c *GenerationContext) MustGoTypeForCType(typeName string, functionName string) string {
	goType := c.GetFuncParamTypeString(typeName)
	if goType != "" {
		return goType
	}
	panic(fmt.Sprintf("no Go mapping for C type %q in function %s", typeName, functionName))
}

func MustPrimitiveTypeName(arg clang.Argument, functionName string) string {
	if arg.Type.Primative != nil {
		return arg.Type.Primative.Name
	}
	panic(fmt.Sprintf("unsupported function-pointer argument %q in %s: %s", arg.Name, functionName, arg.Type.CStyleString()))
}

func GenerateFile(funcs template.FuncMap, name string, text string, data any, dstPath string) error {
	tmpl, err := template.New(name).
		Funcs(funcs).
		Parse(text)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, data)
	if err != nil {
		return err
	}
	output := b.Bytes()
	isGoFile := filepath.Ext(dstPath) == ".go"
	if isGoFile {
		output = licenseheader.AddToGoSource(output)
		output, err = format.Source(output)
		if err != nil {
			return fmt.Errorf("format generated Go file %q: %w", dstPath, err)
		}
	}

	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated file directory %q: %w", dir, err)
	}
	if err := os.WriteFile(dstPath, output, 0o644); err != nil {
		return fmt.Errorf("write generated file %q: %w", dstPath, err)
	}
	spxlog.Info("Generated file: %s", dstPath)
	return nil
}
