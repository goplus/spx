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
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartBatchRejectsInvalidMode(t *testing.T) {
	for _, mode := range []BatchMode{0, 255} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("StartBatch accepted mode %d", mode)
				}
			}()
			New(nil).StartBatch(nil, mode)
		}()
	}

	co := New(nil)
	for _, mode := range []BatchMode{BatchAsync, BatchWaitFirstSlice, BatchWaitDone} {
		if threads := co.StartBatch(nil, mode); threads != nil {
			t.Errorf("StartBatch(nil, %d) = %v, want nil", mode, threads)
		}
	}
	if co.LastThreadID() != 0 {
		t.Fatalf("empty batches created %d threads", co.LastThreadID())
	}
}

func TestStartBatchWaitsForOrderedFirstSlices(t *testing.T) {
	co := New(nil)
	co.OnInited()
	releaseFirst := make(chan struct{})
	callerDone := make(chan struct{})
	var (
		order            []string
		registered       int
		createdBeforeRun int
		created          int
		threads          []Thread
	)
	onRegistered := func(Thread) func() { created++; return nil }

	caller := co.Create("caller", func(Thread) int {
		threads = co.StartBatch([]BatchTask{
			{
				Owner:        "first",
				OnRegistered: onRegistered,
				Run: func(Thread) {
					registered = len(co.snapshotThreads())
					createdBeforeRun = created
					order = append(order, "first")
					var signal struct{}
					WaitForChan(co, releaseFirst, &signal)
				},
			},
			{
				Owner:        "second",
				OnRegistered: onRegistered,
				Run: func(Thread) {
					order = append(order, "second")
				},
			},
		}, BatchWaitFirstSlice)
		order = append(order, "caller")
		close(callerDone)
		return 0
	})

	select {
	case <-callerDone:
	case <-time.After(time.Second):
		t.Fatal("StartBatch did not finish every first slice")
	}
	if want := []string{"first", "second", "caller"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("batch order = %v, want %v", order, want)
	}
	if registered != 3 {
		t.Fatalf("threads registered before first Run = %d, want 3", registered)
	}
	if createdBeforeRun != 2 {
		t.Fatalf("OnRegistered calls before first Run = %d, want 2", createdBeforeRun)
	}

	close(releaseFirst)
	co.JoinAll(threads)
	co.Join(caller)
}

func TestStartBatchWaitFirstSliceSurvivesFinalCancellation(t *testing.T) {
	co := New(nil)
	co.OnInited()
	callerDone := make(chan struct{})
	var (
		order   []string
		threads []Thread
	)

	caller := co.Create("caller", func(Thread) int {
		threads = co.StartBatch([]BatchTask{
			{
				Owner: "first",
				Run: func(Thread) {
					order = append(order, "first")
					co.StopIf(func(thread Thread) bool { return thread.Obj == "last" })
				},
			},
			{
				Owner: "last",
				Run: func(Thread) {
					order = append(order, "last")
				},
			},
		}, BatchWaitFirstSlice)
		order = append(order, "caller")
		close(callerDone)
		return 0
	})

	select {
	case <-callerDone:
	case <-time.After(time.Second):
		t.Fatal("final cancellation blocked StartBatch")
	}
	if want := []string{"first", "caller"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("batch order after cancellation = %v, want %v", order, want)
	}

	co.JoinAll(threads)
	co.Join(caller)
}

