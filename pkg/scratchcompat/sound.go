/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package scratchcompat

import (
	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/internal/engine"
)

// VolumeSetter is implemented by Scratch targets that support sound volume.
type VolumeSetter interface {
	SetVolume(volume float64)
}

// SetVolume updates the target's volume, then suspends the calling SPX script
// until the next frame, matching Scratch's sound block Promise. This wait also
// applies in run-without-screen-refresh mode. Outside a script it returns
// immediately after the update.
func SetVolume(target VolumeSetter, volume float64) {
	target.SetVolume(volume)
	waitSoundUpdate()
}

// VolumeChanger is implemented by Scratch targets that support changing sound
// volume.
type VolumeChanger interface {
	ChangeVolume(delta float64)
}

// ChangeVolume changes the target's volume with the same wait as [SetVolume].
func ChangeVolume(target VolumeChanger, delta float64) {
	target.ChangeVolume(delta)
	waitSoundUpdate()
}

// SoundEffectSetter is implemented by Scratch targets that support sound
// effects.
type SoundEffectSetter interface {
	SetSoundEffect(kind spx.SoundEffectKind, value float64)
}

// SetSoundEffect updates a sound effect with the same wait as [SetVolume].
func SetSoundEffect(target SoundEffectSetter, kind spx.SoundEffectKind, value float64) {
	target.SetSoundEffect(kind, value)
	waitSoundUpdate()
}

// SoundEffectChanger is implemented by Scratch targets that support changing
// sound effects.
type SoundEffectChanger interface {
	ChangeSoundEffect(kind spx.SoundEffectKind, delta float64)
}

// ChangeSoundEffect changes a sound effect with the same wait as [SetVolume].
func ChangeSoundEffect(target SoundEffectChanger, kind spx.SoundEffectKind, delta float64) {
	target.ChangeSoundEffect(kind, delta)
	waitSoundUpdate()
}

func waitSoundUpdate() {
	if engine.IsInCoroutine() {
		engine.WaitNextFrame()
	}
}
