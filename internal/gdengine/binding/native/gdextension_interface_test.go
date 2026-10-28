//go:build cgo

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
	"math"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/gdengine/binding/native/testdata/stringallocator"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type sceneMainEntered struct{}

var expectSceneMain bool

func TestMain(m *testing.M) {
	// Scene initialization calls main.main through go:linkname. Stop that
	// reentry here so it can be asserted without running the suite recursively.
	if expectSceneMain {
		panic(sceneMainEntered{})
	}
	os.Exit(m.Run())
}

func TestToGdArrayInfoPreservesTypedEmptyArrays(t *testing.T) {
	tests := []struct {
		name     string
		empty    any
		nilValue any
		wantType int64
		toSlice  func(*ArrayInfoImpl) any
	}{
		{
			name:     "int64",
			empty:    []int64{},
			nilValue: []int64(nil),
			wantType: ArrayTypeInt64,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToInt64s() },
		},
		{
			name:     "float32",
			empty:    []float32{},
			nilValue: []float32(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToFloats() },
		},
		{
			name:     "float64",
			empty:    []float64{},
			nilValue: []float64(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToFloats() },
		},
		{
			name:     "bool",
			empty:    []bool{},
			nilValue: []bool(nil),
			wantType: ArrayTypeBool,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToBools() },
		},
		{
			name:     "string",
			empty:    []string{},
			nilValue: []string(nil),
			wantType: ArrayTypeString,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToStrings() },
		},
		{
			name:     "object",
			empty:    []GdObj{},
			nilValue: []GdObj(nil),
			wantType: ArrayTypeGdObj,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToObjects() },
		},
		{
			name:     "byte",
			empty:    []byte{},
			nilValue: []byte(nil),
			wantType: ArrayTypeByte,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToBytes() },
		},
	}

	for _, test := range tests {
		t.Run(test.name+"/empty", func(t *testing.T) {
			info := ToGdArrayInfo(test.empty)
			if info == nil {
				t.Fatal("ToGdArrayInfo() returned nil")
			}
			defer info.Free()

			if info.Raw() == nil {
				t.Fatal("ToGdArrayInfo().Raw() returned nil for a typed empty array")
			}
			if got := info.Size(); got != 0 {
				t.Fatalf("ToGdArrayInfo().Size() = %d, want 0", got)
			}
			if got := info.Type(); got != test.wantType {
				t.Fatalf("ToGdArrayInfo().Type() = %d, want %d", got, test.wantType)
			}

			converted := reflect.ValueOf(test.toSlice(info))
			if converted.Kind() != reflect.Slice || converted.IsNil() || converted.Len() != 0 {
				t.Fatalf("converted value = %#v, want a non-nil empty slice", converted.Interface())
			}
		})

		t.Run(test.name+"/nil", func(t *testing.T) {
			info := ToGdArrayInfo(test.nilValue)
			if info == nil {
				t.Fatal("ToGdArrayInfo() returned nil")
			}
			defer info.Free()

			if info.Raw() != nil {
				t.Fatal("ToGdArrayInfo().Raw() returned non-nil for a typed nil slice")
			}
		})
	}
}

func TestNativeArrayValuesAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input, want any
		kind        int64
	}{
		{"integers", []int64{math.MinInt64, 0, math.MaxInt64}, []int64{math.MinInt64, 0, math.MaxInt64}, ArrayTypeInt64},
		{"float32", []float32{-1.5, 0, 2.25}, []float32{-1.5, 0, 2.25}, ArrayTypeFloat},
		{"float64", []float64{-1.5, 0, 2.25}, []float32{-1.5, 0, 2.25}, ArrayTypeFloat},
		{"booleans", []bool{true, false, true}, []bool{true, false, true}, ArrayTypeBool},
		{"bytes", []byte{0, 127, 255}, []byte{0, 127, 255}, ArrayTypeByte},
		{"objects", []GdObj{-1, 0, 1<<54 + 3}, []int64{-1, 0, 1<<54 + 3}, ArrayTypeGdObj},
		{"strings", []string{"", "héllo", "two words"}, []string{"", "héllo", "two words"}, ArrayTypeString},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := ToGdArrayInfo(tc.input)
			defer info.Free()
			if info.Size() != 3 || info.Type() != tc.kind {
				t.Fatalf("array metadata = %d/%d", info.Size(), info.Type())
			}
			readers := map[int64]func() any{
				ArrayTypeInt64: func() any { return info.ToInt64s() }, ArrayTypeFloat: func() any { return info.ToFloats() },
				ArrayTypeBool: func() any { return info.ToBools() }, ArrayTypeByte: func() any { return info.ToBytes() },
				ArrayTypeGdObj: func() any { return info.ToObjects() }, ArrayTypeString: func() any { return info.ToStrings() },
			}
			for kind, read := range readers {
				got := read()
				if kind == tc.kind {
					if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("decoded = %#v, want %#v", got, tc.want)
					}
				} else if !reflect.ValueOf(got).IsNil() {
					t.Fatalf("wrong type %d decoded as %#v", kind, got)
				}
			}
			// Each read is a Go-owned copy; changing it must not alter native storage.
			reflect.ValueOf(readers[tc.kind]()).Clear()
			if got := readers[tc.kind](); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("native buffer aliased Go copy: %#v", got)
			}
			info.Free()
			info.Free()
			if info.Raw() != nil || info.Size() != 0 || info.Type() != 0 {
				t.Fatal("freed array retained native storage")
			}
			for _, read := range readers {
				if !reflect.ValueOf(read()).IsNil() {
					t.Fatal("read from freed array returned data")
				}
			}
			// ToArray consumes the owned return buffer; its result survives that free.
			if got := ToArray(ToGdArrayInfo(tc.input).Raw()); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("owned return = %#v, want %#v", got, tc.want)
			}
		})
	}
	if (*ArrayInfoImpl)(nil).Raw() != nil || ToArray(nil) != nil || ToGdArrayInfo([]byte(nil)).Raw() != nil {
		t.Fatal("nil array conversion was not nil")
	}
	for _, input := range []any{nil, 42, []int{1}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("unsupported array %T accepted", input)
				}
			}()
			ToGdArrayInfo(input)
		}()
	}
}

func TestToArrayRejectsUnknownNativeType(t *testing.T) {
	info := allocateGdArrayInfo(-1, 0, false)
	if info == nil || info.Raw() == nil {
		t.Fatal("failed to allocate empty native array")
	}
	// ToArray takes ownership of this buffer; do not also call info.Free.
	if got := ToArray(info.Raw()); got != nil {
		t.Fatalf("unknown native array type decoded as %#v, want nil", got)
	}
}

func TestNativeScalarConversions(t *testing.T) {
	for _, value := range []bool{false, true} {
		if ToBool(ToGdBool(value)) != value {
			t.Fatalf("boolean round trip %v", value)
		}
	}
	if !ToBool(2) || ToString(nil) != "" || copyString(nil) != "" {
		t.Fatal("noncanonical boolean or nil string conversion")
	}
	for _, value := range []int64{math.MinInt64, -1, 0, 1<<54 + 3, math.MaxInt64} {
		if ToInt64(ToGdInt(value)) != value || int64(ToObject(ToGdObj(engine.Object(value)))) != value {
			t.Fatalf("integer precision lost: %d", value)
		}
	}
	for _, value := range []float64{-1.5, 0, 2.25} {
		gd := ToGdFloat(value)
		if ToFloat64(gd) != value {
			t.Fatalf("float round trip %v", value)
		}
	}
	vec2, vec4 := mathf.NewVec2(-1.5, 2.25), mathf.NewVec4(1, 2, 3, 4)
	color := mathf.NewColor(.25, .5, .75, 1)
	rect := mathf.Rect2{Position: vec2, Size: mathf.NewVec2(40, 50)}
	if ToVec2(ToGdVec2(vec2)) != vec2 || ToVec4(ToGdVec4(vec4)) != vec4 || ToColor(ToGdColor(color)) != color || ToRect2(ToGdRect2(rect)) != rect {
		t.Fatal("structured value round trip changed components")
	}
}

