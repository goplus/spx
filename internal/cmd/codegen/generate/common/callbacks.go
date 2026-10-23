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

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
)

// Callback pairs an ABI struct field with its function signature.
type Callback struct {
	Field    string
	Function clang.TypedefFunction
}

// CollectCallbacks preserves SpxCallbackInfo order and rejects fields that
// cannot be initialized by native registration or engine fallback callbacks.
func CollectCallbacks(ast clang.CHeaderFileAST) ([]Callback, error) {
	functions := make(map[string]clang.TypedefFunction)
	for _, function := range ast.CollectGDExtensionICallbackFunctions() {
		functions[function.Name] = function
	}
	var callbackStruct *clang.TypedefStruct
	for _, expr := range ast.Expr {
		if expr.Struct != nil && expr.Struct.Name == "SpxCallbackInfo" {
			callbackStruct = expr.Struct
			break
		}
	}
	if callbackStruct == nil {
		return nil, fmt.Errorf("missing SpxCallbackInfo")
	}
	var callbacks []Callback
	for _, field := range callbackStruct.Fields {
		if field.Variable == nil {
			return nil, fmt.Errorf("unsupported inline callback field")
		}
		variable := field.Variable
		function, ok := functions[variable.Type.Name]
		if !ok || variable.Type.IsPointer || function.ReturnType.CStyleString() != "void" {
			return nil, fmt.Errorf("unsupported callback %s", variable.Name)
		}
		callbacks = append(callbacks, Callback{Field: variable.Name, Function: function})
	}
	return callbacks, nil
}
