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

	"github.com/goplus/spx/v4/internal/engine"
)

type UiQuote struct {
	UiNode
	container *UiNode
	labelDes  *UiNode
	labelMsg  *UiNode
}

func NewUiQuote() *UiQuote {
	return engine.NewUiNode[UiQuote]()
}

// OnStart binds UI nodes in the engine callback context.
func (p *UiQuote) OnStart() {
	p.container = engine.BridgeBindUI[UiNode](p.GetId(), "C")
	p.labelDes = engine.BridgeBindUI[UiNode](p.GetId(), "C/LabelDes")
	p.labelMsg = engine.BridgeBindUI[UiNode](p.GetId(), "C/LabelMsg")
}

func (p *UiQuote) SetText(pos mathf.Vec2, size mathf.Vec2, msg, description string) {
	engine.Managers().UiMgr.SetScale(p.GetId(), engine.UniformVec2(engine.WindowScale()))
	pos = engine.BridgeWorldToView(pos)
	targetPos := pos.Sub(mathf.NewVec2(size.X, -size.Y))
	engine.Managers().UiMgr.SetGlobalPosition(p.container.GetId(), ViewToUI(targetPos))
	engine.Managers().UiMgr.SetSize(p.container.GetId(), size.Mulf(2))
	engine.Managers().UiMgr.SetText(p.labelMsg.GetId(), msg)
	engine.Managers().UiMgr.SetText(p.labelDes.GetId(), description)
}
