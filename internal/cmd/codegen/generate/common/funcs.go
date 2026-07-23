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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/internal/licenseheader"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"

	"github.com/iancoleman/strcase"
)

var (
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

func init() {
	// Set callback function so the clang package can get the list of known manager names
	clang.KnownManagerNamesProvider = func() []string {
		return KnownManagerNames
	}
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

var (
	managerSet            = map[string]bool{}
	cppType2Go            = map[string]string{}
	KnownManagerNames     = []string{} // List of correct manager names obtained from header parsing
	nativeArrayBridges    = map[string]NativeArrayBridgeSpec{}
	arrayTransformBridges = map[string]ArrayTransformBridgeSpec{}
)

type ManagerData struct {
	Ast     clang.CHeaderFileAST
	Mangers []string
}

// RegisterManagerName registers a known manager name (obtained from header parsing).
func RegisterManagerName(name string) {
	name = strings.ToLower(name)
	// Avoid duplicate entries
	for _, n := range KnownManagerNames {
		if n == name {
			return
		}
	}
	KnownManagerNames = append(KnownManagerNames, name)
}

// ClearKnownManagerNames clears the list of known manager names.
func ClearKnownManagerNames() {
	KnownManagerNames = []string{}
}

func ClearNativeArrayBridgeSpecs() {
	nativeArrayBridges = map[string]NativeArrayBridgeSpec{}
}

func ClearArrayTransformBridgeSpecs() {
	arrayTransformBridges = map[string]ArrayTransformBridgeSpec{}
}

func RegisterNativeArrayBridgeSpec(spec NativeArrayBridgeSpec) {
	nativeArrayBridges[spec.BaseFunctionName] = spec
}

func RegisterArrayTransformBridgeSpec(spec ArrayTransformBridgeSpec) {
	arrayTransformBridges[spec.FunctionName] = spec
}

func HasArrayTransformBridgeSpec(function *clang.TypedefFunction) bool {
	if function == nil {
		return false
	}
	_, ok := arrayTransformBridges[function.Name]
	return ok
}

func GetNativeArrayBridgeSpec(functionName string) (NativeArrayBridgeSpec, bool) {
	spec, ok := nativeArrayBridges[functionName]
	return spec, ok
}

func GetArrayTransformBridgeSpec(functionName string) (ArrayTransformBridgeSpec, bool) {
	spec, ok := arrayTransformBridges[functionName]
	return spec, ok
}

func ListArrayTransformBridgeSpecs() []ArrayTransformBridgeSpec {
	specs := make([]ArrayTransformBridgeSpec, 0, len(arrayTransformBridges))
	for _, spec := range arrayTransformBridges {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].FunctionName < specs[j].FunctionName
	})
	return specs
}

func HasNativeArrayBridgeSpec(function *clang.TypedefFunction) bool {
	if function == nil {
		return false
	}
	_, ok := GetNativeArrayBridgeSpec(function.Name)
	return ok
}

func IsNativeArrayBridgeArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	return IsNativeArrayDataArg(function, arg)
}

func IsNativeArrayDataArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.DataArgName || arg.Name == spec.BaseArgName
}

func IsNativeArrayLenArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.LenArgName && spec.LenArgName != ""
}

func ShouldSkipHighLevelArgument(function *clang.TypedefFunction, arg clang.Argument) bool {
	return IsNativeArrayLenArg(function, arg)
}

func EffectiveGoArgumentName(function *clang.TypedefFunction, arg clang.Argument) string {
	if IsNativeArrayDataArg(function, arg) {
		spec, _ := GetNativeArrayBridgeSpec(function.Name)
		return spec.BaseArgName
	}
	return arg.Name
}

func EffectiveGoArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	if IsNativeArrayDataArg(function, arg) {
		spec, _ := GetNativeArrayBridgeSpec(function.Name)
		return spec.DataArgGoType
	}
	return MustGoTypeForCType(MustPrimitiveTypeName(arg, function.Name), function.Name)
}

func EffectiveGdxArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	typeName := EffectiveGoArgumentType(function, arg)
	switch typeName {
	case "Object":
		return "gdx.Object"
	case "Array":
		return "gdx.Array"
	default:
		return typeName
	}
}

func NativeArrayLenExpr(function *clang.TypedefFunction, argName string) string {
	spec, _ := GetNativeArrayBridgeSpec(function.Name)
	return spec.LenArgGoType + "(len(" + argName + "))"
}

func GetManagerName(str string) string {
	prefix := "GDExtensionSpx"
	str = str[len(prefix):]
	lowerStr := strings.ToLower(str)

	// Prefer matching against known manager names (sorted by length descending, prioritizing longer names)
	if len(KnownManagerNames) > 0 {
		// Create a copy sorted by length in descending order
		sortedNames := make([]string, len(KnownManagerNames))
		copy(sortedNames, KnownManagerNames)
		sort.Slice(sortedNames, func(i, j int) bool {
			return len(sortedNames[i]) > len(sortedNames[j])
		})

		for _, mgr := range sortedNames {
			if strings.HasPrefix(lowerStr, mgr) {
				return mgr
			}
		}
	}

	// Fall back to the original logic (stop at uppercase letter)
	chs := []rune{}
	chs = append(chs, rune(str[0]), rune(str[1]))
	for _, ch := range str[2:] {
		if unicode.IsUpper(rune(ch)) {
			break
		}
		chs = append(chs, rune(ch))
	}
	result := strings.ToLower(string(chs))
	return result
}

func IsManagerMethod(function *clang.TypedefFunction) bool {
	return managerSet[GetManagerName(function.Name)]
}

func EffectiveArguments(function *clang.TypedefFunction) []clang.Argument {
	args := function.Arguments
	if function.ReturnType.Name == "void" && len(args) > 0 && args[len(args)-1].Name == "ret_value" {
		return args[:len(args)-1]
	}
	return args
}

func HighLevelArguments(function *clang.TypedefFunction) []clang.Argument {
	args := EffectiveArguments(function)
	result := make([]clang.Argument, 0, len(args))
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		cloned := arg
		cloned.Name = EffectiveGoArgumentName(function, arg)
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

func EffectiveGoReturnType(function *clang.TypedefFunction) string {
	rawType := EffectiveRawReturnType(function)
	if rawType == "" {
		return ""
	}
	return MustGoTypeForCType(rawType, function.Name)
}

func HasEffectiveReturn(function *clang.TypedefFunction) bool {
	return EffectiveRawReturnType(function) != ""
}

func GetFuncParamTypeString(typeName string) string {
	return cppType2Go[typeName]
}

func MustGoTypeForCType(typeName string, functionName string) string {
	goType := GetFuncParamTypeString(typeName)
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

func GetManagers(ast clang.CHeaderFileAST) []string {
	items := []string{}
	for _, item := range ast.CollectGDExtensionInterfaceFunctions() {
		items = append(items, item.Name)
	}
	managerSet = make(map[string]bool)
	managers := []string{}
	for _, str := range items {
		managerSet[GetManagerName(str)] = true
	}
	delete(managerSet, "")
	delete(managerSet, "string")
	delete(managerSet, "variant")
	delete(managerSet, "global")
	for item := range managerSet {
		managers = append(managers, item)
	}
	sort.Strings(managers)
	cppType2Go = map[string]string{
		"GdInt":    "int64",
		"GdFloat":  "float64",
		"GdObj":    "Object",
		"GdVec2":   "Vec2",
		"GdVec3":   "Vec3",
		"GdVec4":   "Vec4",
		"GdRect2":  "Rect2",
		"GdString": "string",
		"GdBool":   "bool",
		"GdColor":  "Color",
		"GdArray":  "Array",
	}
	return managers
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
	}

	dir := filepath.Dir(dstPath)
	os.MkdirAll(dir, os.ModePerm)
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	f.Write(output)
	f.Close()
	if isGoFile {
		exec.Command("go", "fmt", dstPath).Run()
		exec.Command("goimports", "-w", dstPath).Run()
	}
	spxlog.Info("Generated file: %s", dstPath)
	return nil
}
