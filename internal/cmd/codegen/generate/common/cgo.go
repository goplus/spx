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
	"fmt"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"

	"github.com/iancoleman/strcase"
)

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
