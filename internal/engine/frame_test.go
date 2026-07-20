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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"
	itime "github.com/goplus/spx/v2/internal/time"
)

func advanceEngineFrameTo(t *testing.T, target int64) {
	t.Helper()
	if current := itime.Frame(); current > target {
		t.Fatalf("engine frame %d already passed target %d", current, target)
	}
	for itime.Frame() < target {
		itime.Update(0, 0)
	}
}

func TestFrameTimelineUsesEngineFrame(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()

	base := itime.Frame()
	var got []int64
	ScheduleFrame(base+1, func() { got = append(got, base+1) })
	ScheduleFrame(base, func() { got = append(got, base) })
	if len(got) != 1 || got[0] != base {
		t.Fatalf("callback at current engine frame = %v, want [%d]", got, base)
	}
	if frame := CurrentFrame(); frame != base {
		t.Fatalf("CurrentFrame = %d, want engine frame %d", frame, base)
	}

	advanceEngineFrameTo(t, base+1)
	RunFrameCallbacks()
	want := []int64{base, base + 1}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("engine frame callbacks = %v, want %v", got, want)
	}
}

func TestBootstrapCaptureUsesEngineFrameMetadata(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()

	var got CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	frame := itime.Frame()
	ScheduleFrame(frame, func() {
		if err := EnqueueCapture("bootstrap"); err != nil {
			t.Fatal(err)
		}
	})
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.Name != "bootstrap" || got.Frame != frame {
		t.Fatalf("bootstrap capture = %+v, want engine frame %d", got, frame)
	}
	if got.InputTick != nil {
		t.Fatalf("bootstrap capture input tick = %v, want nil", *got.InputTick)
	}
}

func TestCaptureCanCarryInputTickMetadata(t *testing.T) {
	SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()

	var got CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := EnqueueCaptureAtInputTick("tick", 0); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 0 {
		t.Fatalf("capture input tick = %v, want 0", got.InputTick)
	}
	if got.Frame != CurrentFrame() {
		t.Fatalf("capture engine frame = %d, want %d", got.Frame, CurrentFrame())
	}
}

func TestFrameCallbackRunsInCapturedCoroutineAndMayYield(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		co.AbortAllAndWait(time.Second)
		SetCoroutines(original)
	})

	game := &struct{ name string }{name: "game"}
	owner := &struct{ name string }{name: "sprite"}
	SetGame(game)
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := itime.Frame()

	type observation struct {
		inCoroutine bool
		owner       any
	}
	observed := make(chan observation, 1)
	done := make(chan struct{})
	registered := make(chan struct{})
	source := co.CreateAndStart(false, owner, func(coroutine.Thread) int {
		ScheduleFrame(base+1, func() {
			observed <- observation{
				inCoroutine: IsInCoroutine(),
				owner:       GetCoroutineOwner(),
			}
			WaitYield()
			close(done)
		})
		close(registered)
		return 0
	})
	<-registered
	co.Join(source)

	advanceEngineFrameTo(t, base+1)
	RunFrameCallbacks()

	got := <-observed
	if !got.inCoroutine {
		t.Fatal("frame callback did not run in a coroutine")
	}
	if got.owner != owner {
		t.Fatalf("frame callback owner = %v, want captured owner %v", got.owner, owner)
	}
	select {
	case <-done:
		t.Fatal("yielding frame callback completed before the scheduler resumed it")
	default:
	}

	co.Update()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("yielding frame callback did not resume")
	}
}

func TestFrameCallbackSkipsExplicitlyStoppedRegistration(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		co.AbortAllAndWait(time.Second)
		SetCoroutines(original)
	})

	owner := &struct{ name string }{name: "sprite"}
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := itime.Frame()

	ran := false
	registered := make(chan struct{})
	source := co.CreateAndStart(false, owner, func(me coroutine.Thread) int {
		ScheduleFrame(base+1, func() { ran = true })
		close(registered)
		co.WaitYield(me)
		return 0
	})
	<-registered
	co.JoinYieldedOrDone(source)
	co.StopIf(func(candidate coroutine.Thread) bool { return candidate == source })
	co.Join(source)

	advanceEngineFrameTo(t, base+1)
	RunFrameCallbacks()
	if ran {
		t.Fatal("callback registered by an explicitly stopped coroutine ran")
	}
}

