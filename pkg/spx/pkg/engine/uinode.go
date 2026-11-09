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

func (u *UiNode) V_OnUiClick() {
	u.OnUiClickEvent.Trigger()
}

func (u *UiNode) V_OnUiPressed() {
	u.OnUiPressedEvent.Trigger()
}

func (u *UiNode) V_OnUiReleased() {
	u.OnUiReleasedEvent.Trigger()
}

func (u *UiNode) V_OnUiHovered() {
	u.OnUiHoveredEvent.Trigger()
}

func (u *UiNode) V_OnUiToggle(isOn bool) {
	u.OnUiToggleEvent.Trigger(isOn)
}

func (u *UiNode) V_OnUiTextChanged(txt string) {
	u.OnUiTextChangedEvent.Trigger(txt)
}

func (u *UiNode) OnUiClick() {
}

func (u *UiNode) OnUiPressed() {
}

func (u *UiNode) OnUiReleased() {
}

func (u *UiNode) OnUiHovered() {
}

func (u *UiNode) OnUiToggle(isOn bool) {
}

func (u *UiNode) OnUiTextChanged(txt string) {
}

func (u *UiNode) Destroy() bool {
	return UiMgr.DestroyNode(u.Id)
}

func (u *UiNode) GetId() Object {
	return u.Id
}

func (u *UiNode) SetId(id Object) {
	u.Id = id
}

func (u *UiNode) OnStart() {
}

func (u *UiNode) OnUpdate(delta float64) {
}

func (u *UiNode) OnFixedUpdate(delta float64) {
}

func (u *UiNode) OnDestroy() {
}

func (u *UiNode) SetSize(value Vec2) {
	UiMgr.SetSize(u.Id, value)
}

func (u *UiNode) GetSize() Vec2 {
	return UiMgr.GetSize(u.Id)
}

func (u *UiNode) SetRotation(value float64) {
	UiMgr.SetRotation(u.Id, value)
}

func (u *UiNode) GetRotation() float64 {
	return UiMgr.GetRotation(u.Id)
}

func (u *UiNode) onCreate() {
	u.OnUiClickEvent = NewEvent0()
	u.OnUiPressedEvent = NewEvent0()
	u.OnUiReleasedEvent = NewEvent0()
	u.OnUiHoveredEvent = NewEvent0()
	u.OnUiToggleEvent = NewEvent1[bool]()
	u.OnUiTextChangedEvent = NewEvent1[string]()
}
