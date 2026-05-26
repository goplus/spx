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
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (a *animationComponent) playAnimationAudio(ani *coreproject.AniConfig, state *animState) {
	a.playOnStartAudio(ani.OnStart, state)
	a.playOnPlayAudio(ani.OnPlay, state)
}

func (a *animationComponent) playOnStartAudio(action *coreproject.ActionConfig, state *animState) {
	if action == nil || action.Play == "" || state == nil {
		return
	}
	if action.Loop != nil && *action.Loop {
		spxlog.Warn("Animation onStart only supports loop=false, ignoring loop=true for %s", action.Play)
	}
	state.OnStartReplayAudioName = action.Play
	a.sprite.playAudio(action.Play, false)
}

func (a *animationComponent) playOnPlayAudio(action *coreproject.ActionConfig, state *animState) {
	if action == nil || action.Play == "" || state == nil {
		return
	}
	if action.Loop != nil && !*action.Loop {
		spxlog.Warn("Animation onPlay only supports loop=true, forcing looped playback for %s within animation cycles", action.Play)
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
	if state == nil || state.OnPlayReplayAudioName == "" {
		return
	}
	state.OnPlayAudioPlaybackID = a.sprite.restartOrPlayLoopedAudio(state.OnPlayReplayAudioName, state.OnPlayAudioPlaybackID)
}

func (a *animationComponent) stopOnPlayAudio(state *animState) {
	if state == nil {
		return
	}
	state.OnPlayAudioRestartPending = false
	id := state.OnPlayAudioPlaybackID
	state.OnPlayAudioPlaybackID = 0
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
