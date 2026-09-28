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
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

// ============================================================================
// Animation Audio
// ============================================================================

func (a *animationComponent) playAnimationAudio(ani *coreproject.AniConfig, state *animState) {
	if state == nil {
		return
	}
	if action := ani.OnStart; action != nil && action.Play != "" {
		state.OnStartReplayAudioName = action.Play
		a.sprite.sound().play(action.Play, false)
	}
	if action := ani.OnPlay; action != nil && action.Play != "" {
		state.OnPlayReplayAudioName = action.Play
		a.restartOnPlayAudio(state)
	}
}

func (a *animationComponent) restartOnPlayAudio(state *animState) {
	if state == nil {
		return
	}

	engine.Lock()
	name := state.OnPlayReplayAudioName
	nextID := state.OnPlayAudioPlaybackID
	canceled := state.IsCanceled
	engine.Unlock()

	if canceled || name == "" {
		return
	}

	if !a.sprite.g.soundMgr.RestartID(nextID) {
		nextID = a.sprite.sound().play(name, true)
	}
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
		a.sprite.g.soundMgr.StopID(nextID)
	}
}

func (a *animationComponent) stopAnimationAudio(id int64) {
	if id == 0 || a.sprite == nil || a.sprite.g == nil {
		return
	}
	gco.CleanupMainThread(func() {
		if !a.sprite.g.soundMgr.IsPlaying(id) {
			a.sprite.g.soundMgr.PruneStoppedIDs([]int64{id})
			return
		}
		a.sprite.g.soundMgr.StopID(id)
	})
}

// ============================================================================
// Pending Replay State
// ============================================================================

func (a *animationComponent) takePendingOnPlayAudioStates() (states [2]*animState) {
	states = [2]*animState{a.curAnimState, a.getCurTweenState()}
	for i, state := range states {
		if state == nil || !state.OnPlayAudioRestartPending {
			states[i] = nil
			continue
		}
		state.OnPlayAudioRestartPending = false
		if state.IsCanceled || state.OnPlayReplayAudioName == "" {
			states[i] = nil
		}
	}
	return
}