func TestConcurrentCaptureMetadataIsCoherentAndOrdered(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	expectedFrame := CurrentFrame()

	var got []CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	defer SetCaptureHandler(nil)

	const count = 32
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			if err := EnqueueCapture("capture"); err != nil {
				t.Errorf("EnqueueCapture(%d): %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if !HasPendingCaptures() {
		t.Fatal("HasPendingCaptures = false with queued captures")
	}
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if HasPendingCaptures() {
		t.Fatal("HasPendingCaptures = true after flush")
	}
	if len(got) != count {
		t.Fatalf("capture count = %d, want %d", len(got), count)
	}
	for i, req := range got {
		if req.Frame != expectedFrame {
			t.Fatalf("capture[%d].Frame = %d, want %d", i, req.Frame, expectedFrame)
		}
		if req.Sequence != uint64(i+1) {
			t.Fatalf("capture[%d].Sequence = %d, want %d", i, req.Sequence, i+1)
		}
	}
}

func TestConcurrentFrameRegistrationAndEngineUpdates(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()

	const count = 256
	const futureFrame = int64(^uint64(0) >> 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			itime.Update(0, 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			ScheduleFrame(futureFrame, func() {})
		}
	}()
	wg.Wait()
}

func TestFlushCapturesContinuesAfterHandlerError(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()

	firstErr := errors.New("first capture failed")
	thirdErr := errors.New("third capture failed")
	var got []string
	SetCaptureHandler(func(req CaptureRequest) error {
		got = append(got, req.Name)
		switch req.Name {
		case "first":
			return firstErr
		case "third":
			return thirdErr
		default:
			return nil
		}
	})
	defer SetCaptureHandler(nil)

	for _, name := range []string{"first", "second", "third"} {
		if err := EnqueueCapture(name); err != nil {
			t.Fatal(err)
		}
	}
	err := FlushCaptures()
	if !errors.Is(err, firstErr) || !errors.Is(err, thirdErr) {
		t.Fatalf("FlushCaptures error = %v, want both handler errors", err)
	}
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("dispatched captures = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dispatched captures = %v, want %v", got, want)
		}
	}
}

func TestFrameCallbacksRunAtTargetFrameOnce(t *testing.T) {
	var got []string
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+3, func() {
		got = append(got, "frame3")
	})
	ScheduleFrame(base+2, func() {
		got = append(got, "frame2")
	})

	advanceEngineFrameTo(t, base+1)
	RunFrameCallbacks()
	if len(got) != 0 {
		t.Fatalf("RunFrameCallbacks ran callbacks early: %v", got)
	}

	advanceEngineFrameTo(t, base+2)
	RunFrameCallbacks()
	RunFrameCallbacks()
	if len(got) != 1 || got[0] != "frame2" {
		t.Fatalf("RunFrameCallbacks = %v, want [frame2]", got)
	}

	advanceEngineFrameTo(t, base+3)
	RunFrameCallbacks()
	if len(got) != 2 || got[1] != "frame3" {
		t.Fatalf("RunFrameCallbacks = %v, want [frame2 frame3]", got)
	}
}

func TestFrameCallbacksRunMissedFramesInFrameOrder(t *testing.T) {
	var got []string
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+5, func() {
		got = append(got, "frame5-a")
	})
	ScheduleFrame(base+3, func() {
		got = append(got, "frame3")
	})
	ScheduleFrame(base+5, func() {
		got = append(got, "frame5-b")
	})

	advanceEngineFrameTo(t, base+6)
	RunFrameCallbacks()

	want := []string{"frame3", "frame5-a", "frame5-b"}
	if len(got) != len(want) {
		t.Fatalf("RunFrameCallbacks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RunFrameCallbacks = %v, want %v", got, want)
		}
	}
}

func TestResetFrameRuntimeClearsPendingWork(t *testing.T) {
	ran := false
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+1, func() {
		ran = true
	})
	if err := EnqueueCapture("queued.png"); err != nil {
		t.Fatal(err)
	}
	ResetFrameRuntime()

	advanceEngineFrameTo(t, base+1)
	RunFrameCallbacks()
	if ran {
		t.Fatal("ResetFrameRuntime did not clear pending callbacks")
	}

	SetCaptureHandler(func(req CaptureRequest) error {
		t.Fatalf("ResetFrameRuntime did not clear pending capture %q", req.Name)
		return nil
	})
	defer SetCaptureHandler(nil)
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRequestQueuesForActiveGame(t *testing.T) {
	var got CaptureRequest
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	expectedFrame := CurrentFrame()
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := EnqueueCapture("step_001.png"); err != nil {
		t.Fatal(err)
	}
	if got.Name != "" {
		t.Fatalf("EnqueueCapture ran immediately: %+v", got)
	}

	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.Name != "step_001.png" {
		t.Fatalf("capture name = %q, want step_001.png", got.Name)
	}
	if got.Frame != expectedFrame {
		t.Fatalf("capture frame = %d, want %d", got.Frame, expectedFrame)
	}
	if got.InputTick != nil {
		t.Fatalf("capture input tick = %v, want nil", *got.InputTick)
	}
	if got.Sequence == 0 {
		t.Fatal("capture sequence was not assigned")
	}
}
