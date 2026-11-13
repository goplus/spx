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

/*
#include "gdextension_spx_interface.h"
*/
import "C"

import (
	"unsafe"

	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type initialization = C.GDExtensionInitialization

type initializationLevel = C.GDExtensionInitializationLevel

const initializationLevelScene = C.GDEXTENSION_INITIALIZATION_SCENE

var (
	resolveCFunc func(string) unsafe.Pointer
	callbacks    engine.CallbackInfo
)

func BindCallbacks(info engine.CallbackInfo) {
	callbacks = info
}

//go:linkname main main.main
func main()

//export gdspx_init
func gdspx_init(resolver uintptr, _ unsafe.Pointer, configuration unsafe.Pointer) uint8 {
	resolveCFunc = func(name string) unsafe.Pointer {
		return getProcAddress(resolver, name)
	}

	builtinAPI.resolveAPIFunctions()
	api.resolveAPIFunctions()
	if api.SpxPlatformIsMainThread == nil {
		panic("gdengine: spx_platform_is_main_thread is unavailable")
	}
	if api.SpxSpriteClearColliderShape == nil {
		panic("gdengine: spx_sprite_clear_collider_shape is unavailable; update the SPX runtime")
	}
	init := (*initialization)(configuration)
	*init = initialization{minimum_initialization_level: initializationLevelScene}
	setupInitialization(init)
	registerEngineCallbacks()
	return 1
}

func setupInitialization(init *initialization) {
	C.cgo_setup_initialization(init)
}

func getProcAddress(resolver uintptr, name string) unsafe.Pointer {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return C.cgo_get_proc_address(C.pointer(resolver), cName)
}

// registerEngineCallbacks registers the Go callback entrypoints with the engine.
func registerEngineCallbacks() {
	C.cgo_register_engine_callbacks(
		C.pointer(uintptr(unsafe.Pointer(builtinAPI.SpxGlobalRegisterCallbacks))),
	)
}

//export initialize
func initialize(_ unsafe.Pointer, level initializationLevel) {
	if level == initializationLevelScene {
		main()
	}
}

//export deinitialize
func deinitialize(_ unsafe.Pointer, level initializationLevel) {}
