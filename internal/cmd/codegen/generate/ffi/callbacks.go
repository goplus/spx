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

package ffi

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
)

type callbackData struct {
	common.Callback
	Arguments []callbackArgumentData
}

type callbackArgumentData struct {
	CType, GoType, Value string
}

func writeCallbacks(codegenDir string, ast clang.CHeaderFileAST) error {
	declarations, err := common.CollectCallbacks(ast)
	if err != nil {
		return err
	}
	var callbacks []callbackData
	for _, declaration := range declarations {
		data := callbackData{Callback: declaration}
		for i, argument := range declaration.Function.Arguments {
			arg, err := callbackArgument(argument, fmt.Sprintf("arg%d", i))
			if err != nil {
				return fmt.Errorf("callback %s: %w", declaration.Field, err)
			}
			data.Arguments = append(data.Arguments, arg)
		}
		callbacks = append(callbacks, data)
	}
	if err := common.GenerateFile(template.FuncMap{"trimPrefix": strings.TrimPrefix}, "callbacks.gen.go", callbacksGoText, callbacks,
		filepath.Join(codegenDir, common.NativeRelDir, "callbacks.gen.go")); err != nil {
		return err
	}
	return common.GenerateFile(nil, "callbacks.gen.h", callbacksHeaderText, callbacks,
		filepath.Join(codegenDir, common.NativeRelDir, "callbacks.gen.h"))
}

func callbackArgument(argument clang.Argument, name string) (callbackArgumentData, error) {
	data := callbackArgumentData{CType: argument.Type.CStyleString()}
	switch data.CType {
	case "GdInt", "GdObj":
		data.GoType, data.Value = "C.GDExtensionInt", "int64("+name+")"
	case "GdFloat":
		data.GoType, data.Value = "C.GDReal", "float64("+name+")"
	case "GdBool":
		data.GoType, data.Value = "C.GDExtensionBool", name+" != 0"
	case "GdString":
		// Callback strings remain owned by the engine. ToString would free them.
		data.GoType, data.Value = "C.GdString", "copyString(GdString("+name+"))"
	default:
		return callbackArgumentData{}, fmt.Errorf("unsupported callback argument %s", data.CType)
	}
	return data, nil
}
