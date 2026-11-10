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
	"fmt"
	"runtime"
	"slices"
	"testing"

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestFrameResumesMixedWaitsInRegistrationOrder(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() {
		scheduler.RunAfterStop(nil)
	})

	waits := []struct {
		name string
		wait func(Thread)
	}{
		{"future time", func(Thread) { scheduler.Wait(1) }},
		{"loop", scheduler.YieldLoopFor},
		{"frame", scheduler.WaitNextFrameFor},
		{"next round", scheduler.YieldToNextRoundFor},
		{"time", func(Thread) { scheduler.Wait(0.01) }},
	}
	var queued, resumed []string
	gates := make([]*Latch, len(waits))
	for i, wait := range waits {
		gates[i] = scheduler.NewLatch()
		thread := scheduler.Create(wait.name, func(me Thread) {
			gates[i].Wait()
			queued = append(queued, wait.name)
			if i > 0 {
				gates[i-1].Open()
			}
			for range 3 {
				wait.wait(me)
				resumed = append(resumed, wait.name)
				scheduler.RequestRedraw()
			}
		})
		scheduler.JoinFirstSlice(thread)
	}

	// Enqueue in reverse registration order.
	gates[len(gates)-1].Open()
	scheduler.RequestRedraw()
	scheduler.Update()
	if want := []string{"time", "next round", "frame", "loop", "future time"}; !slices.Equal(queued, want) {
		t.Fatalf("wait order = %v, want %v", queued, want)
	}
	if len(resumed) != 0 {
		t.Fatalf("resumed before the next frame: %v", resumed)
	}
	for frame := 1; frame <= 3; frame++ {
		resumed = nil
		itime.Update(1.0/30, 30)
		scheduler.Update()
		if want := []string{"loop", "frame", "next round", "time"}; !slices.Equal(resumed, want) {
			t.Fatalf("frame %d: resume order = %v, want %v", frame, resumed, want)
		}
	}
}

func TestFrameResumesOlderScriptBeforeBatchFirstSlices(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() {
		scheduler.RunAfterStop(nil)
	})

	var order []string
	parent := scheduler.Create("parent", func(Thread) {
		scheduler.WaitNextFrame()
		order = append(order, "parent")
		scheduler.StartBatch([]Task{
			{Run: func(Thread) {
				order = append(order, "first handler")
				scheduler.WaitNextFrame()
			}},
			{Run: func(Thread) {
				order = append(order, "second handler")
				scheduler.WaitNextFrame()
			}},
		}, BatchWaitFirstSlice)
		order = append(order, "parent continued")
	})
	scheduler.JoinFirstSlice(parent)
	sibling := scheduler.Create("sibling", func(Thread) {
		scheduler.WaitNextFrame()
		order = append(order, "sibling")
	})
	scheduler.JoinFirstSlice(sibling)
	scheduler.Update()
	itime.Update(1.0/30, 30)
	scheduler.Update()

	// Waiting for the new batch yields to the older sibling already admitted
	// in this round; the parent still resumes only after both first slices.
	if want := []string{"parent", "sibling", "first handler", "second handler", "parent continued"}; !slices.Equal(order, want) {
		t.Fatalf("script order = %v, want %v", order, want)
	}
}

func TestScriptOrderAcrossRoundsWithAddedAndCanceledPeers(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	var order []string
	var peers []Thread
	var added Thread
	cleaned := 0
	var run func(string) func(Thread)
	run = func(name string) func(Thread) {
		return func(me Thread) {
			if name == "b" {
				defer func() { cleaned++ }()
			}
			for turn := range 3 {
				order = append(order, fmt.Sprintf("%s%d", name, turn))
				if turn == 0 {
					switch name {
					case "a":
						added = scheduler.Create("d", run("d"))
					case "c":
						scheduler.Stop(peers[1])
					}
				}
				if turn < 2 {
					scheduler.Sched(me)
				}
			}
		}
	}
	caller := scheduler.Create("caller", func(Thread) {
		peers = scheduler.StartBatch([]Task{
			{Run: run("a")}, {Run: run("b")}, {Run: run("c")},
		}, BatchAsync)
		scheduler.JoinAll(peers)
		scheduler.Join(added)
	})
	waitForThreadSignal(t, caller.done, "script rounds did not complete without Update")
	if want := []string{"a0", "b0", "c0", "d0", "a1", "c1", "d1", "a2", "c2", "d2"}; !slices.Equal(order, want) {
		t.Fatalf("script order = %v, want %v", order, want)
	}
	if cleaned != 1 {
		t.Fatalf("canceled peer cleanup count = %d, want 1", cleaned)
	}
}

func TestPendingRestartsKeepPositionWhileRetiredInvocationsCleanUp(t *testing.T) {
	for _, tt := range []struct {
		name            string
		separateBatches bool
	}{
		{name: "same batch"},
		{name: "separate batches", separateBatches: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := New(nil)
			t.Cleanup(func() { scheduler.RunAfterStop(nil) })
			state := NewHandlerState(RestartExisting)
			blocked := scheduler.NewLatch()
			releaseSibling := scheduler.NewLatch()
			var order []string
			cleaned := make(map[string]int)
			task := func(name string) Task {
				return Task{
					RegisterInvocation: func(thread Thread) func() {
						retire := state.RegisterInvocation(thread)
						return func() {
							retire()
							cleaned[name]++
						}
					},
					Run: func(Thread) {
						order = append(order, name)
						if name == "original" {
							blocked.Wait()
						}
					},
				}
			}
			original := scheduler.StartBatch([]Task{task("original")}, BatchAsync)[0]
			scheduler.JoinFirstSlice(original)
			sibling := scheduler.Create("sibling", func(Thread) {
				releaseSibling.Wait()
				order = append(order, "sibling")
			})
			scheduler.JoinFirstSlice(sibling)
			caller := scheduler.Create("caller", func(Thread) {
				// Both retired invocations still need cleanup when their shared
				// script position is handed to the latest invocation.
				tasks := []Task{task("obsolete"), task("latest")}
				var replacements []Thread
				if tt.separateBatches {
					// Publish the obsolete invocation before replacing it while
					// the caller still holds execution, so both remain queued.
					for _, replacement := range tasks {
						replacements = append(replacements, scheduler.StartBatch([]Task{replacement}, BatchAsync)...)
					}
				} else {
					replacements = scheduler.StartBatch(tasks, BatchAsync)
				}
				releaseSibling.Open()
				scheduler.JoinAll(replacements)
			})
			waitForThreadSignal(t, caller.done, "replacement batch did not finish")
			waitForThreadSignal(t, original.done, "original invocation did not clean up")
			waitForThreadSignal(t, sibling.done, "sibling did not resume")
			if want := []string{"original", "latest", "sibling"}; !slices.Equal(order, want) {
				t.Fatalf("restart order = %v, want %v", order, want)
			}
			for _, name := range []string{"original", "obsolete", "latest"} {
				if cleaned[name] != 1 {
					t.Fatalf("%s invocation cleanup count = %d, want 1", name, cleaned[name])
				}
			}
		})
	}
}
