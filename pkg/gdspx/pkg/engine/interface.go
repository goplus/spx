package engine

import (
	. "github.com/realdream-ai/mathf"
)

type ILifeCycle interface {
	OnStart()
	OnFixedUpdate(delta float64)
	OnUpdate(delta float64)
	OnDestroy()
}
type IManager interface {
	ILifeCycle
}

type ISpriter interface {
	ILifeCycle
	onCreate()
	GetId() Object
	SetId(Object)
	Destroy() bool
	GetPosition() Vec2
	SetPosition(pos Vec2)

	OnTriggerEnter(ISpriter)
	V_OnTriggerEnter(ISpriter)

	OnTriggerExit(ISpriter)
	V_OnTriggerExit(ISpriter)

	OnScreenEntered()
	V_OnScreenEntered()

	OnScreenExited()
	V_OnScreenExited()

	OnVfxFinished()
	V_OnVfxFinished()

	OnAnimationFinished()
	V_OnAnimationFinished()

	OnAnimationLooped()
	V_OnAnimationLooped()

	OnFrameChanged()
	V_OnFrameChanged()

	OnAnimationChanged()
	V_OnAnimationChanged()

	OnFramesSetChanged()
	V_OnFramesSetChanged()
}
type IUiNode interface {
	ILifeCycle
	onCreate()
	GetId() Object
	SetId(Object)
	Destroy() bool
	OnUiClick()
	V_OnUiClick()

	OnUiPressed()
	V_OnUiPressed()

	OnUiReleased()
	V_OnUiReleased()

	OnUiHovered()
	V_OnUiHovered()

	OnUiToggle(isOn bool)
	V_OnUiToggle(isOn bool)

	OnUiTextChanged(txt string)
	V_OnUiTextChanged(txt string)
}
type EngineCallbackInfo struct {
	OnEngineStart       func()
	OnEngineUpdate      func(float64)
	OnEngineFixedUpdate func(float64)
	OnEngineDestroy     func()

	OnKeyPressed  func(int64)
	OnKeyReleased func(int64)
}

type CallbackInfo struct {
	EngineCallbackInfo
	// scene
	OnSceneSpriteInstantiated func(int64, string)
	// life cycle
	OnSpriteReady        func(int64)
	OnSpriteUpdated      func(float64)
	OnSpriteFixedUpdated func(float64)
	OnSpriteDestroyed    func(int64)

	// input
	OnMousePressed       func(int64)
	OnMouseReleased      func(int64)
	OnActionPressed      func(string)
	OnActionJustPressed  func(string)
	OnActionJustReleased func(string)
	OnAxisChanged        func(string, float64)

	// physic
	OnCollisionEnter func(int64, int64)
	OnCollisionStay  func(int64, int64)
	OnCollisionExit  func(int64, int64)

	OnTriggerEnter func(int64, int64)
	OnTriggerStay  func(int64, int64)
	OnTriggerExit  func(int64, int64)

	// UI
	OnUiReady       func(int64)
	OnUiUpdated     func(int64)
	OnUiDestroyed   func(int64)
	OnUiPressed     func(int64)
	OnUiReleased    func(int64)
	OnUiHovered     func(int64)
	OnUiClicked     func(int64)
	OnUiToggle      func(int64, bool)
	OnUiTextChanged func(int64, string)

	OnSpriteScreenEntered     func(int64)
	OnSpriteScreenExited      func(int64)
	OnSpriteVfxFinished       func(int64)
	OnSpriteAnimationFinished func(int64)
	OnSpriteAnimationLooped   func(int64)
	OnSpriteFrameChanged      func(int64)
	OnSpriteAnimationChanged  func(int64)
	OnSpriteFramesSetChanged  func(int64)
}
