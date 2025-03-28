package engine

import (
	. "github.com/realdream-ai/mathf"
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

func (pself *UiNode) onCreate() {
	pself.OnUiClickEvent = NewEvent0()
	pself.OnUiPressedEvent = NewEvent0()
	pself.OnUiReleasedEvent = NewEvent0()
	pself.OnUiHoveredEvent = NewEvent0()
	pself.OnUiToggleEvent = NewEvent1[bool]()
	pself.OnUiTextChangedEvent = NewEvent1[string]()
}

func (pself *UiNode) V_OnUiClick() {
	pself.OnUiClickEvent.Trigger()
}
func (pself *UiNode) V_OnUiPressed() {
	pself.OnUiPressedEvent.Trigger()
}
func (pself *UiNode) V_OnUiReleased() {
	pself.OnUiReleasedEvent.Trigger()
}
func (pself *UiNode) V_OnUiHovered() {
	pself.OnUiHoveredEvent.Trigger()
}
func (pself *UiNode) V_OnUiToggle(isOn bool) {
	pself.OnUiToggleEvent.Trigger(isOn)
}
func (pself *UiNode) V_OnUiTextChanged(txt string) {
	pself.OnUiTextChangedEvent.Trigger(txt)
}

func (pself *UiNode) OnUiClick() {
}
func (pself *UiNode) OnUiPressed() {
}
func (pself *UiNode) OnUiReleased() {
}
func (pself *UiNode) OnUiHovered() {
}
func (pself *UiNode) OnUiToggle(isOn bool) {
}
func (pself *UiNode) OnUiTextChanged(txt string) {
}

func (pself *UiNode) Destroy() bool {
	return UiMgr.DestroyNode(pself.Id)
}
func (pself *UiNode) GetId() Object {
	return pself.Id
}
func (pself *UiNode) SetId(id Object) {
	pself.Id = id
}
func (pself *UiNode) OnStart() {
}
func (pself *UiNode) OnUpdate(delta float64) {
}
func (pself *UiNode) OnFixedUpdate(delta float64) {
}

func (pself *UiNode) OnDestroy() {
}

func (pself *UiNode) SetSize(value Vec2) {
	UiMgr.SetSize(pself.Id, value)
}
func (pself *UiNode) GetSize() Vec2 {
	return UiMgr.GetSize(pself.Id)
}
func (pself *UiNode) SetRotation(value float64) {
	UiMgr.SetRotation(pself.Id, value)
}
func (pself *UiNode) GetRotation() float64 {
	return UiMgr.GetRotation(pself.Id)
}
