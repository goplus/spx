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
	"runtime"
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

func TestStartBatchRejectedByShutdown(t *testing.T) {
	for _, test := range []struct {
		name string
		mode BatchMode
	}{
		{"async", BatchAsync},
		{"done", BatchWaitDone},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkDrainReentry(t, func() {
				co := New(nil)
				task := Task{
					RegisterInvocation: func(Thread) func() {
						t.Error("rejected batch ran its registration callback")
						return nil
					},
					Run: func(Thread) { t.Error("rejected batch ran a script") },
				}
				err := co.RunAfterStop(drainTestContext(t), func() {
					threads := co.StartBatch([]Task{task, task}, test.mode)
					if len(threads) != 2 {
						t.Errorf("rejected batch returned %d threads, want 2", len(threads))
					}
					for _, thread := range threads {
						if !thread.Stopped() || thread.Context().Err() == nil {
							t.Error("rejected batch returned an uncanceled thread")
						}
						for _, done := range []<-chan struct{}{thread.firstSliceDone, thread.done} {
							select {
							case <-done:
							default:
								t.Error("rejected batch returned an unfinished thread")
							}
						}
					}
					if co.hasPendingWork() || !co.admissionClosed() {
						t.Error("rejected batch registered work or reopened admission")
					}
				})
				if err != nil || co.admissionClosed() {
					t.Errorf("shutdown did not finish and reopen admission: %v", err)
				}
			})
		})
	}
}

func TestStartBatchWaitFirstSliceRequiresManagedCaller(t *testing.T) {
	co := New(nil)
	var registered, ran atomic.Int32
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		co.StartBatch([]Task{{
			RegisterInvocation: func(Thread) func() {
				registered.Add(1)
				return nil
			},
			Run: func(Thread) { ran.Add(1) },
		}}, BatchWaitFirstSlice)
	}()
	if want := "coroutine: FirstSlice requires a managed caller"; panicValue != want {
		t.Errorf("host FirstSlice panic = %v, want %q", panicValue, want)
	}
	if got := co.LastThreadID(); got != 0 {
		t.Errorf("host FirstSlice allocated %d threads before rejection", got)
	}
	if err := co.RunAfterStop(drainTestContext(t), nil); err != nil {
		t.Fatal(err)
	}
	if got := registered.Load(); got != 0 {
		t.Errorf("host FirstSlice registered %d invocations before rejection", got)
	}
	if got := ran.Load(); got != 0 {
		t.Errorf("host FirstSlice ran %d tasks before rejection", got)
	}
}

