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
	"fmt"
	"slices"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

func TestAtFrameSchedulesCallbackForActiveGame(t *testing.T) {
	co := setupRuntimeScheduler(t)

	ran := false
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := engine.CurrentFrame()

	AtFrame(base+1, func() {
		ran = true
	})
	if ran {
		t.Fatal("AtFrame ran callback before target frame")
	}

	itime.Update(0, 0)
	engine.RunFrameCallbacks()
	co.Update()

	if !ran {
		t.Fatal("AtFrame did not run callback at target frame")
	}
}

func TestCurrentFrameSurvivesGameReload(t *testing.T) {
	base := itime.Frame()
	if got := CurrentFrame(); got != base {
		t.Fatalf("CurrentFrame = %d, want engine frame %d", got, base)
	}
	itime.Update(0, 0)
	if got := CurrentFrame(); got != base+1 {
		t.Fatalf("CurrentFrame after update = %d, want %d", got, base+1)
	}
	itime.OnReload()
	if got := CurrentFrame(); got != base+1 {
		t.Fatalf("CurrentFrame after reload = %d, want %d", got, base+1)
	}
}

func TestAtFrameWithoutGameRunsSynchronously(t *testing.T) {
	setupSnapshotInputSessionTest(t)
	var calls []string
	AtFrame(CurrentFrame()+100, func() { calls = append(calls, "callback") })
	calls = append(calls, "returned")
	AtFrame(CurrentFrame(), nil)
	engine.RunFrameCallbacks()
	if want := []string{"callback", "returned"}; !slices.Equal(calls, want) {
		t.Fatalf("callback order = %v, want %v", calls, want)
	}
}

func TestAtFrameAlreadyReachedRunsWithoutAdvancingClock(t *testing.T) {
	for _, offset := range []int64{-1, 0} {
		t.Run(fmt.Sprintf("offset=%d", offset), func(t *testing.T) {
			setupSnapshotInputSessionTest(t)
			co := setupRuntimeScheduler(t)
			engine.SetGame(struct{}{})
			base := CurrentFrame()
			calls := 0
			AtFrame(base+offset, func() { calls++ })
			co.Update()
			if calls != 1 || CurrentFrame() != base {
				t.Fatalf("calls = %d, frame = %d; want 1 in frame %d", calls, CurrentFrame(), base)
			}
			engine.RunFrameCallbacks()
			co.Update()
			if calls != 1 {
				t.Fatalf("already reached callback ran %d times, want once", calls)
			}
		})
	}
}

func TestAtFrameCallbackCanWaitForMainThread(t *testing.T) {
	co := setupRuntimeScheduler(t)

	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := itime.Frame()

	mainThreadCallRan := false
	AtFrame(base+1, func() {
		engine.WaitMainThread(func() {
			mainThreadCallRan = true
		})
	})
	itime.Update(0, 0)
	engine.RunFrameCallbacks()

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()
	select {
	case <-updateDone:
	case <-time.After(time.Second):
		t.Fatal("AtFrame callback deadlocked while waiting for the main thread")
	}
	if !mainThreadCallRan {
		t.Fatal("AtFrame callback did not complete its main-thread call")
	}
}

