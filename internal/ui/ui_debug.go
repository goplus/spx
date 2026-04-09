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

package ui

import "github.com/goplus/spx/v2/internal/engine"

type UiDebug struct {
	UiNode
	input *UiNode
}

func NewUiDebug() *UiDebug {
	panel := engine.NewUiNode[UiDebug]()
	return panel
}

// !!Warning: this method is called from the engine callback context
func (pself *UiDebug) OnStart() {
	pself.input = engine.BridgeBindUI[UiNode](pself.GetId(), "Label")
}

func (pself *UiDebug) Show(msg string) {
	mgr.UiMgr.SetScale(pself.GetId(), engine.UniformVec2(engine.WindowScale()))
	mgr.UiMgr.SetVisible(pself.input.GetId(), msg != "")
	mgr.UiMgr.SetText(pself.input.GetId(), msg)
}
