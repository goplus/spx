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

import (
	. "github.com/goplus/spbase/mathf"
)

func (pself *UiNode) GetType() int64 {
	return UiMgr.GetType(pself.Id)
}
func (pself *UiNode) SetText(text string) {
	UiMgr.SetText(pself.Id, text)
}
func (pself *UiNode) GetText() string {
	return UiMgr.GetText(pself.Id)
}
func (pself *UiNode) SetTexture(path string) {
	UiMgr.SetTexture(pself.Id, path)
}
func (pself *UiNode) GetTexture() string {
	return UiMgr.GetTexture(pself.Id)
}
func (pself *UiNode) SetColor(color Color) {
	UiMgr.SetColor(pself.Id, color)
}
func (pself *UiNode) GetColor() Color {
	return UiMgr.GetColor(pself.Id)
}
func (pself *UiNode) SetFontSize(size int64) {
	UiMgr.SetFontSize(pself.Id, size)
}
func (pself *UiNode) GetFontSize() int64 {
	return UiMgr.GetFontSize(pself.Id)
}
func (pself *UiNode) SetVisible(visible bool) {
	UiMgr.SetVisible(pself.Id, visible)
}
func (pself *UiNode) GetVisible() bool {
	return UiMgr.GetVisible(pself.Id)
}
func (pself *UiNode) SetInteractable(interactable bool) {
	UiMgr.SetInteractable(pself.Id, interactable)
}
func (pself *UiNode) GetInteractable() bool {
	return UiMgr.GetInteractable(pself.Id)
}
func (pself *UiNode) SetRect(rect Rect2) {
	UiMgr.SetRect(pself.Id, rect)
}
func (pself *UiNode) GetRect() Rect2 {
	return UiMgr.GetRect(pself.Id)
}
func (pself *UiNode) SetPosition(value Vec2) {
	UiMgr.SetPosition(pself.Id, value)
}
func (pself *UiNode) GetPosition() Vec2 {
	return UiMgr.GetPosition(pself.Id)
}

func (pself *UiNode) SetGlobalPosition(value Vec2) {
	UiMgr.SetGlobalPosition(pself.Id, value)
}
func (pself *UiNode) GetGlobalPosition() Vec2 {
	return UiMgr.GetGlobalPosition(pself.Id)
}
func (pself *UiNode) SetScale(value Vec2) {
	UiMgr.SetScale(pself.Id, value)
}
func (pself *UiNode) GetScale() Vec2 {
	return UiMgr.GetScale(pself.Id)
}
