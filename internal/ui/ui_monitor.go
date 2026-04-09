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
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
)

type UiMonitor struct {
	UiNode
	bgAll          *UiNode
	valueOnly      *UiNode
	labelName      *UiNode
	labelBg        *UiNode
	labelValue     *UiNode
	labelValueOnly *UiNode
}
type UpdateFunc func(float64)

func NewUiMonitor() *UiMonitor {
	panel := engine.NewUiNode[UiMonitor]()
	return panel
}

// !!Warning: this method is called from the engine callback context
func (pself *UiMonitor) OnStart() {
	pself.bgAll = engine.BridgeBindUI[UiNode](pself.GetId(), "BG")
	pself.labelName = engine.BridgeBindUI[UiNode](pself.GetId(), "BG/H/LabelName")
	pself.labelBg = engine.BridgeBindUI[UiNode](pself.GetId(), "BG/H/C")
	pself.labelValue = engine.BridgeBindUI[UiNode](pself.GetId(), "BG/H/C/H/LabelValue")

	pself.valueOnly = engine.BridgeBindUI[UiNode](pself.GetId(), "ValueOnly")
	pself.labelValueOnly = engine.BridgeBindUI[UiNode](pself.GetId(), "ValueOnly/LabelValue")

}
func (pself *UiMonitor) ShowAll(isOn bool) {
	mgr.UiMgr.SetVisible(pself.bgAll.GetId(), isOn)
	mgr.UiMgr.SetVisible(pself.valueOnly.GetId(), !isOn)
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	mgr.UiMgr.SetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	x *= engine.WindowScale()
	mgr.UiMgr.SetScale(pself.GetId(), engine.UniformVec2(x))
}
func (pself *UiMonitor) UpdatePos(wpos Vec2) {
	mgr.UiMgr.SetGlobalPosition(pself.GetId(), ViewToUI(wpos))
}

func (pself *UiMonitor) UpdateText(name, value string) {
	mgr.UiMgr.SetText(pself.labelName.GetId(), name)
	mgr.UiMgr.SetText(pself.labelValue.GetId(), value)
	mgr.UiMgr.SetText(pself.labelValueOnly.GetId(), value)
}
func (pself *UiMonitor) UpdateColor(color Color) {
	mgr.UiMgr.SetColor(pself.labelBg.GetId(), color)
	mgr.UiMgr.SetColor(pself.valueOnly.GetId(), color)
}
