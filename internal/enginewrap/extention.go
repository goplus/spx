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

package enginewrap

//lint:file-ignore ST1001 Engine manager extension code intentionally dot-imports engine API types.

import (
	. "github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

var mainCallback func(call func())

func Init(call func(f func())) {
	mainCallback = call
}

func callInMainThread(call func()) {
	if mainCallback == nil {
		panic("enginewrap: Init must be called before using manager methods off the main thread")
	}
	mainCallback(call)
}

const (
	MOUSE_BUTTON_LEFT   int64 = 1
	MOUSE_BUTTON_RIGHT  int64 = 2
	MOUSE_BUTTON_MIDDLE int64 = 3
)

func (*inputMgrImpl) MousePressed() bool {
	return inputMgr.GetMouseState(MOUSE_BUTTON_LEFT) || inputMgr.GetMouseState(MOUSE_BUTTON_RIGHT)
}

func (*platformMgrImpl) SetRunnableOnUnfocused(flag bool) {
	if !flag {
		spxlog.Warn("SetRunnableOnUnfocused(false) is not implemented yet")
	}
}

func (c *cameraMgrImpl) GetPosition() Vec2 {
	pos := c.GetCameraPosition()
	return NewVec2(pos.X, -pos.Y)
}

func (c *cameraMgrImpl) SetPosition(position Vec2) {
	c.SetCameraPosition(NewVec2(position.X, -position.Y))
}
