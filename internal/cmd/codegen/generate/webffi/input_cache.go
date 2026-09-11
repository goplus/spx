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

package webffi

import (
	"fmt"
	"strings"
)

type managerFuncBodySpec struct {
	call      string
	fallback  []string
	boolAsInt bool
}

var inputCacheManagerFuncBodies = map[string]managerFuncBodySpec{
	"GDExtensionSpxInputGetGlobalMousePos": {
		call: "WebInputMousePos(func() Vec2",
		fallback: []string{
			"_retValue := API.SpxInputGetGlobalMousePos.Invoke()",
			"return JsToGdVec2(_retValue)",
		},
	},
	"GDExtensionSpxInputGetKey": {
		call: "WebInputKeyState(key, func() bool",
		fallback: []string{
			"arg0Low, arg0High := JsSplitGdInt(key)",
			"_retValue := API.SpxInputGetKey.Invoke(arg0Low, arg0High)",
			"return JsToGdBool(_retValue)",
		},
	},
	"GDExtensionSpxInputGetMouseState": {
		call: "WebInputMouseState(mouse_id, func() bool",
		fallback: []string{
			"arg0Low, arg0High := JsSplitGdInt(mouse_id)",
			"_retValue := API.SpxInputGetMouseState.Invoke(arg0Low, arg0High)",
			"return JsToGdBool(_retValue)",
		},
	},
	"GDExtensionSpxInputGetKeyState": {
		call: "WebInputKeyState(key, func() bool",
		fallback: []string{
			"arg0Low, arg0High := JsSplitGdInt(key)",
			"_retValue := API.SpxInputGetKeyState.Invoke(arg0Low, arg0High)",
			"return JsToGdInt(_retValue) != 0",
		},
		boolAsInt: true,
	},
	"GDExtensionSpxInputGetAxis": {
		call: "CachedActionAxis(neg_action, pos_action, func() float64",
		fallback: []string{
			"arg0 := JsFromGdString(neg_action)",
			"arg1 := JsFromGdString(pos_action)",
			"_retValue := API.SpxInputGetAxis.Invoke(arg0, arg1)",
			"return JsToGdFloat(_retValue)",
		},
	},
	"GDExtensionSpxInputIsActionPressed":      actionBoolManagerFuncBody("pressed", "API.SpxInputIsActionPressed"),
	"GDExtensionSpxInputIsActionJustPressed":  actionBoolManagerFuncBody("just_pressed", "API.SpxInputIsActionJustPressed"),
	"GDExtensionSpxInputIsActionJustReleased": actionBoolManagerFuncBody("just_released", "API.SpxInputIsActionJustReleased"),
}

func getInputCacheManagerFuncBody(name string) (string, bool) {
	spec, ok := inputCacheManagerFuncBodies[name]
	if !ok {
		return "", false
	}
	if spec.boolAsInt {
		return cachedBoolAsIntBody(spec.call, spec.fallback...), true
	}
	return cachedReturnBody(spec.call, spec.fallback...), true
}

func actionBoolManagerFuncBody(kind, api string) managerFuncBodySpec {
	return managerFuncBodySpec{
		call: fmt.Sprintf("CachedActionBool(%q, action, func() bool", kind),
		fallback: []string{
			"arg0 := JsFromGdString(action)",
			"_retValue := " + api + ".Invoke(arg0)",
			"return JsToGdBool(_retValue)",
		},
	}
}

func cachedReturnBody(call string, fallbackLines ...string) string {
	lines := []string{"return " + call + " {"}
	lines = appendIndented(lines, "\t\t", fallbackLines...)
	lines = append(lines, "\t})")
	return strings.Join(lines, "\n")
}

func cachedBoolAsIntBody(call string, fallbackLines ...string) string {
	lines := []string{"if " + call + " {"}
	lines = appendIndented(lines, "\t\t", fallbackLines...)
	lines = append(lines,
		"\t}) {",
		"\t\treturn 1",
		"\t}",
		"\treturn 0",
	)
	return strings.Join(lines, "\n")
}

func appendIndented(lines []string, indent string, values ...string) []string {
	for _, value := range values {
		lines = append(lines, indent+value)
	}
	return lines
}
