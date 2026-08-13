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
	"unsafe"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)
import "C"

var (
	resolveCFunc func(string) unsafe.Pointer
	callbacks    engine.CallbackInfo
)

//go:linkname main main.main
func main()

func Link() bool {
	return false
}

func Linked() {

}

func Unlink() {

}

func BindCallback(info engine.CallbackInfo) {
	callbacks = info
}

//export gdspx_init
func gdspx_init(lookupFunc uintptr, classes, configuration unsafe.Pointer) uint8 {
	_ = classes // reserved for future class registration
	resolveCFunc = func(s string) unsafe.Pointer {
		return getProcAddress(lookupFunc, s)
	}

	builtinAPI.resolveAPIFunctions()
	api.resolveAPIFunctions()
	if api.SpxPlatformIsMainThread == nil {
		panic("gdengine: spx_platform_is_main_thread is unavailable")
	}
	init := (*initialization)(configuration)
	*init = initialization{}
	init.minimum_initialization_level = initializationLevel(GDExtensionInitializationLevelScene)
	doInitialization(init)
	registerEngineCallback()
	return 1
}