func TestReturnedStringsUseRuntimeAllocator(t *testing.T) {
	previous := builtinAPI.SpxGlobalFreeString
	builtinAPI.SpxGlobalFreeString = GDExtensionSpxGlobalFreeString(stringallocator.FreeCallback())
	t.Cleanup(func() { builtinAPI.SpxGlobalFreeString = previous })
	for _, tt := range []struct{ name, value string }{
		{"empty", ""},
		{"unicode", "héllo 世界"},
		{"newlines", "line one\nline two"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stringallocator.Reset()
			t.Cleanup(stringallocator.Reset)
			owned := GdString(stringallocator.Allocate(tt.value))
			if owned == nil {
				t.Fatal("fixture failed to allocate a native string")
			}
			borrowed := copyString(owned)
			if borrowed != tt.value || stringallocator.ReleaseCount() != 0 {
				t.Fatalf("borrowed string = %q, release count = %d", borrowed, stringallocator.ReleaseCount())
			}
			got := ToString(owned)
			if got != tt.value || borrowed != tt.value {
				t.Fatalf("string copies after native release = %q, %q; want %q", got, borrowed, tt.value)
			}
			if stringallocator.ReleaseCount() != 1 || stringallocator.HasOutstandingAllocation() {
				t.Fatalf("runtime allocator release count = %d, outstanding allocation = %v", stringallocator.ReleaseCount(), stringallocator.HasOutstandingAllocation())
			}
			if got := ToString(nil); got != "" || stringallocator.ReleaseCount() != 1 {
				t.Fatalf("nil return = %q, release count = %d", got, stringallocator.ReleaseCount())
			}
		})
	}
}

// Native bindings and the C fixture are process-wide, so these tests must not
// run in parallel. Restore every binding to keep shuffled/repeated runs isolated.
func bindNativeBuiltinFixture(t *testing.T) []string {
	t.Helper()
	previousResolver, previousBuiltinAPI, previousAPI := resolveCFunc, builtinAPI, api
	previousCallbacks := callbacks
	stringallocator.Reset()
	t.Cleanup(func() {
		resolveCFunc, builtinAPI, api = previousResolver, previousBuiltinAPI, previousAPI
		callbacks = previousCallbacks
		stringallocator.Reset()
	})
	var names []string
	resolveCFunc = func(name string) unsafe.Pointer {
		names = append(names, name)
		return getProcAddress(uintptr(stringallocator.Resolver()), name)
	}
	builtinAPI.resolveAPIFunctions()
	return names
}

func TestNativeBuiltinResolution(t *testing.T) {
	names := bindNativeBuiltinFixture(t)
	bindings := map[string]unsafe.Pointer{
		"spx_global_register_callbacks": unsafe.Pointer(builtinAPI.SpxGlobalRegisterCallbacks),
		"spx_global_free_string":        unsafe.Pointer(builtinAPI.SpxGlobalFreeString),
	}
	if len(names) != len(bindings) {
		t.Fatalf("resolved symbols = %v, want %d builtin symbols", names, len(bindings))
	}
	for _, name := range names {
		got, ok := bindings[name]
		want := getProcAddress(uintptr(stringallocator.Resolver()), name)
		if !ok || got == nil || got != want {
			t.Fatalf("binding %q = %p, want %p", name, got, want)
		}
		delete(bindings, name)
	}
	if got := getProcAddress(uintptr(stringallocator.Resolver()), "unknown_builtin"); got != nil {
		t.Fatalf("unknown symbol = %p, want nil", got)
	}
	owned := GdString(stringallocator.Allocate("resolved allocator"))
	if got := ToString(owned); got != "resolved allocator" || stringallocator.ReleaseCount() != 1 || stringallocator.HasOutstandingAllocation() {
		t.Fatalf("resolved allocator returned %q, releases = %d, outstanding = %v", got, stringallocator.ReleaseCount(), stringallocator.HasOutstandingAllocation())
	}
}

func TestNativeInitialization(t *testing.T) {
	bindNativeBuiltinFixture(t)
	configuration := initialization{minimum_initialization_level: initializationLevelScene + 1}
	if got := gdspx_init(uintptr(stringallocator.Resolver()), nil, unsafe.Pointer(&configuration)); got != 1 {
		t.Fatalf("initialization result = %d, want success", got)
	}
	if configuration.minimum_initialization_level != initializationLevelScene {
		t.Fatal("initialization did not select the scene level")
	}
	if configuration.initialize == nil || configuration.deinitialize == nil {
		t.Fatal("initialization callbacks were not installed")
	}
	if !ToBool(CallPlatformIsMainThread()) || stringallocator.RegistrationCount() != 1 || !stringallocator.HasRegistration() {
		t.Fatal("initialization did not bind the engine API and register callbacks once")
	}
	// Other initialization levels and every shutdown level must safely return
	// through the installed C hooks without entering the application's main.
	stringallocator.InvokeInactiveLifecycle(unsafe.Pointer(&configuration))
}

func TestNativeSceneInitializationEntersMain(t *testing.T) {
	expectSceneMain = true
	defer func() {
		expectSceneMain = false
		if got := recover(); got != (sceneMainEntered{}) {
			t.Fatalf("scene initialization = %v, want entry into main", got)
		}
	}()
	initialize(nil, initializationLevelScene)
}

