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
	"fmt"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/goplus/spx/v4/internal/gdengine/binding/native/testdata/stringallocator"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type applicationMainEntered struct{}

var interceptApplicationMain bool

func TestMain(m *testing.M) {
	// The installed scene hook calls main.main through go:linkname. Intercept
	// that reentry so lifecycle tests cannot recursively run the test suite.
	if interceptApplicationMain {
		panic(applicationMainEntered{})
	}
	os.Exit(m.Run())
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
	for _, phase := range []struct {
		name       string
		initialize bool
	}{{"initialize", true}, {"deinitialize", false}} {
		for level := 0; level <= 3; level++ {
			t.Run(fmt.Sprintf("%s/%d", phase.name, level), func(t *testing.T) {
				interceptApplicationMain = true
				t.Cleanup(func() { interceptApplicationMain = false })
				defer func() {
					got := recover()
					if phase.initialize && level == int(initializationLevelScene) {
						if got != (applicationMainEntered{}) {
							t.Fatalf("scene hook entered %v, want application main", got)
						}
					} else if got != nil {
						t.Fatalf("inactive lifecycle hook entered application main: %v", got)
					}
				}()
				stringallocator.InvokeLifecycle(unsafe.Pointer(&configuration), level, phase.initialize)
			})
		}
	}
}

func TestNativeInitializationRejectsMissingMainThreadAPI(t *testing.T) {
	bindNativeBuiltinFixture(t)
	configuration := initialization{}
	gdspx_init(uintptr(stringallocator.Resolver()), nil, unsafe.Pointer(&configuration))
	// A previous link must not mask a missing required symbol in the next one.
	configuration = initialization{minimum_initialization_level: initializationLevelScene + 1}
	defer func() {
		if got := recover(); got != "gdengine: spx_platform_is_main_thread is unavailable" {
			t.Fatalf("missing main-thread API panic = %v", got)
		}
		if api.SpxPlatformIsMainThread != nil || stringallocator.RegistrationCount() != 1 {
			t.Fatal("failed startup reused a stale symbol or registered callbacks")
		}
		if configuration.minimum_initialization_level != initializationLevelScene+1 || configuration.initialize != nil || configuration.deinitialize != nil {
			t.Fatal("failed startup published initialization hooks")
		}
	}()
	gdspx_init(uintptr(stringallocator.ResolverWithoutMainThread()), nil, unsafe.Pointer(&configuration))
}

