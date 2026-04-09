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

import (
	"github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
)

type UiQuote struct {
	UiNode
	container *UiNode
	imageL    *UiNode
	imageR    *UiNode
	labelDes  *UiNode
	labelMsg  *UiNode
}

func NewUiQuote() *UiQuote {
	panel := engine.NewUiNode[UiQuote]()
	return panel
}

// !!Warning: this method is called from the engine callback context
func (pself *UiQuote) OnStart() {
	pself.container = engine.BridgeBindUI[UiNode](pself.GetId(), "C")
	pself.imageL = engine.BridgeBindUI[UiNode](pself.GetId(), "C/ImageL")
	pself.imageR = engine.BridgeBindUI[UiNode](pself.GetId(), "C/ImageR")
	pself.labelDes = engine.BridgeBindUI[UiNode](pself.GetId(), "C/LabelDes")
	pself.labelMsg = engine.BridgeBindUI[UiNode](pself.GetId(), "C/LabelMsg")
}

func (pself *UiQuote) SetText(pos mathf.Vec2, size mathf.Vec2, msg, description string) {
	mgr.UiMgr.SetScale(pself.GetId(), engine.UniformVec2(engine.WindowScale()))
	pos = engine.BridgeWorldToView(pos)
	targetPos := pos.Sub(mathf.NewVec2(size.X, -size.Y))
	mgr.UiMgr.SetGlobalPosition(pself.container.GetId(), ViewToUI(targetPos))
	mgr.UiMgr.SetSize(pself.container.GetId(), size.Mulf(2))
	mgr.UiMgr.SetText(pself.labelMsg.GetId(), msg)
	mgr.UiMgr.SetText(pself.labelDes.GetId(), description)
}
