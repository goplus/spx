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

package spx

import (
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
)

// ============================================================================
// Animation Audio
// ============================================================================
// This file manages animation-bound audio playback and replay state.

// ============================================================================
// Audio Playback
// ============================================================================

func (a *animationComponent) playAnimationAudio(ani *coreproject.AniConfig, state *animState) {
	a.playOnStartAudio(ani.OnStart, state)
	a.playOnPlayAudio(ani.OnPlay, state)
}

func (a *animationComponent) playOnStartAudio(action *coreproject.ActionConfig, state *animState) {
	if action == nil || action.Play == "" || state == nil {
		return
	}
	state.OnStartReplayAudioName = action.Play
	a.sprite.playAudio(action.Play, false)
}

func (a *animationComponent) playOnPlayAudio(action *coreproject.ActionConfig, state *animState) {
	if action == nil || action.Play == "" || state == nil {
		return
	}
	state.OnPlayReplayAudioName = action.Play
	a.restartOnPlayAudio(state)
}

func (a *animationComponent) stopAnimationAudio(state *animState) {
	if state == nil {
		return
	}
	a.stopOnPlayAudio(state)
}

func (a *animationComponent) restartOnPlayAudio(state *animState) {
	if state == nil {
		return
	}

	engine.Lock()
	name := state.OnPlayReplayAudioName
	prevID := state.OnPlayAudioPlaybackID
	canceled := state.IsCanceled
	engine.Unlock()

	if canceled || name == "" {
		return
	}

	nextID := a.sprite.restartOrPlayLoopedAudio(name, prevID)
	if nextID == 0 {
		return
	}

	engine.Lock()
	canceled = state.IsCanceled
	if !canceled {
		state.OnPlayAudioPlaybackID = nextID
	}
	engine.Unlock()

	if canceled {
		a.sprite.stopAudioPlayback(nextID)
	}
}

func (a *animationComponent) stopOnPlayAudio(state *animState) {
	if state == nil {
		return
	}

	engine.Lock()
	state.OnPlayAudioRestartPending = false
	id := state.OnPlayAudioPlaybackID
	state.OnPlayAudioPlaybackID = 0
	engine.Unlock()

	if id == 0 || a.sprite == nil || a.sprite.g == nil {
		return
	}
	if !a.sprite.g.soundMgr.IsPlaying(id) {
		a.sprite.g.soundMgr.PruneStoppedIDs([]int64{id})
		return
	}
	a.sprite.stopAudioPlayback(id)
}

func (a *animationComponent) markOnPlayAudioRestartPending(state *animState) {
	if state == nil || state.OnPlayReplayAudioName == "" {
		return
	}
	state.OnPlayAudioRestartPending = true
}

// ============================================================================
// Pending Replay State
// ============================================================================

func (a *animationComponent) takePendingOnPlayAudioStates(buffer []*animState) []*animState {
	buffer = a.takePendingOnPlayAudioState(buffer[:0], a.curAnimState)
	if a.curTweenState != a.curAnimState {
		buffer = a.takePendingOnPlayAudioState(buffer, a.curTweenState)
	}
	return buffer
}

func (a *animationComponent) takePendingOnPlayAudioState(buffer []*animState, state *animState) []*animState {
	if state == nil || !state.OnPlayAudioRestartPending {
		return buffer
	}
	state.OnPlayAudioRestartPending = false
	if state.IsCanceled || state.OnPlayReplayAudioName == "" {
		return buffer
	}
	return append(buffer, state)
}
