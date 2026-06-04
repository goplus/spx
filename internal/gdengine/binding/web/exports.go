//go:build js && wasm

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

package webffi

// Direct wasm exports bypass js.FuncOf and the generic event dispatcher for
// non-string callback entry from JS into Go.

func directBool(value uint32) bool {
	return value != 0
}

//go:wasmexport gdspx_on_engine_update
func GdspxOnEngineUpdate(delta float64) {
	SyncWebInputSnapshot()
	if callbacks.OnEngineUpdate != nil {
		callbacks.OnEngineUpdate(delta)
	}
}

//go:wasmexport gdspx_on_engine_fixed_update
func GdspxOnEngineFixedUpdate(delta float64) {
	SyncWebInputSnapshot()
	if callbacks.OnEngineFixedUpdate != nil {
		callbacks.OnEngineFixedUpdate(delta)
	}
}

//go:wasmexport gdspx_on_engine_destroy
func GdspxOnEngineDestroy() {
	if callbacks.OnEngineDestroy != nil {
		callbacks.OnEngineDestroy()
	}
}

//go:wasmexport gdspx_on_engine_reset
func GdspxOnEngineReset() {
	if callbacks.OnEngineReset != nil {
		callbacks.OnEngineReset()
	}
}

//go:wasmexport gdspx_on_engine_pause
func GdspxOnEnginePause(isOn uint32) {
	if callbacks.OnEnginePause != nil {
		callbacks.OnEnginePause(directBool(isOn))
	}
}

//go:wasmexport gdspx_on_sprite_ready
func GdspxOnSpriteReady(obj int64) {
	if callbacks.OnSpriteReady != nil {
		callbacks.OnSpriteReady(obj)
	}
}

//go:wasmexport gdspx_on_sprite_updated
func GdspxOnSpriteUpdated(delta float64) {
	if callbacks.OnSpriteUpdated != nil {
		callbacks.OnSpriteUpdated(delta)
	}
}

//go:wasmexport gdspx_on_sprite_fixed_updated
func GdspxOnSpriteFixedUpdated(delta float64) {
	if callbacks.OnSpriteFixedUpdated != nil {
		callbacks.OnSpriteFixedUpdated(delta)
	}
}

//go:wasmexport gdspx_on_sprite_destroyed
func GdspxOnSpriteDestroyed(obj int64) {
	if callbacks.OnSpriteDestroyed != nil {
		callbacks.OnSpriteDestroyed(obj)
	}
}

//go:wasmexport gdspx_on_sprite_frames_set_changed
func GdspxOnSpriteFramesSetChanged(obj int64) {
	if callbacks.OnSpriteFramesSetChanged != nil {
		callbacks.OnSpriteFramesSetChanged(obj)
	}
}

//go:wasmexport gdspx_on_sprite_animation_changed
func GdspxOnSpriteAnimationChanged(obj int64) {
	if callbacks.OnSpriteAnimationChanged != nil {
		callbacks.OnSpriteAnimationChanged(obj)
	}
}

//go:wasmexport gdspx_on_sprite_frame_changed
func GdspxOnSpriteFrameChanged(obj int64) {
	if callbacks.OnSpriteFrameChanged != nil {
		callbacks.OnSpriteFrameChanged(obj)
	}
}

//go:wasmexport gdspx_on_sprite_animation_looped
func GdspxOnSpriteAnimationLooped(obj int64) {
	if callbacks.OnSpriteAnimationLooped != nil {
		callbacks.OnSpriteAnimationLooped(obj)
	}
}

//go:wasmexport gdspx_on_sprite_animation_finished
func GdspxOnSpriteAnimationFinished(obj int64) {
	if callbacks.OnSpriteAnimationFinished != nil {
		callbacks.OnSpriteAnimationFinished(obj)
	}
}

//go:wasmexport gdspx_on_sprite_vfx_finished
func GdspxOnSpriteVfxFinished(obj int64) {
	if callbacks.OnSpriteVfxFinished != nil {
		callbacks.OnSpriteVfxFinished(obj)
	}
}

//go:wasmexport gdspx_on_sprite_screen_exited
func GdspxOnSpriteScreenExited(obj int64) {
	if callbacks.OnSpriteScreenExited != nil {
		callbacks.OnSpriteScreenExited(obj)
	}
}

