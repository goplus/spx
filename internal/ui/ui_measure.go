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

//lint:file-ignore ST1001 UI glue intentionally dot-imports mathf to mirror engine type names.

import (
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v4/internal/engine"
)

type UiMeasure struct {
	UiNode
	container      *UiNode
	imageLine      *UiNode
	labelValue     *UiNode
	labelContainer *UiNode
}

func NewUiMeasure() *UiMeasure {
	return engine.NewUiNode[UiMeasure]()
}

// OnStart binds UI nodes in the engine callback context.
func (p *UiMeasure) OnStart() {
	p.container = engine.BridgeBindUI[UiNode](p.GetId(), "C")
	p.imageLine = engine.BridgeBindUI[UiNode](p.GetId(), "C/Line")
	p.labelContainer = engine.BridgeBindUI[UiNode](p.GetId(), "LC")
	p.labelValue = engine.BridgeBindUI[UiNode](p.GetId(), "LC/Label")
}

func (p *UiMeasure) UpdateInfo(wpos Vec2, length, heading float64, name string, color Color) {
	engine.Managers().UiMgr.SetScale(p.GetId(), engine.UniformVec2(engine.WindowScale()))
	extraLen := 4.0 // Compensate for the engine's measurement image size.
	length += extraLen

	rad := engine.DegToRad(heading - 90)
	sc := engine.Sincos(rad).Mulf(length / 2)
	pos := ViewToUI(wpos)
	labelPos := pos
	pos = pos.Sub(NewVec2(sc.Y, sc.X))

	engine.Managers().UiMgr.SetGlobalPosition(p.container.GetId(), pos)
	engine.Managers().UiMgr.SetColor(p.container.GetId(), color)
	engine.Managers().UiMgr.SetSize(p.container.GetId(), NewVec2(length+extraLen, 26))
	engine.Managers().UiMgr.SetRotation(p.container.GetId(), rad)

	engine.Managers().UiMgr.SetGlobalPosition(p.labelContainer.GetId(), labelPos)
	engine.Managers().UiMgr.SetColor(p.labelContainer.GetId(), color)
	engine.Managers().UiMgr.SetText(p.labelValue.GetId(), name)
}
