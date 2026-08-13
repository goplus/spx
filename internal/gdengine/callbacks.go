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

package gdengine

//lint:file-ignore ST1001 Godot callback glue intentionally dot-imports engine API types.

import (
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
	. "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func bindCallbacks() CallbackInfo {
	infos := CallbackInfo{}
	infos.OnEngineStart = onEngineStart
	infos.OnEngineUpdate = onEngineUpdate
	infos.OnEngineFixedUpdate = onEngineFixedUpdate
	infos.OnEngineDestroy = onEngineDestroy
	infos.OnEngineReset = onEngineReset
	infos.OnEnginePause = onEnginePause

	infos.OnSceneSpriteInstantiated = onSceneSpriteInstantiated

	infos.OnSpriteReady = onSpriteReady
	infos.OnSpriteUpdated = onSpriteUpdated
	infos.OnSpriteFixedUpdated = onSpriteFixedUpdated
	infos.OnSpriteDestroyed = onSpriteDestroyed

	// input
	infos.OnMousePressed = onMousePressed
	infos.OnMouseReleased = onMouseReleased
	infos.OnKeyPressed = onKeyPressed
	infos.OnKeyReleased = onKeyReleased
	infos.OnActionPressed = onActionPressed
	infos.OnActionJustPressed = onActionJustPressed
	infos.OnActionJustReleased = onActionJustReleased
	infos.OnAxisChanged = onAxisChanged

	// physics
	infos.OnCollisionEnter = onCollisionEnter
	infos.OnCollisionStay = onCollisionStay
	infos.OnCollisionExit = onCollisionExit

	infos.OnTriggerEnter = onTriggerEnter
	infos.OnTriggerStay = onTriggerStay
	infos.OnTriggerExit = onTriggerExit

	// ui
	// OnUiReady/OnUiUpdated are intentionally left unbound here.
	// Go-side UI construction already triggers OnStart, and the callback payload
	// does not include delta for a meaningful OnUpdate dispatch.
	infos.OnUiDestroyed = onUiDestroyed
	infos.OnUiPressed = onUiPressed
	infos.OnUiReleased = onUiReleased
	infos.OnUiHovered = onUiHovered
	infos.OnUiClicked = onUiClicked
	infos.OnUiToggle = onUiToggle
	infos.OnUiTextChanged = onUiTextChanged

	infos.OnSpriteScreenEntered = onSpriteScreenEntered
	infos.OnSpriteScreenExited = onSpriteScreenExited
	infos.OnSpriteVfxFinished = onSpriteVfxFinished
	infos.OnSpriteAnimationFinished = onSpriteAnimationFinished
	infos.OnSpriteAnimationLooped = onSpriteAnimationLooped
	infos.OnSpriteFrameChanged = onSpriteFrameChanged
	infos.OnSpriteAnimationChanged = onSpriteAnimationChanged
	infos.OnSpriteFramesSetChanged = onSpriteFramesSetChanged

	return infos
}

func onEngineStart() {
	for _, mgr := range mgrs {
		mgr.OnStart()
	}
	if coreCallbacks.OnEngineStart != nil {
		coreCallbacks.OnEngineStart()
	}
}

func onEngineUpdate(delta float64) {
	// Logical update callbacks share SPX's fixed delta while an input replay
	// session is active, keeping scripts, timers, tweens, and engine-facing
	// updates aligned.
	delta = itime.EffectiveLogicalDeltaTime(delta)
	for _, mgr := range mgrs {
		mgr.OnUpdate(delta)
	}
	AdvanceTimeSinceGameStart(delta)
	sprites = sprites[:0]
	for _, sprite := range Sprites() {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnUpdate(delta)
	}
	if coreCallbacks.OnEngineUpdate != nil {
		coreCallbacks.OnEngineUpdate(delta)
	}
	InternalUpdateEngine(delta)
}

func onEngineFixedUpdate(delta float64) {
	// Fixed-update callbacks intentionally keep Godot's raw physics delta.
	// Input replay virtualizes only SPX logical update time.
	for _, mgr := range mgrs {
		mgr.OnFixedUpdate(delta)
	}
	sprites = sprites[:0]
	for _, sprite := range Sprites() {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnFixedUpdate(delta)
	}
	if coreCallbacks.OnEngineFixedUpdate != nil {
		coreCallbacks.OnEngineFixedUpdate(delta)
	}
}

func onEngineDestroy() {
	if coreCallbacks.OnEngineDestroy != nil {
		coreCallbacks.OnEngineDestroy()
	}
	sprites = sprites[:0]
	for _, sprite := range Sprites() {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnDestroy()
	}
	for _, mgr := range mgrs {
		mgr.OnDestroy()
	}
}

func onEngineReset() {
	if coreCallbacks.OnEngineReset != nil {
		coreCallbacks.OnEngineReset()
	}
}

