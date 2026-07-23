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

package gdspx

import (
	engine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type LinkerBridge interface {
	IsWebIntepreterMode() bool
	Link(coreCallbackInfo engine.CoreCallbackInfo)
	Unlink()
}

var linkerBridge LinkerBridge

func SetLinkerBridge(bridge LinkerBridge) {
	linkerBridge = bridge
}

func requireLinkerBridge() LinkerBridge {
	if linkerBridge == nil {
		panic("gdspx linker bridge is not initialized")
	}
	return linkerBridge
}

// IsWebIntepreterMode is a compatibility shim around the runtime linker bridge.
func IsWebIntepreterMode() bool {
	return requireLinkerBridge().IsWebIntepreterMode()
}

// LinkEngine is a compatibility shim around the runtime linker bridge.
func LinkEngine(coreCallbackInfo engine.CoreCallbackInfo) {
	requireLinkerBridge().Link(coreCallbackInfo)
}

// UnlinkEngine is a compatibility shim around the runtime linker bridge.
func UnlinkEngine() {
	requireLinkerBridge().Unlink()
}
