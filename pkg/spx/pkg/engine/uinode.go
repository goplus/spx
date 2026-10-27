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

package engine

//lint:file-ignore ST1001 Public engine API intentionally exposes mathf type names directly.

import (
	. "github.com/goplus/spbase/mathf"
)

type UiNode struct {
	Id                   Object
	OnUiClickEvent       *Event0
	OnUiPressedEvent     *Event0
	OnUiReleasedEvent    *Event0
	OnUiHoveredEvent     *Event0
	OnUiToggleEvent      *Event1[bool]
	OnUiTextChangedEvent *Event1[string]
}

func (p *UiNode) V_OnUiClick() {
	p.OnUiClickEvent.Trigger()
}

func (p *UiNode) V_OnUiPressed() {
	p.OnUiPressedEvent.Trigger()
}

func (p *UiNode) V_OnUiReleased() {
	p.OnUiReleasedEvent.Trigger()
}

func (p *UiNode) V_OnUiHovered() {
	p.OnUiHoveredEvent.Trigger()
}

func (p *UiNode) V_OnUiToggle(isOn bool) {
	p.OnUiToggleEvent.Trigger(isOn)
}

func (p *UiNode) V_OnUiTextChanged(txt string) {
	p.OnUiTextChangedEvent.Trigger(txt)
}

func (p *UiNode) OnUiClick() {
}

func (p *UiNode) OnUiPressed() {
}

func (p *UiNode) OnUiReleased() {
}

func (p *UiNode) OnUiHovered() {
}

func (p *UiNode) OnUiToggle(isOn bool) {
}

func (p *UiNode) OnUiTextChanged(txt string) {
}

func (p *UiNode) Destroy() bool {
	return UiMgr.DestroyNode(p.Id)
}

func (p *UiNode) GetId() Object {
	return p.Id
}

func (p *UiNode) SetId(id Object) {
	p.Id = id
}

func (p *UiNode) OnStart() {
}

func (p *UiNode) OnUpdate(delta float64) {
}

func (p *UiNode) OnFixedUpdate(delta float64) {
}

func (p *UiNode) OnDestroy() {
}

func (p *UiNode) SetSize(value Vec2) {
	UiMgr.SetSize(p.Id, value)
}

func (p *UiNode) GetSize() Vec2 {
	return UiMgr.GetSize(p.Id)
}

func (p *UiNode) SetRotation(value float64) {
	UiMgr.SetRotation(p.Id, value)
}

func (p *UiNode) GetRotation() float64 {
	return UiMgr.GetRotation(p.Id)
}

func (p *UiNode) onCreate() {
	p.OnUiClickEvent = NewEvent0()
	p.OnUiPressedEvent = NewEvent0()
	p.OnUiReleasedEvent = NewEvent0()
	p.OnUiHoveredEvent = NewEvent0()
	p.OnUiToggleEvent = NewEvent1[bool]()
	p.OnUiTextChangedEvent = NewEvent1[string]()
}
