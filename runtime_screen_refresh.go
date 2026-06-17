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

package spx

import (
	"fmt"
	"reflect"

	"github.com/goplus/spx/v2/internal/engine"
)

// IsRunWithoutScreenRefresh reports whether the current coroutine is in
// Scratch-style "run without screen refresh" mode.
func IsRunWithoutScreenRefresh() bool {
	return engine.IsRunWithoutScreenRefresh()
}

// SetRunWithoutScreenRefresh enables or disables Scratch-style
// "run without screen refresh" mode for the current coroutine.
//
// Prefer RunWithoutScreenRefresh for scoped execution.
//
// It returns the previous value so callers can restore nested state with:
// `prev := spx.SetRunWithoutScreenRefresh(true); defer spx.SetRunWithoutScreenRefresh(prev)`.
func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	return engine.SetRunWithoutScreenRefresh(enabled)
}

// RunWithoutScreenRefresh runs call in Scratch-style "run without screen refresh" mode and
// restores the previous mode when call returns.
func RunWithoutScreenRefresh(call func()) {
	engine.RunWithoutScreenRefresh(call)
}

// Warp calls fn with args in Scratch-style "run without screen refresh" mode.
func Warp(fn any, args ...any) {
	RunWithoutScreenRefresh(func() {
		call(fn, args...)
	})
}

func call(fn any, args ...any) {
	if fn == nil {
		return
	}
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		panic(fmt.Sprintf("spx: warp expects a function, got %T", fn))
	}
	if fv.IsNil() {
		return
	}

	ft := fv.Type()
	if !ft.IsVariadic() && len(args) != ft.NumIn() {
		panic(fmt.Sprintf("spx: warp argument count mismatch: got %d, want %d", len(args), ft.NumIn()))
	}
	if ft.IsVariadic() && len(args) < ft.NumIn()-1 {
		panic(fmt.Sprintf("spx: warp argument count mismatch: got %d, want at least %d", len(args), ft.NumIn()-1))
	}

	useCallSlice := false
	if ft.IsVariadic() && len(args) == ft.NumIn() {
		variadicType := ft.In(ft.NumIn() - 1)
		useCallSlice = canUseAs(args[len(args)-1], variadicType)
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		argType := callArgType(ft, i, useCallSlice)
		in[i] = callArgValue(arg, argType)
	}
	if useCallSlice {
		fv.CallSlice(in)
		return
	}
	fv.Call(in)
}

func callArgType(ft reflect.Type, idx int, useCallSlice bool) reflect.Type {
	if ft.IsVariadic() && idx >= ft.NumIn()-1 {
		variadicType := ft.In(ft.NumIn() - 1)
		if useCallSlice && idx == ft.NumIn()-1 {
			return variadicType
		}
		return variadicType.Elem()
	}
	return ft.In(idx)
}

func callArgValue(arg any, typ reflect.Type) reflect.Value {
	if arg == nil {
		if canBeNil(typ) {
			return reflect.Zero(typ)
		}
		panic(fmt.Sprintf("spx: cannot use nil as %s in warp call", typ))
	}
	value := reflect.ValueOf(arg)
	if value.Type().AssignableTo(typ) {
		return value
	}
	if value.Type().ConvertibleTo(typ) {
		return value.Convert(typ)
	}
	panic(fmt.Sprintf("spx: cannot use %s as %s in warp call", value.Type(), typ))
}

func canUseAs(arg any, typ reflect.Type) bool {
	if arg == nil {
		return false
	}
	argType := reflect.TypeOf(arg)
	return argType.AssignableTo(typ) || argType.ConvertibleTo(typ)
}

func canBeNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

func waitNextFrameForControlFlow() {
	if engine.ShouldWaitNextFrame() {
		engine.WaitNextFrame()
	}
}
