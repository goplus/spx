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

package coroutine

import (
	"runtime"
	"slices"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestFrameResumesMixedWaitsInRegistrationOrder(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
	})

	waits := []struct {
		name string
		wait func(Thread)
	}{
		{"future time", func(Thread) { co.Wait(1) }},
		{"loop", co.YieldLoopFor},
		{"frame", co.WaitNextFrameFor},
		{"next round", co.YieldToNextRoundFor},
		{"time", func(Thread) { co.Wait(0.01) }},
	}
	var queued, resumed []string
	gates := make([]*Latch, len(waits))
	for i, wait := range waits {
		gates[i] = co.NewLatch()
		thread := co.Create(wait.name, func(me Thread) int {
			gates[i].Wait()
			queued = append(queued, wait.name)
			if i > 0 {
				gates[i-1].Open()
			}
			for range 3 {
				wait.wait(me)
				resumed = append(resumed, wait.name)
				co.RequestRedraw()
			}
			return 0
		})
		co.JoinYieldedOrDone(thread)
	}

	// Enqueue in reverse registration order.
	gates[len(gates)-1].Open()
	co.RequestRedraw()
	co.Update()
	if want := []string{"time", "next round", "frame", "loop", "future time"}; !slices.Equal(queued, want) {
		t.Fatalf("wait order = %v, want %v", queued, want)
	}
	if len(resumed) != 0 {
		t.Fatalf("resumed before the next frame: %v", resumed)
	}
	for frame := 1; frame <= 3; frame++ {
		resumed = nil
		itime.Update(1.0/30, 30)
		co.Update()
		if want := []string{"loop", "frame", "next round", "time"}; !slices.Equal(resumed, want) {
			t.Fatalf("frame %d: resume order = %v, want %v", frame, resumed, want)
		}
	}
}

func TestFrameWaitsForBatchFirstSlicesBeforeNextScript(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
	})

	var order []string
	parent := co.Create("parent", func(Thread) int {
		co.WaitNextFrame()
		order = append(order, "parent")
		co.StartBatch([]Task{
			{Run: func(Thread) {
				order = append(order, "first handler")
				co.WaitNextFrame()
			}},
			{Run: func(Thread) {
				order = append(order, "second handler")
				co.WaitNextFrame()
			}},
		}, BatchWaitFirstSlice)
		order = append(order, "parent continued")
		return 0
	})
	co.JoinYieldedOrDone(parent)
	sibling := co.Create("sibling", func(Thread) int {
		co.WaitNextFrame()
		order = append(order, "sibling")
		return 0
	})
	co.JoinYieldedOrDone(sibling)
	co.Update()
	itime.Update(1.0/30, 30)
	co.Update()

	if want := []string{"parent", "first handler", "second handler", "parent continued", "sibling"}; !slices.Equal(order, want) {
		t.Fatalf("script order = %v, want %v", order, want)
	}
}