func TestNativeRegisteredCallbacksCrossCABI(t *testing.T) {
	bindNativeBuiltinFixture(t)
	registerEngineCallback()
	if stringallocator.RegistrationCount() != 1 || !stringallocator.HasRegistration() {
		t.Fatal("generated C callback table was not registered")
	}
	const id int64 = 1<<54 + 3
	const text = "héllo action 世界"
	borrowed := stringallocator.Allocate(text)
	BindCallback(engine.CallbackInfo{})
	// Calling back through C must also be safe before handlers are bound.
	if calls := stringallocator.InvokeCallbacks(id, -7, -.125, 2, borrowed); calls != 11 {
		t.Fatalf("registered C callback fields = %d, want 11", calls)
	}
	var got []any
	BindCallback(engine.CallbackInfo{
		CoreCallbackInfo: engine.CoreCallbackInfo{
			OnEngineStart:  func() { got = append(got, "start") },
			OnEngineUpdate: func(delta float64) { got = append(got, delta) },
			OnEnginePause:  func(paused bool) { got = append(got, paused) },
			OnKeyPressed:   func(key int64) { got = append(got, key) },
		},
		OnSpriteReady:             func(object int64) { got = append(got, object) },
		OnActionPressed:           func(action string) { got = append(got, action) },
		OnAxisChanged:             func(action string, value float64) { got = append(got, action, value) },
		OnCollisionEnter:          func(self, other int64) { got = append(got, self, other) },
		OnSceneSpriteInstantiated: func(object int64, name string) { got = append(got, object, name) },
		OnUiToggle:                func(object int64, on bool) { got = append(got, object, on) },
		OnUiTextChanged:           func(object int64, text string) { got = append(got, object, text) },
	})
	if calls := stringallocator.InvokeCallbacks(id, -7, -.125, 2, borrowed); calls != 11 {
		t.Fatalf("C callback calls = %d, want 11", calls)
	}
	if stringallocator.ReleaseCount() != 0 || copyString(GdString(borrowed)) != text {
		t.Fatal("callbacks released or modified the borrowed string")
	}
	*(*byte)(borrowed) = '?'
	want := []any{"start", -.125, true, id, int64(-7), text, text, -.125, id, int64(-7), id, text, id, true, id, text}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retained C callback values = %#v, want %#v", got, want)
	}
}