func onEnginePause(isPaused bool) {
	if coreCallbacks.OnEnginePause != nil {
		coreCallbacks.OnEnginePause(isPaused)
	}

	for _, mgr := range mgrs {
		mgr.OnPause(isPaused)
	}
}

func onSceneSpriteInstantiated(id int64, type_name string) {
	BindSceneInstantiatedSprite(Object(id), type_name)
}

// sprite
func onSpriteReady(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.OnStart()
	}
}

func onSpriteUpdated(delta float64) {
	spxlog.Debug("OnSpriteUpdated %f", delta)
}

func onSpriteFixedUpdated(delta float64) {
	spxlog.Debug("OnSpriteFixedUpdated %f", delta)
}

func onSpriteDestroyed(id int64) {
	DeleteSprite(Object(id))
}

// input
func onMousePressed(id int64) {
	spxlog.Debug("OnMousePressed %d", id)
	if coreCallbacks.OnMousePressed != nil {
		coreCallbacks.OnMousePressed(id)
	}
}

func onMouseReleased(id int64) {
	spxlog.Debug("OnMouseReleased %d", id)
	if coreCallbacks.OnMouseReleased != nil {
		coreCallbacks.OnMouseReleased(id)
	}
}

func onKeyPressed(id int64) {
	spxlog.Debug("OnKeyPressed %d", id)
	if coreCallbacks.OnKeyPressed != nil {
		coreCallbacks.OnKeyPressed(id)
	}
}

func onKeyReleased(id int64) {
	spxlog.Debug("OnKeyReleased %d", id)
	if coreCallbacks.OnKeyReleased != nil {
		coreCallbacks.OnKeyReleased(id)
	}
}

func onActionPressed(name string) {
	spxlog.Debug("OnActionPressed %s", name)
}

func onActionJustPressed(name string) {
	spxlog.Debug("OnActionJustPressed %s", name)
}

func onActionJustReleased(name string) {
	spxlog.Debug("OnActionJustReleased %s", name)
}

func onAxisChanged(name string, value float64) {
	spxlog.Debug("OnAxisChanged %s %f", name, value)
}

// physics
func onCollisionEnter(id int64, oid int64) {
	spxlog.Debug("OnCollisionEnter %d %d", id, oid)
}

func onCollisionStay(id int64, oid int64) {
	spxlog.Debug("OnCollisionStay %d %d", id, oid)
}

func onCollisionExit(id int64, oid int64) {
	spxlog.Debug("OnCollisionExit %d %d", id, oid)
}

func onTriggerEnter(id int64, oid int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		if other := GetSprite(Object(oid)); other != nil {
			sprite.V_OnTriggerEnter(other)
			sprite.OnTriggerEnter(other)
		}
	}
}

func onTriggerStay(id int64, oid int64) {
}

func onTriggerExit(id int64, oid int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		if other := GetSprite(Object(oid)); other != nil {
			sprite.V_OnTriggerExit(other)
			sprite.OnTriggerExit(other)
		}
	}
}

// ui
func onUiPressed(id int64) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiPressed()
		node.OnUiPressed()
	}
}

func onUiReleased(id int64) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiReleased()
		node.OnUiReleased()
	}
}

func onUiHovered(id int64) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiHovered()
		node.OnUiHovered()
	}
}

func onUiClicked(id int64) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiClick()
		node.OnUiClick()
	}
}

func onUiToggle(id int64, isOn bool) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiToggle(isOn)
		node.OnUiToggle(isOn)
	}
}

func onUiTextChanged(id int64, text string) {
	if node := GetUINode(Object(id)); node != nil {
		node.V_OnUiTextChanged(text)
		node.OnUiTextChanged(text)
	}
}

func onUiDestroyed(id int64) {
	DeleteUINode(Object(id))
}

func onSpriteScreenEntered(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnScreenEntered()
		sprite.OnScreenEntered()
	}
}

func onSpriteScreenExited(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnScreenExited()
		sprite.OnScreenExited()
	}
}

func onSpriteVfxFinished(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnVfxFinished()
		sprite.OnVfxFinished()
	}
}

func onSpriteAnimationFinished(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnAnimationFinished()
		sprite.OnAnimationFinished()
	}
}

func onSpriteAnimationLooped(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnAnimationLooped()
		sprite.OnAnimationLooped()
	}
}

func onSpriteFrameChanged(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnFrameChanged()
		sprite.OnFrameChanged()
	}
}

func onSpriteAnimationChanged(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnAnimationChanged()
		sprite.OnAnimationChanged()
	}
}

func onSpriteFramesSetChanged(id int64) {
	if sprite := GetSprite(Object(id)); sprite != nil {
		sprite.V_OnFramesSetChanged()
		sprite.OnFramesSetChanged()
	}
}
