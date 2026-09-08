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
	"sync"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestLoopBudgetYieldsWithoutStoppingScript(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() { co.AbortAllAndWait(time.Second) })
	iterations := 0
	th := co.Create(nil, func(me Thread) int {
		for {
			iterations++
			co.YieldLoopFor(me)
		}
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	if iterations <= 1 || th.Stopped() {
		t.Fatalf("iterations = %d, stopped = %v", iterations, th.Stopped())
	}
	previous := iterations
	itime.Update(1.0/30, 30)
	co.Update()
	if iterations <= previous || th.Stopped() {
		t.Fatalf("script did not continue after budget: %d -> %d", previous, iterations)
	}
}

func TestReadScriptStateServicesPendingMainThreadCall(t *testing.T) {
	co := New(nil)
	co.OnInited()
	queued := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	value := 0
	go func() {
		co.runMu.Lock()
		defer co.runMu.Unlock()
		co.enqueuePriorityJob(&WaitJob{Type: waitTypeMainThread, Call: unblock})
		close(queued)
		<-release
		value = 42
	}()
	<-queued
	result := make(chan int, 1)
	go co.ReadScriptState(func() { result <- value })
	select {
	case got := <-result:
		if got != 42 {
			t.Fatalf("script state = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("frame-boundary read deadlocked behind an engine call")
	}
}