func TestNativeCallbacksForwardValuesAndAllowMissingHandlers(t *testing.T) {
	previous := callbacks
	t.Cleanup(func() { BindCallback(previous) })
	text := ToGdArrayInfo([]string{"héllo action"})
	defer text.Free()
	pointer := *(*unsafe.Pointer)(text.gdArray.data)
	const id int64 = 1<<54 + 3
	tested := make(map[string]bool)
	for _, tc := range []struct {
		field      string
		callback   any
		args, want []any
	}{
		{"OnEngineStart", func_on_engine_start, nil, []any{}},
		{"OnEngineDestroy", func_on_engine_destroy, nil, []any{}},
		{"OnEngineDestroyed", func_on_engine_destroyed, nil, []any{}},
		{"OnEngineReset", func_on_engine_reset, nil, []any{}},
		{"OnSpriteReady", func_on_sprite_ready, []any{id}, []any{id}},
		{"OnSpriteDestroyed", func_on_sprite_destroyed, []any{id}, []any{id}},
		{"OnMousePressed", func_on_mouse_pressed, []any{id}, []any{id}},
		{"OnMouseReleased", func_on_mouse_released, []any{id}, []any{id}},
		{"OnKeyPressed", func_on_key_pressed, []any{id}, []any{id}},
		{"OnKeyReleased", func_on_key_released, []any{id}, []any{id}},
		{"OnUiReady", func_on_ui_ready, []any{id}, []any{id}},
		{"OnUiUpdated", func_on_ui_updated, []any{id}, []any{id}},
		{"OnUiDestroyed", func_on_ui_destroyed, []any{id}, []any{id}},
		{"OnUiPressed", func_on_ui_pressed, []any{id}, []any{id}},
		{"OnUiReleased", func_on_ui_released, []any{id}, []any{id}},
		{"OnUiHovered", func_on_ui_hovered, []any{id}, []any{id}},
		{"OnUiClicked", func_on_ui_clicked, []any{id}, []any{id}},
		{"OnSpriteScreenEntered", func_on_sprite_screen_entered, []any{id}, []any{id}},
		{"OnSpriteScreenExited", func_on_sprite_screen_exited, []any{id}, []any{id}},
		{"OnSpriteVfxFinished", func_on_sprite_vfx_finished, []any{id}, []any{id}},
		{"OnSpriteAnimationFinished", func_on_sprite_animation_finished, []any{id}, []any{id}},
		{"OnSpriteAnimationLooped", func_on_sprite_animation_looped, []any{id}, []any{id}},
		{"OnSpriteFrameChanged", func_on_sprite_frame_changed, []any{id}, []any{id}},
		{"OnSpriteAnimationChanged", func_on_sprite_animation_changed, []any{id}, []any{id}},
		{"OnSpriteFramesSetChanged", func_on_sprite_frames_set_changed, []any{id}, []any{id}},
		{"OnEngineUpdate", func_on_engine_update, []any{.125}, []any{.125}},
		{"OnEngineFixedUpdate", func_on_engine_fixed_update, []any{.125}, []any{.125}},
		{"OnSpriteUpdated", func_on_sprite_updated, []any{.125}, []any{.125}},
		{"OnSpriteFixedUpdated", func_on_sprite_fixed_updated, []any{.125}, []any{.125}},
		{"OnCollisionEnter", func_on_collision_enter, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnCollisionStay", func_on_collision_stay, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnCollisionExit", func_on_collision_exit, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerEnter", func_on_trigger_enter, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerStay", func_on_trigger_stay, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerExit", func_on_trigger_exit, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnActionPressed", func_on_action_pressed, []any{pointer}, []any{"héllo action"}},
		{"OnActionJustPressed", func_on_action_just_pressed, []any{pointer}, []any{"héllo action"}},
		{"OnActionJustReleased", func_on_action_just_released, []any{pointer}, []any{"héllo action"}},

		{"OnSceneSpriteInstantiated", func_on_scene_sprite_instantiated, []any{id, pointer}, []any{id, "héllo action"}},
		{"OnUiTextChanged", func_on_ui_text_changed, []any{id, pointer}, []any{id, "héllo action"}},
		{"OnAxisChanged", func_on_axis_changed, []any{pointer, -.5}, []any{"héllo action", -.5}},
		{"OnEnginePause", func_on_engine_pause, []any{uint8(1)}, []any{true}},
		{"OnEnginePause", func_on_engine_pause, []any{uint8(2)}, []any{true}},
		{"OnEnginePause", func_on_engine_pause, []any{uint8(0)}, []any{false}},
		{"OnUiToggle", func_on_ui_toggle, []any{id, uint8(2)}, []any{id, true}},
		{"OnUiToggle", func_on_ui_toggle, []any{id, uint8(0)}, []any{id, false}},
	} {
		tested[tc.field] = true
		t.Run(tc.field, func(t *testing.T) {
			function := reflect.ValueOf(tc.callback)
			args := make([]reflect.Value, len(tc.args))
			for i, arg := range tc.args {
				args[i] = reflect.ValueOf(arg).Convert(function.Type().In(i))
			}
			BindCallback(engine.CallbackInfo{})
			function.Call(args) // Missing handlers must be safe.
			calls := 0
			var got []any
			info := engine.CallbackInfo{}
			field := reflect.ValueOf(&info).Elem().FieldByName(tc.field)
			field.Set(reflect.MakeFunc(field.Type(), func(values []reflect.Value) []reflect.Value {
				calls++
				got = make([]any, len(values))
				for i, value := range values {
					got[i] = value.Interface()
				}
				return nil
			}))
			BindCallback(info)
			function.Call(args)
			if calls != 1 {
				t.Fatalf("callback calls = %d, want 1", calls)
			}
			if text.ToStrings()[0] != "héllo action" {
				t.Fatal("callback freed or changed borrowed string")
			}
			// Retained callback values must survive changes to caller-owned memory.
			*(*byte)(pointer) = 'H'
			t.Cleanup(func() { *(*byte)(pointer) = 'h' })
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("retained callback values = %#v, want %#v", got, tc.want)
			}
		})
	}
	for _, field := range reflect.VisibleFields(reflect.TypeFor[engine.CallbackInfo]()) {
		if field.Type.Kind() == reflect.Func && !tested[field.Name] {
			t.Errorf("callback %s is missing a forwarding test", field.Name)
		}
	}
}
