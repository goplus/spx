//go:build !js && !pure_engine
// +build !js,!pure_engine

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

package coroutine

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestTryRunFromEngineRejectedByShutdownBarrier(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	blocker := co.Create("blocker", func(Thread) {
		close(blockerStarted)
		<-releaseBlocker
	})
	waitForThreadSignal(t, blockerStarted, "blocking coroutine did not start")

	callbackRan := atomic.Bool{}
	dispatchDone := make(chan struct{})
	go func() {
		co.TryRunFromEngine("dispatcher", func() {
			callbackRan.Store(true)
		})
		close(dispatchDone)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		co.threadsMu.Lock()
		count := len(co.allThreads)
		co.threadsMu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("managed dispatcher was not registered")
		}
		time.Sleep(time.Millisecond)
	}

	stopResult := make(chan error, 1)
	go func() {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		stopResult <- co.RunAfterStop(timeoutCtx, nil)
	}()
	select {
	case err := <-stopResult:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked drain error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not time out")
	}

	close(releaseBlocker)
	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		t.Fatal("rejected managed dispatcher did not clean up")
	}
	if callbackRan.Load() {
		t.Fatal("managed dispatcher callback ran after shutdown admission closed")
	}
	select {
	case <-blocker.done:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish")
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("explicit recovery barrier did not complete")
	}
	if co.hasPendingWork() {
		t.Fatal("rejected managed dispatcher leaked from thread registry")
	}
}

func TestWaitMainThreadPreservesScriptSliceAcrossFrames(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	itime.Start(nil)
	t.Cleanup(func() {
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("script order test coroutines did not stop")
		}
	})

	var mu sync.Mutex
	var trace []string
	record := func(event string) {
		mu.Lock()
		trace = append(trace, event)
		mu.Unlock()
	}
	first := co.Create("A", func(Thread) {
		for range 3 {
			co.WaitNextFrame()
			record("A-before")
			co.WaitMainThread(func() {
				record("main")
				if co.runMu.TryLock() {
					co.runMu.Unlock()
					t.Error("main-thread call released the current script slice")
				}
				runtime.Gosched()
			})
			record("A-after")
		}
	})
	second := co.Create("B", func(Thread) {
		for range 3 {
			co.WaitNextFrame()
			record("B")
		}
	})
	joinFirstSliceAll(co, []Thread{first, second})
	co.Update()

	want := []string{"A-before", "main", "A-after", "B"}
	for frame := range 3 {
		itime.Update(1.0/30, 30)
		co.Update()
		mu.Lock()
		got := append([]string(nil), trace...)
		trace = nil
		mu.Unlock()
		if !slices.Equal(got, want) {
			t.Fatalf("frame %d script order = %v, want %v", frame+1, got, want)
		}
	}
	co.JoinAll([]Thread{first, second})
}

func TestRunBetweenScriptsServicesPendingMainThreadCall(t *testing.T) {
	co := New(nil)
	queued := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	value := 0
	go func() {
		co.runMu.Lock()
		defer co.runMu.Unlock()
		co.enqueuePriorityJob(&waitJob{kind: waitMainThread, call: unblock})
		close(queued)
		<-release
		value = 42
	}()
	<-queued
	result := make(chan int, 1)
	go co.RunBetweenScripts(func() { result <- value })
	select {
	case got := <-result:
		if got != 42 {
			t.Fatalf("script state = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("frame-boundary callback deadlocked behind an engine call")
	}
}

func TestUpdateServicesMainThreadDuringStartup(t *testing.T) {
	setMainThreadForTest(t, false)
	for _, outcome := range []string{"success", "failure", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			var completed atomic.Bool
			co := New(func(PanicReport) {})
			t.Cleanup(func() {
				if co.RunAfterStop(drainTestContext(t), nil) != nil {
					t.Error("startup coroutine did not stop")
				}
			})
			called := false
			co.Create("startup", func(me Thread) {
				co.WaitMainThread(func() {
					called = true
					if outcome == "canceled" {
						co.Stop(me)
					}
				})
				if outcome == "failure" {
					panic("startup failed")
				}
				completed.Store(true)
			})

			updated := make(chan struct{})
			go func() {
				co.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "startup's main-thread call prevented the frame from returning")
			if !called {
				t.Fatal("startup's main-thread call was not served")
			}
			if got, want := completed.Load(), outcome == "success"; got != want {
				t.Fatalf("completed = %v, want %v", got, want)
			}
		})
	}
}
