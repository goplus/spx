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

	"github.com/goplus/spx/v3/internal/engine"
)

type UiMonitor struct {
	UiNode
	views  [monitorAppearanceCount]monitorView
	active MonitorAppearance
}
type UpdateFunc func(float64)

type MonitorAppearance uint8

const (
	MonitorAppearanceDefault MonitorAppearance = iota
	MonitorAppearanceDefaultLarge
	MonitorAppearanceScratch
	MonitorAppearanceScratchLarge
	monitorAppearanceCount
)

type monitorView struct {
	root        *UiNode
	label       *UiNode
	value       *UiNode
	colorTarget *UiNode
}

type monitorViewSpec struct {
	root        string
	label       string
	value       string
	colorTarget string
}

var monitorViewSpecs = [monitorAppearanceCount]monitorViewSpec{
	MonitorAppearanceDefault: {
		root:  "BG",
		label: "BG/H/LabelName",
		value: "BG/H/C/H/LabelValue",
	},
	MonitorAppearanceDefaultLarge: {
		root:  "ValueOnly",
		value: "ValueOnly/LabelValue",
	},
	MonitorAppearanceScratch: {
		root:        "ScratchBG",
		label:       "ScratchBG/H/LabelMargin/LabelName",
		value:       "ScratchBG/H/ValueMargin/C/LabelValue",
		colorTarget: "ScratchBG/H/ValueMargin/C",
	},
	MonitorAppearanceScratchLarge: {
		root:        "ScratchValueOnly",
		value:       "ScratchValueOnly/C/LabelValue",
		colorTarget: "ScratchValueOnly/C",
	},
}

type monitorRenderSink interface {
	SetVisible(engine.Object, bool)
	SetText(engine.Object, string)
	SetColor(engine.Object, Color)
}

func (p MonitorAppearance) IsScratch() bool {
	return p == MonitorAppearanceScratch || p == MonitorAppearanceScratchLarge
}

func normalizeMonitorAppearance(appearance MonitorAppearance) MonitorAppearance {
	if appearance >= monitorAppearanceCount {
		return MonitorAppearanceDefault
	}
	return appearance
}

func NewUiMonitor() *UiMonitor {
	return engine.NewUiNode[UiMonitor]()
}

// !!Warning: this method is called from the engine callback context
func (pself *UiMonitor) OnStart() {
	pself.bindViews(func(path string) *UiNode {
		return engine.BridgeBindUI[UiNode](pself.GetId(), path)
	})
}

func (pself *UiMonitor) bindViews(bind func(string) *UiNode) {
	for i, spec := range monitorViewSpecs {
		view := monitorView{
			root:  bind(spec.root),
			value: bind(spec.value),
		}
		if spec.label != "" {
			view.label = bind(spec.label)
		}
		if spec.colorTarget != "" {
			view.colorTarget = bind(spec.colorTarget)
		}
		pself.views[i] = view
	}
	pself.active = monitorAppearanceCount
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

func (pself *UiMonitor) Render(appearance MonitorAppearance, name, value string, color Color) {
	pself.render(&mgr.UiMgr, appearance, name, value, color)
}

func (pself *UiMonitor) render(sink monitorRenderSink, appearance MonitorAppearance, name, value string, color Color) {
	appearance = normalizeMonitorAppearance(appearance)
	if pself.active != appearance {
		for i, view := range pself.views {
			sink.SetVisible(view.root.GetId(), MonitorAppearance(i) == appearance)
		}
		pself.active = appearance
	}

	view := pself.views[appearance]
	if view.label != nil {
		sink.SetText(view.label.GetId(), name)
	}
	sink.SetText(view.value.GetId(), value)
	if view.colorTarget != nil {
		sink.SetColor(view.colorTarget.GetId(), color)
	}
}
