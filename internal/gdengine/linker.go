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

package gdengine

//lint:file-ignore ST1001 Godot linker glue intentionally dot-imports engine API types.

import (
	"github.com/goplus/spx/v2/internal/gdengine/binding/facade"
	engineimpl "github.com/goplus/spx/v2/internal/gdengine/impl"
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
	gdspx "github.com/goplus/spx/v2/pkg/spx/pkg/gdspx"
)

var (
	mgrs                []IManager
	coreCallbacks       CoreCallbackInfo
	sprites             = make([]ISpriter, 0)
	isWebIntepreterMode bool
)

func init() {
	gdspx.SetLinkerBridge(linkerBridge{})
}

type linkerBridge struct{}

func (linkerBridge) IsWebIntepreterMode() bool {
	return IsWebIntepreterMode()
}

func (linkerBridge) Link(coreCallbackInfo CoreCallbackInfo) {
	Link(coreCallbackInfo)
}

func (linkerBridge) Unlink() {
	Unlink()
}

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}

func Link(coreCallbackInfo CoreCallbackInfo) {
	isWebIntepreterMode = facade.LinkFFI()
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	facade.RegisterCallbacks(infos)
	mgrs = engineimpl.CreateMgrs()
	engineimpl.BindMgr(mgrs)
	facade.OnLinked()
}

func Unlink() {
	mgrs = nil
	facade.UnlinkFFI()
}
