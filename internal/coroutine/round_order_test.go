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
	"sync"
	"testing"

	itime "github.com/goplus/spx/v4/internal/time"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestFrameWaitBatchResetsScriptCursor(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })

	var order []string
	first := scheduler.Create("A", func(Thread) {
		scheduler.WaitNextFrame()
		order = append(order, "A1")
		scheduler.WaitNextFrame()
		order = append(order, "A2")
	})
	scheduler.JoinFirstSlice(first)
	second := scheduler.Create("B", func(Thread) {
		scheduler.Wait(0.5)
		order = append(order, "B")
	})
	scheduler.JoinFirstSlice(second)
	scheduler.Update()

	// Only A resumes in this frame, while B keeps waiting for its deadline.
	itime.Update(0.25, 30)
	scheduler.Update()
	if want := []string{"A1"}; !slices.Equal(order, want) {
		t.Fatalf("first frame order = %v, want %v", order, want)
	}

	// Both waits are now eligible. The new frame starts at A, before B.
	itime.Update(0.25, 30)
	scheduler.Update()
	if want := []string{"A1", "A2", "B"}; !slices.Equal(order, want) {
		t.Fatalf("second frame order = %v, want %v", order, want)
	}
}

func TestWaitMainThreadKeepsAdmittedRoundOrder(t *testing.T) {
	previousPlatform := gdx.PlatformMgr
	gdx.PlatformMgr = nil
	t.Cleanup(func() { gdx.PlatformMgr = previousPlatform })
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })

	var mu sync.Mutex
	var order []string
	record := func(event string) {
		mu.Lock()
		order = append(order, event)
		mu.Unlock()
	}
	var second Thread
	first := scheduler.Create("A", func(Thread) {
		scheduler.WaitNextFrame()
		record("A-before")
		scheduler.WaitMainThread(func() {
			record("main")
			scheduler.schedulerMu.Lock()
			_, pending := scheduler.pendingExecution[second]
			queued := second.readyHeap != nil
			scheduler.schedulerMu.Unlock()
			if !pending || !queued {
				t.Error("B was not admitted to the same round before A's engine call")
			}
			if scheduler.runMu.TryLock() {
				scheduler.runMu.Unlock()
				t.Error("engine call released A's script slice")
			}
			runtime.Gosched()
		})
		record("A-after")
	})
	scheduler.JoinFirstSlice(first)
	second = scheduler.Create("B", func(Thread) {
		scheduler.WaitNextFrame()
		record("B")
	})
	scheduler.JoinFirstSlice(second)
	scheduler.Update()

	itime.Update(1.0/30, 30)
	scheduler.Update()
	mu.Lock()
	got := slices.Clone(order)
	mu.Unlock()
	if want := []string{"A-before", "main", "A-after", "B"}; !slices.Equal(got, want) {
		t.Fatalf("script order = %v, want %v", got, want)
	}
}
