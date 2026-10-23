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
	co := New(nil)
	itime.Start(nil)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
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
		thread := co.Create(wait.name, func(me Thread) {
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
		})
		co.JoinFirstSlice(thread)
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
	itime.Start(nil)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})

	var order []string
	parent := co.Create("parent", func(Thread) {
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
	})
	co.JoinFirstSlice(parent)
	sibling := co.Create("sibling", func(Thread) {
		co.WaitNextFrame()
		order = append(order, "sibling")
	})
	co.JoinFirstSlice(sibling)
	co.Update()
	itime.Update(1.0/30, 30)
	co.Update()

	if want := []string{"parent", "first handler", "second handler", "parent continued", "sibling"}; !slices.Equal(order, want) {
		t.Fatalf("script order = %v, want %v", order, want)
	}
}

func TestScriptOrderAcrossRoundsWithAddedAndCanceledPeers(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
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
						added = co.Create("d", run("d"))
					case "c":
						co.Stop(peers[1])
					}
				}
				if turn < 2 {
					co.Sched(me)
				}
			}
		}
	}
	caller := co.Create("caller", func(Thread) {
		peers = co.StartBatch([]Task{
			{Run: run("a")}, {Run: run("b")}, {Run: run("c")},
		}, BatchAsync)
		co.JoinAll(peers)
		co.Join(added)
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
			co := New(nil)
			t.Cleanup(func() { co.RunAfterStop(nil) })
			state := NewHandlerState(RestartExisting)
			blocked := co.NewLatch()
			releaseSibling := co.NewLatch()
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
			original := co.StartBatch([]Task{task("original")}, BatchAsync)[0]
			co.JoinFirstSlice(original)
			sibling := co.Create("sibling", func(Thread) {
				releaseSibling.Wait()
				order = append(order, "sibling")
			})
			co.JoinFirstSlice(sibling)
			caller := co.Create("caller", func(Thread) {
				// Both retired invocations still need cleanup when their shared
				// script position is handed to the latest invocation.
				tasks := []Task{task("obsolete"), task("latest")}
				var replacements []Thread
				if tt.separateBatches {
					// Publish the obsolete invocation before replacing it while
					// the caller still holds execution, so both remain queued.
					for _, replacement := range tasks {
						replacements = append(replacements, co.StartBatch([]Task{replacement}, BatchAsync)...)
					}
				} else {
					replacements = co.StartBatch(tasks, BatchAsync)
				}
				releaseSibling.Open()
				co.JoinAll(replacements)
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