func TestLogicFramePreservesPhasesAndShapeSnapshot(t *testing.T) {
	setupRuntimeScheduler(t)
	itime.OnReload()
	itime.Start(nil)
	t.Cleanup(itime.OnReload)
	game := &Game{sounds: make(map[string]sound)}
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)

	newSprite := func(name string) *SpriteImpl {
		anim := newTestAnimationComponent()
		sprite := anim.sprite
		sprite.name, sprite.g = name, game
		game.sounds[name] = &coreproject.SoundConfig{Path: name + ".wav"}
		sprite.sound().pendingAudios = []string{name}
		anim.curAnimState = &animState{Name: name}
		anim.doneAnimations = []*animState{anim.curAnimState}
		return sprite
	}
	first, second, late := newSprite("first"), newSprite("second"), newSprite("late")
	destroyed, unbound := newSprite("destroyed"), newSprite("unbound")
	destroyed.markDestroyed()
	unbound.runtimeState.SyncSprite = nil
	game.shapeMgr.items = []Shape{first, struct{}{}, destroyed, second, unbound}
	var steps []string
	backend.onPlay = func(id int64) {
		path := backend.plays[len(backend.plays)-1].path
		steps = append(steps, "play:"+path)
		switch id {
		case 1:
			first.animation().curAnimState.OnPlayAudioPlaybackID = id
			// Both passes must finish the original snapshot, even if playback
			// changes the live shape list.
			game.shapeMgr.items = []Shape{late}
		case 2:
			if first.animation().curAnimState == nil {
				t.Error("first animation completed before second audio played")
			}
			second.animation().curAnimState.OnPlayAudioPlaybackID = id
		case 3:
			late.animation().curAnimState.OnPlayAudioPlaybackID = id
		}
	}
	backend.onStop = func(id int64) {
		steps = append(steps, fmt.Sprintf("complete:%d", id))
	}
	itime.RegisterTimer(0.2)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	itime.Update(0.2, 30)
	audios, animations := make([]string, 0, 4), make([]*animState, 0, 4)
	audioStorage, animationStorage := &audios[:cap(audios)][0], &animations[:cap(animations)][0]
	audios, animations = game.processLogicFrame(audios, animations)
	want := []string{"play:" + engine.ToAssetPath("first.wav"), "play:" + engine.ToAssetPath("second.wav"), "complete:1", "complete:2"}
	if !slices.Equal(steps, want) {
		t.Fatalf("logic phases = %v, want %v", steps, want)
	}
	if first.animation().curAnimState != nil || second.animation().curAnimState != nil {
		t.Fatal("original snapshot did not complete both animations")
	}
	if len(late.sound().pendingAudios) != 1 || len(late.animation().doneAnimations) != 1 {
		t.Fatal("newly added sprite was processed in the old snapshot")
	}
	for _, sprite := range []*SpriteImpl{destroyed, unbound} {
		if len(sprite.sound().pendingAudios) != 0 || len(sprite.animation().doneAnimations) != 0 || sprite.animation().curAnimState == nil {
			t.Fatalf("inactive sprite %s was not drained without playback", sprite.name)
		}
	}
	// Timers belong to the frame event phase, not the audio/animation loop.
	for _, timestamp := range []int64{100, 200} {
		if timer, ok := itime.NextTimer(); !ok || timer != timestamp {
			t.Fatalf("pending timer = %d, %v, want %d", timer, ok, timestamp)
		}
	}
	if _, ok := itime.NextTimer(); ok {
		t.Fatal("future timer became due early")
	}
	steps = nil
	audios, animations = game.processLogicFrame(audios, animations)
	want = []string{"play:" + engine.ToAssetPath("late.wav"), "complete:3"}
	if !slices.Equal(steps, want) {
		t.Fatalf("next frame = %v, want %v", steps, want)
	}
	if len(audios) != 0 || cap(audios) != 4 || &audios[:cap(audios)][0] != audioStorage ||
		len(animations) != 0 || cap(animations) != 4 || &animations[:cap(animations)][0] != animationStorage {
		t.Fatal("logic frames did not preserve both reusable scratch buffers")
	}
}

func TestLogicFramePreservesEmptyScratch(t *testing.T) {
	itime.OnReload()
	t.Cleanup(itime.OnReload)
	for _, items := range [][]Shape{nil, {struct{}{}}} {
		game := &Game{}
		game.shapeMgr.items = items
		audios, animations := game.processLogicFrame([]string{}, []*animState{})
		if audios == nil || animations == nil || len(audios) != 0 || len(animations) != 0 {
			t.Fatalf("scratch = %#v, %#v, want non-nil empty slices", audios, animations)
		}
		audios, animations = game.processLogicFrame(nil, nil)
		if audios != nil || animations != nil {
			t.Fatalf("nil scratch = %#v, %#v, want nil slices", audios, animations)
		}
	}
}