func TestStartBatchWaitsForOrderedFirstSlices(t *testing.T) {
	co := New(nil)
	releaseFirst := make(chan struct{})
	callerDone := make(chan struct{})
	var (
		order            []string
		registered       int
		createdBeforeRun int
		created          int
		threads          []Thread
	)
	setup := func(Thread) func() { created++; return nil }

	caller := co.Create("caller", func(Thread) {
		threads = co.StartBatch([]Task{
			{
				Owner:              "first",
				RegisterInvocation: setup,
				Run: func(Thread) {
					registered = len(co.snapshotThreads())
					createdBeforeRun = created
					order = append(order, "first")
					WaitForChan(co, releaseFirst)
				},
			},
			{
				Owner:              "second",
				RegisterInvocation: setup,
				Run: func(Thread) {
					order = append(order, "second")
				},
			},
		}, BatchWaitFirstSlice)
		order = append(order, "caller")
		close(callerDone)
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
		t.Fatalf("RegisterInvocation calls before first Run = %d, want 2", createdBeforeRun)
	}

	close(releaseFirst)
	co.JoinAll(threads)
	co.Join(caller)
}

func TestStartBatchSnapshotsParentAdmission(t *testing.T) {
	co := New(nil)
	batchDone := make(chan []Thread, 1)
	var registered, ran atomic.Int32

	parent := co.Create("parent", func(parent Thread) {
		batchDone <- co.StartBatch([]Task{
			{
				RegisterInvocation: func(Thread) func() {
					registered.Add(1)
					co.Stop(parent)
					return nil
				},
				Run: func(Thread) { ran.Add(1) },
			},
			{
				RegisterInvocation: func(Thread) func() {
					registered.Add(1)
					return nil
				},
				Run: func(Thread) { ran.Add(1) },
			},
		}, BatchAsync)
	})

	var threads []Thread
	select {
	case threads = <-batchDone:
	case <-time.After(time.Second):
		t.Fatal("batch registration did not finish")
	}
	co.JoinAll(threads)
	co.Join(parent)
	if got := registered.Load(); got != 2 {
		t.Fatalf("lifecycle registrations = %d, want 2", got)
	}
	if got := ran.Load(); got != 2 {
		t.Fatalf("tasks ran %d times, want 2", got)
	}
}

func TestStartBatchWaitFirstSliceSurvivesFinalCancellation(t *testing.T) {
	co := New(nil)
	callerDone := make(chan struct{})
	var (
		order   []string
		threads []Thread
	)

	caller := co.Create("caller", func(Thread) {
		threads = co.StartBatch([]Task{
			{
				Owner: "first",
				Run: func(Thread) {
					order = append(order, "first")
					co.StopIf(func(thread Thread) bool { return thread.Owner == "last" })
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
	callerDone := make(chan struct{})
	var (
		order   []string
		threads []Thread
	)

	caller := co.Create("caller", func(Thread) {
		threads = co.StartBatch([]Task{
			{Owner: "first", Run: func(Thread) { order = append(order, "first") }},
			{Owner: "middle", Run: func(Thread) { order = append(order, "middle") }},
			{Owner: "last", Run: func(Thread) { order = append(order, "last") }},
		}, BatchAsync)
		co.StopIf(func(thread Thread) bool { return thread == threads[1] })
		close(callerDone)
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

func TestStartBatchShutdownWaitsForRegistration(t *testing.T) {
	co := New(nil)
	registered := make(chan Thread, 1)
	releaseRegistration := make(chan struct{})
	batchDone := make(chan struct{})
	go func() {
		co.StartBatch([]Task{{
			Owner: "task",
			RegisterInvocation: func(thread Thread) func() {
				registered <- thread
				<-releaseRegistration
				return nil
			},
			Run: func(Thread) {},
		}}, BatchAsync)
		close(batchDone)
	}()

	var thread Thread
	select {
	case thread = <-registered:
	case <-time.After(time.Second):
		t.Fatal("batch task was not registered")
	}
	stopDone := make(chan bool, 1)
	go func() {
		stopDone <- co.RunAfterStop(drainTestContext(t), nil) == nil
	}()
	deadline := time.Now().Add(time.Second)
	for !thread.Stopped() {
		select {
		case completed := <-stopDone:
			t.Fatalf("shutdown returned %v before stopping the registered task", completed)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("shutdown did not stop the registered task")
		}
		runtime.Gosched()
	}
	select {
	case completed := <-stopDone:
		t.Fatalf("shutdown returned %v before registration finished", completed)
	default:
	}

	close(releaseRegistration)
	select {
	case <-batchDone:
	case <-time.After(time.Second):
		t.Fatal("batch registration did not finish")
	}
	select {
	case completed := <-stopDone:
		if !completed {
			t.Fatal("shutdown timed out after registration")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after registration")
	}
}

func TestStartBatchRegistrationCanStopAll(t *testing.T) {
	co := New(nil)
	finished := make(chan []Thread, 1)
	var registered, cleaned, ran atomic.Int32

	go func() {
		finished <- co.StartBatch([]Task{
			{
				RegisterInvocation: func(Thread) func() {
					registered.Add(1)
					co.StopAll()
					return func() { cleaned.Add(1) }
				},
				Run: func(Thread) { ran.Add(1) },
			},
			{
				RegisterInvocation: func(Thread) func() {
					registered.Add(1)
					return func() { cleaned.Add(1) }
				},
				Run: func(Thread) { ran.Add(1) },
			},
		}, BatchAsync)
	}()

	var threads []Thread
	select {
	case threads = <-finished:
	case <-time.After(time.Second):
		t.Fatal("registration deadlocked while stopping scripts")
	}
	co.JoinAll(threads)
	for _, thread := range threads {
		if !thread.Stopped() {
			t.Fatal("StopAll did not reject the complete batch")
		}
	}
	if got := ran.Load(); got != 0 {
		t.Fatalf("task ran %d times after registration stopped it", got)
	}
	if got := registered.Load(); got != 1 {
		t.Fatalf("lifecycle registrations = %d, want 1", got)
	}
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestStopAllKeepsAdmissionClosedUntilRegistrationsFinish(t *testing.T) {
	co := New(nil)
	registered := make(chan Thread, 2)
	releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
	batchDone := make(chan Thread, 2)
	var cleaned, lateRan atomic.Int32

	for _, release := range releases {
		go func() {
			threads := co.StartBatch([]Task{{
				RegisterInvocation: func(thread Thread) func() {
					registered <- thread
					<-release
					return func() { cleaned.Add(1) }
				},
				Run: func(Thread) {},
			}}, BatchAsync)
			batchDone <- threads[0]
		}()
	}
	registeredThreads := make([]Thread, 0, len(releases))
	for range releases {
		select {
		case thread := <-registered:
			registeredThreads = append(registeredThreads, thread)
		case <-time.After(time.Second):
			t.Fatal("registration did not start")
		}
	}

	stopDone := make(chan struct{})
	go func() {
		co.StopAll()
		co.StopAll()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("StopAll waited for registration hooks")
	}
	for _, thread := range registeredThreads {
		if !thread.Stopped() {
			t.Fatal("StopAll did not stop a registered task")
		}
	}

	rejected := co.Create("during-registration", func(Thread) {
		lateRan.Add(1)
	})
	if !rejected.Stopped() {
		t.Fatal("admission reopened before registration finished")
	}

	close(releases[0])
	var threads []Thread
	select {
	case thread := <-batchDone:
		threads = append(threads, thread)
	case <-time.After(time.Second):
		t.Fatal("registration did not finish")
	}
	stillRejected := co.Create("between-registrations", func(Thread) {
		lateRan.Add(1)
	})
	if !stillRejected.Stopped() {
		t.Fatal("admission reopened before every registration finished")
	}

	close(releases[1])
	select {
	case thread := <-batchDone:
		threads = append(threads, thread)
	case <-time.After(time.Second):
		t.Fatal("last registration did not finish")
	}
	co.JoinAll(append(threads, rejected, stillRejected))
	if got := cleaned.Load(); got != 2 {
		t.Fatalf("cleanup calls = %d, want 2", got)
	}
	if got := lateRan.Load(); got != 0 {
		t.Fatalf("rejected task ran %d times", got)
	}

	admitted := co.Create("after-registration", func(Thread) {
		lateRan.Add(1)
	})
	co.Join(admitted)
	if admitted.Stopped() {
		t.Fatal("admission stayed closed after registration finished")
	}
	if got := lateRan.Load(); got != 1 {
		t.Fatalf("admitted task ran %d times, want 1", got)
	}
}

func TestStartBatchRegistrationGoexitRestoresAdmission(t *testing.T) {
	co := New(nil)
	done := make(chan struct{})
	var cleaned, ran atomic.Int32

	go func() {
		defer close(done)
		co.StartBatch([]Task{
			{
				RegisterInvocation: func(Thread) func() {
					return func() { cleaned.Add(1) }
				},
				Run: func(Thread) { ran.Add(1) },
			},
			{
				RegisterInvocation: func(Thread) func() {
					co.StopAll()
					runtime.Goexit()
					return nil
				},
				Run: func(Thread) { ran.Add(1) },
			},
		}, BatchAsync)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("registration Goexit did not unwind the batch")
	}
	if co.drain(drainTestContext(t)) != nil {
		t.Fatal("registration Goexit leaked its batch")
	}
	if got := co.pendingSetups.Load(); got != 0 {
		t.Fatalf("pending registration hooks = %d, want 0", got)
	}
	if got := ran.Load(); got != 0 {
		t.Fatalf("tasks ran %d times after registration Goexit", got)
	}
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}

	next := co.Create("after-registration-Goexit", func(Thread) {
		ran.Add(1)
	})
	co.Join(next)
	if next.Stopped() || ran.Load() != 1 {
		t.Fatal("admission did not recover after registration Goexit")
	}
}

func TestStartBatchRegistrationPanicStopsBatch(t *testing.T) {
	co := New(nil)
	recovered := make(chan any, 1)
	var cleaned, ran atomic.Int32

	go func() {
		defer func() { recovered <- recover() }()
		co.StartBatch([]Task{
			{
				RegisterInvocation: func(Thread) func() {
					return func() { cleaned.Add(1) }
				},
				Run: func(Thread) { ran.Add(1) },
			},
			{
				RegisterInvocation: func(Thread) func() {
					co.StopAll()
					panic("registration failure")
				},
				Run: func(Thread) { ran.Add(1) },
			},
		}, BatchAsync)
	}()

	select {
	case got := <-recovered:
		if got != "registration failure" {
			t.Fatalf("registration panic = %v, want registration failure", got)
		}
	case <-time.After(time.Second):
		t.Fatal("registration panic did not return")
	}
	if co.drain(drainTestContext(t)) != nil {
		t.Fatal("registration panic leaked its batch")
	}
	if got := ran.Load(); got != 0 {
		t.Fatalf("tasks ran %d times after registration panic", got)
	}
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
	if got := co.pendingSetups.Load(); got != 0 {
		t.Fatalf("pending registration hooks = %d, want 0", got)
	}

	next := co.Create("after-registration-panic", func(Thread) {
		ran.Add(1)
	})
	co.Join(next)
	if next.Stopped() || ran.Load() != 1 {
		t.Fatal("admission did not recover after registration panic")
	}
}

func TestStartBatchFinalizesRegisteredTaskBeforeFirstRun(t *testing.T) {
	co := New(nil)
	registered := make(chan struct{})
	batchReady := make(chan Thread)
	releaseCaller := make(chan struct{})
	var cleaned, ran atomic.Int32

	caller := co.Create("caller", func(Thread) {
		threads := co.StartBatch([]Task{{
			RegisterInvocation: func(Thread) func() {
				close(registered)
				return func() { cleaned.Add(1) }
			},
			Run: func(Thread) { ran.Add(1) },
		}}, BatchAsync)
		batchReady <- threads[0]
		<-releaseCaller
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
	co.StopAll()
	close(releaseCaller)
	co.Join(child)
	co.Join(caller)

	if got := ran.Load(); got != 0 {
		t.Fatalf("canceled task ran %d times after registration", got)
	}
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestStartBatchCleanupPanicStillFinishesThread(t *testing.T) {
	reported := make(chan any, 1)
	co := New(func(report PanicReport) { reported <- report.Value })
	thread := co.StartBatch([]Task{{
		RegisterInvocation: func(Thread) func() {
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
	next := co.Create("after-cleanup-panic", func(Thread) {
		ran.Store(true)
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