func TestStartBatchCancellationBeforeWrapperPassesBaton(t *testing.T) {
	co := New(nil)
	co.OnInited()
	callerDone := make(chan struct{})
	var (
		order   []string
		threads []Thread
	)

	caller := co.Create("caller", func(Thread) int {
		threads = co.StartBatch([]BatchTask{
			{Owner: "first", Run: func(Thread) { order = append(order, "first") }},
			{Owner: "middle", Run: func(Thread) { order = append(order, "middle") }},
			{Owner: "last", Run: func(Thread) { order = append(order, "last") }},
		}, BatchAsync)
		co.StopIf(func(thread Thread) bool { return thread == threads[1] })
		close(callerDone)
		return 0
	})

	select {
	case <-callerDone:
	case <-time.After(time.Second):
		t.Fatal("async caller did not return")
	}
	batchDone := make(chan struct{})
	go func() {
		co.JoinAll(threads)
		close(batchDone)
	}()
	select {
	case <-batchDone:
	case <-time.After(time.Second):
		t.Fatal("canceled task did not pass the batch baton")
	}
	if want := []string{"first", "last"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("batch order after early cancellation = %v, want %v", order, want)
	}
	co.Join(caller)
}

func TestStartBatchAdmissionCoversRegistration(t *testing.T) {
	co := New(nil)
	co.OnInited()
	registered := make(chan struct{})
	releaseRegistration := make(chan struct{})
	batchDone := make(chan struct{})
	go func() {
		co.StartBatch([]BatchTask{{
			Owner: "task",
			OnRegistered: func(Thread) func() {
				close(registered)
				<-releaseRegistration
				return nil
			},
			Run: func(Thread) {},
		}}, BatchAsync)
		close(batchDone)
	}()

	select {
	case <-registered:
	case <-time.After(time.Second):
		t.Fatal("batch task was not registered")
	}
	abortDone := make(chan struct{})
	go func() {
		co.AbortAll()
		close(abortDone)
	}()
	select {
	case <-abortDone:
		t.Fatal("AbortAll bypassed an in-flight task registration")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseRegistration)
	select {
	case <-batchDone:
	case <-time.After(time.Second):
		t.Fatal("batch registration did not finish")
	}
	select {
	case <-abortDone:
	case <-time.After(time.Second):
		t.Fatal("AbortAll did not finish after registration")
	}
}

func TestStartBatchFinalizesRegisteredTaskBeforeFirstRun(t *testing.T) {
	co := New(nil)
	co.OnInited()
	registered := make(chan struct{})
	batchReady := make(chan Thread)
	releaseCaller := make(chan struct{})
	var cleaned, ran atomic.Bool

	caller := co.Create("caller", func(Thread) int {
		threads := co.StartBatch([]BatchTask{{
			OnRegistered: func(Thread) func() {
				close(registered)
				return func() { cleaned.Store(true) }
			},
			Run: func(Thread) { ran.Store(true) },
		}}, BatchAsync)
		batchReady <- threads[0]
		<-releaseCaller
		return 0
	})

	select {
	case <-registered:
	case <-time.After(time.Second):
		t.Fatal("batch task was not registered")
	}
	var child Thread
	select {
	case child = <-batchReady:
	case <-time.After(time.Second):
		t.Fatal("batch did not return its task")
	}
	co.AbortAll()
	close(releaseCaller)
	co.Join(child)
	co.Join(caller)

	if ran.Load() {
		t.Fatal("canceled task ran after registration")
	}
	if !cleaned.Load() {
		t.Fatal("registered task cleanup did not run after cancellation")
	}
}

func TestStartBatchCleanupPanicStillFinishesThread(t *testing.T) {
	reported := make(chan any, 1)
	co := New(func(report PanicReport) { reported <- report.Value })
	co.OnInited()
	thread := co.StartBatch([]BatchTask{{
		OnRegistered: func(Thread) func() {
			return func() { panic("cleanup failure") }
		},
		Run: func(Thread) {},
	}}, BatchAsync)[0]

	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("cleanup panic left the task running")
	}
	select {
	case got := <-reported:
		if got != "cleanup failure" {
			t.Fatalf("cleanup panic report = %v, want cleanup failure", got)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup panic was not reported")
	}

	var ran atomic.Bool
	next := co.CreateAndStart(true, "after-cleanup-panic", func(Thread) int {
		ran.Store(true)
		return 0
	})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("runMu remained locked after cleanup panic")
	}
	if !ran.Load() {
		t.Fatal("next coroutine did not run after cleanup panic")
	}
}