//go:wasmexport gdspx_on_sprite_screen_entered
func GdspxOnSpriteScreenEntered(obj int64) {
	if callbacks.OnSpriteScreenEntered != nil {
		callbacks.OnSpriteScreenEntered(obj)
	}
}

//go:wasmexport gdspx_on_mouse_pressed
func GdspxOnMousePressed(keyID int64) {
	if callbacks.OnMousePressed != nil {
		callbacks.OnMousePressed(keyID)
	}
}

//go:wasmexport gdspx_on_mouse_released
func GdspxOnMouseReleased(keyID int64) {
	if callbacks.OnMouseReleased != nil {
		callbacks.OnMouseReleased(keyID)
	}
}

//go:wasmexport gdspx_on_key_pressed
func GdspxOnKeyPressed(keyID int64) {
	RecordWebKeyState(keyID, true)
	if callbacks.OnKeyPressed != nil {
		callbacks.OnKeyPressed(keyID)
	}
}

//go:wasmexport gdspx_on_key_released
func GdspxOnKeyReleased(keyID int64) {
	RecordWebKeyState(keyID, false)
	if callbacks.OnKeyReleased != nil {
		callbacks.OnKeyReleased(keyID)
	}
}

//go:wasmexport gdspx_on_collision_enter
func GdspxOnCollisionEnter(selfID int64, otherID int64) {
	if callbacks.OnCollisionEnter != nil {
		callbacks.OnCollisionEnter(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_collision_stay
func GdspxOnCollisionStay(selfID int64, otherID int64) {
	if callbacks.OnCollisionStay != nil {
		callbacks.OnCollisionStay(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_collision_exit
func GdspxOnCollisionExit(selfID int64, otherID int64) {
	if callbacks.OnCollisionExit != nil {
		callbacks.OnCollisionExit(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_trigger_enter
func GdspxOnTriggerEnter(selfID int64, otherID int64) {
	if callbacks.OnTriggerEnter != nil {
		callbacks.OnTriggerEnter(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_trigger_stay
func GdspxOnTriggerStay(selfID int64, otherID int64) {
	if callbacks.OnTriggerStay != nil {
		callbacks.OnTriggerStay(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_trigger_exit
func GdspxOnTriggerExit(selfID int64, otherID int64) {
	if callbacks.OnTriggerExit != nil {
		callbacks.OnTriggerExit(selfID, otherID)
	}
}

//go:wasmexport gdspx_on_ui_ready
func GdspxOnUiReady(obj int64) {
	if callbacks.OnUiReady != nil {
		callbacks.OnUiReady(obj)
	}
}

//go:wasmexport gdspx_on_ui_updated
func GdspxOnUiUpdated(obj int64) {
	if callbacks.OnUiUpdated != nil {
		callbacks.OnUiUpdated(obj)
	}
}

//go:wasmexport gdspx_on_ui_destroyed
func GdspxOnUiDestroyed(obj int64) {
	if callbacks.OnUiDestroyed != nil {
		callbacks.OnUiDestroyed(obj)
	}
}

//go:wasmexport gdspx_on_ui_pressed
func GdspxOnUiPressed(obj int64) {
	if callbacks.OnUiPressed != nil {
		callbacks.OnUiPressed(obj)
	}
}

//go:wasmexport gdspx_on_ui_released
func GdspxOnUiReleased(obj int64) {
	if callbacks.OnUiReleased != nil {
		callbacks.OnUiReleased(obj)
	}
}

//go:wasmexport gdspx_on_ui_hovered
func GdspxOnUiHovered(obj int64) {
	if callbacks.OnUiHovered != nil {
		callbacks.OnUiHovered(obj)
	}
}

//go:wasmexport gdspx_on_ui_clicked
func GdspxOnUiClicked(obj int64) {
	if callbacks.OnUiClicked != nil {
		callbacks.OnUiClicked(obj)
	}
}

//go:wasmexport gdspx_on_ui_toggle
func GdspxOnUiToggle(obj int64, isOn uint32) {
	if callbacks.OnUiToggle != nil {
		callbacks.OnUiToggle(obj, directBool(isOn))
	}
}