func TestNativeRegisteredCallbacksCrossCABI(t *testing.T) {
	bindNativeBuiltinFixture(t)
	registerEngineCallbacks()
	if stringallocator.RegistrationCount() != 1 || !stringallocator.HasRegistration() {
		t.Fatal("generated C callback table was not registered")
	}
	const id int64 = 1<<54 + 3
	const text = "héllo action 世界"
	borrowed := stringallocator.Allocate(text)
	BindCallbacks(engine.CallbackInfo{})
	// Calling back through C must also be safe before handlers are bound.
	if calls := stringallocator.InvokeCallbacks(id, -7, -.125, 2, borrowed); calls != 11 {
		t.Fatalf("registered C callback fields = %d, want 11", calls)
	}
	var got []any
	BindCallbacks(engine.CallbackInfo{
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
	t.Cleanup(func() { BindCallbacks(previous) })
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
		{"OnEngineStart", go_on_engine_start, nil, []any{}},
		{"OnEngineDestroy", go_on_engine_destroy, nil, []any{}},
		{"OnEngineDestroyed", go_on_engine_destroyed, nil, []any{}},
		{"OnEngineReset", go_on_engine_reset, nil, []any{}},
		{"OnSpriteReady", go_on_sprite_ready, []any{id}, []any{id}},
		{"OnSpriteDestroyed", go_on_sprite_destroyed, []any{id}, []any{id}},
		{"OnMousePressed", go_on_mouse_pressed, []any{id}, []any{id}},
		{"OnMouseReleased", go_on_mouse_released, []any{id}, []any{id}},
		{"OnKeyPressed", go_on_key_pressed, []any{id}, []any{id}},
		{"OnKeyReleased", go_on_key_released, []any{id}, []any{id}},
		{"OnUiReady", go_on_ui_ready, []any{id}, []any{id}},
		{"OnUiUpdated", go_on_ui_updated, []any{id}, []any{id}},
		{"OnUiDestroyed", go_on_ui_destroyed, []any{id}, []any{id}},
		{"OnUiPressed", go_on_ui_pressed, []any{id}, []any{id}},
		{"OnUiReleased", go_on_ui_released, []any{id}, []any{id}},
		{"OnUiHovered", go_on_ui_hovered, []any{id}, []any{id}},
		{"OnUiClicked", go_on_ui_clicked, []any{id}, []any{id}},
		{"OnSpriteScreenEntered", go_on_sprite_screen_entered, []any{id}, []any{id}},
		{"OnSpriteScreenExited", go_on_sprite_screen_exited, []any{id}, []any{id}},
		{"OnSpriteVfxFinished", go_on_sprite_vfx_finished, []any{id}, []any{id}},
		{"OnSpriteAnimationFinished", go_on_sprite_animation_finished, []any{id}, []any{id}},
		{"OnSpriteAnimationLooped", go_on_sprite_animation_looped, []any{id}, []any{id}},
		{"OnSpriteFrameChanged", go_on_sprite_frame_changed, []any{id}, []any{id}},
		{"OnSpriteAnimationChanged", go_on_sprite_animation_changed, []any{id}, []any{id}},
		{"OnSpriteFramesSetChanged", go_on_sprite_frames_set_changed, []any{id}, []any{id}},
		{"OnEngineUpdate", go_on_engine_update, []any{.125}, []any{.125}},
		{"OnEngineFixedUpdate", go_on_engine_fixed_update, []any{.125}, []any{.125}},
		{"OnSpriteUpdated", go_on_sprite_updated, []any{.125}, []any{.125}},
		{"OnSpriteFixedUpdated", go_on_sprite_fixed_updated, []any{.125}, []any{.125}},
		{"OnCollisionEnter", go_on_collision_enter, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnCollisionStay", go_on_collision_stay, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnCollisionExit", go_on_collision_exit, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerEnter", go_on_trigger_enter, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerStay", go_on_trigger_stay, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnTriggerExit", go_on_trigger_exit, []any{id, int64(-7)}, []any{id, int64(-7)}},
		{"OnActionPressed", go_on_action_pressed, []any{pointer}, []any{"héllo action"}},
		{"OnActionJustPressed", go_on_action_just_pressed, []any{pointer}, []any{"héllo action"}},
		{"OnActionJustReleased", go_on_action_just_released, []any{pointer}, []any{"héllo action"}},

		{"OnSceneSpriteInstantiated", go_on_scene_sprite_instantiated, []any{id, pointer}, []any{id, "héllo action"}},
		{"OnUiTextChanged", go_on_ui_text_changed, []any{id, pointer}, []any{id, "héllo action"}},
		{"OnAxisChanged", go_on_axis_changed, []any{pointer, -.5}, []any{"héllo action", -.5}},
		{"OnEnginePause", go_on_engine_pause, []any{uint8(1)}, []any{true}},
		{"OnEnginePause", go_on_engine_pause, []any{uint8(2)}, []any{true}},
		{"OnEnginePause", go_on_engine_pause, []any{uint8(0)}, []any{false}},
		{"OnUiToggle", go_on_ui_toggle, []any{id, uint8(2)}, []any{id, true}},
		{"OnUiToggle", go_on_ui_toggle, []any{id, uint8(0)}, []any{id, false}},
	} {
		tested[tc.field] = true
		t.Run(tc.field, func(t *testing.T) {
			function := reflect.ValueOf(tc.callback)
			args := make([]reflect.Value, len(tc.args))
			for i, arg := range tc.args {
				args[i] = reflect.ValueOf(arg).Convert(function.Type().In(i))
			}
			BindCallbacks(engine.CallbackInfo{})
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
			BindCallbacks(info)
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
