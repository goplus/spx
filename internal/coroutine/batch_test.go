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
	"sync"
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
				co.RunAfterStop(func() {
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
				if co.admissionClosed() {
					t.Error("shutdown did not reopen admission")
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
	co.RunAfterStop(nil)
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

func TestStartBatchSkipsCanceledPendingTask(t *testing.T) {
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
		t.Fatal("canceled task blocked the remaining batch")
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
	stopDone := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(stopDone)
	}()
	deadline := time.Now().Add(time.Second)
	for !thread.Stopped() {
		select {
		case <-stopDone:
			t.Fatal("shutdown returned before stopping the registered task")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("shutdown did not stop the registered task")
		}
		runtime.Gosched()
	}
	select {
	case <-stopDone:
		t.Fatal("shutdown returned before registration finished")
	default:
	}

	close(releaseRegistration)
	select {
	case <-batchDone:
	case <-time.After(time.Second):
		t.Fatal("batch registration did not finish")
	}
	select {
	case <-stopDone:
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
	co.drain()
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
	co.drain()
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

func TestBatchWaitDoneDrainsNestedAsyncSlicesWithoutUpdate(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	entries := make(chan string, 6)
	leafRan := make(chan struct{})

	caller := co.Create("caller", func(Thread) {
		co.StartBatch([]Task{{Run: func(Thread) {
			entries <- "outer-before"
			co.StartBatch([]Task{{Run: func(Thread) {
				entries <- "inner-before"
				co.StartBatch([]Task{{Run: func(Thread) {
					entries <- "leaf"
					close(leafRan)
				}}}, BatchAsync)
				entries <- "inner-after"
			}}}, BatchAsync)
			entries <- "outer-after"
		}}}, BatchWaitDone)
		entries <- "caller"
	})

	// These tasks need no engine update or clock advancement to finish.
	waitForThreadSignal(t, caller.done, "batch caller did not resume without Update")
	waitForThreadSignal(t, leafRan, "nested async leaf did not run without Update")
	var got []string
	for len(entries) != 0 {
		got = append(got, <-entries)
	}
	want := []string{"outer-before", "outer-after", "inner-before", "inner-after", "leaf", "caller"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nested batch order = %v, want %v", got, want)
	}
}

func TestBatchWaitDoneReturnsAfterAsyncChildYields(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	release := co.NewLatch()
	childDone := make(chan struct{})
	var childStarted, childFinished atomic.Bool
	observed := make(chan [2]bool, 1)

	caller := co.Create("caller", func(Thread) {
		co.StartBatch([]Task{{Run: func(Thread) {
			co.StartBatch([]Task{{Run: func(Thread) {
				childStarted.Store(true)
				release.Wait()
				childFinished.Store(true)
				close(childDone)
			}}}, BatchAsync)
		}}}, BatchWaitDone)
		observed <- [2]bool{childStarted.Load(), childFinished.Load()}
	})

	waitForThreadSignal(t, caller.done, "batch waited for an async child's completion")
	if got := <-observed; got != [2]bool{true, false} {
		t.Fatalf("child started/finished when caller returned = %v, want [true false]", got)
	}
	release.Open()
	waitForThreadSignal(t, childDone, "async child did not finish after release")
}

func TestBatchWaitDoneInsideRegistrationWithoutUpdate(t *testing.T) {
	for _, test := range []struct {
		name     string
		depth    int
		external bool
	}{
		{"managed direct wait", 0, false},
		{"managed nested waits", 3, false},
		{"external direct wait", 0, true},
		{"external nested waits", 3, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			t.Cleanup(func() { co.RunAfterStop(nil) })
			var order []string
			var runNested func(int)
			runNested = func(depth int) {
				if depth == 0 {
					order = append(order, "independent")
					return
				}
				co.StartBatch([]Task{{Run: func(Thread) {
					runNested(depth - 1)
				}}}, BatchWaitDone)
			}

			run := func() {
				co.StartBatch([]Task{
					{Run: func(Thread) { order = append(order, "first") }},
					{
						RegisterInvocation: func(Thread) func() {
							// Earlier batch entries stay unpublished while this
							// hook yields to independently registered tasks.
							co.StartBatch([]Task{{Run: func(Thread) {
								runNested(test.depth)
							}}}, BatchWaitDone)
							order = append(order, "registration-finished")
							return nil
						},
						Run: func(Thread) { order = append(order, "second") },
					},
				}, BatchWaitDone)
			}
			var callerDone <-chan struct{}
			if test.external {
				done := make(chan struct{})
				callerDone = done
				go func() {
					defer close(done)
					run()
				}()
			} else {
				callerDone = co.Create("caller", func(Thread) { run() }).done
			}

			waitForThreadSignal(t, callerDone, "registering caller did not finish")
			if want := []string{"independent", "registration-finished", "first", "second"}; !reflect.DeepEqual(order, want) {
				t.Fatalf("registration order = %v, want %v", order, want)
			}
		})
	}
}

func TestRegistrationCanJoinIndependentYield(t *testing.T) {
	for _, test := range []struct {
		name    string
		cancel  bool
		sibling bool
		managed bool
	}{
		{"external live", false, false, false},
		{"external canceled", true, false, false},
		{"external live with unpublished sibling", false, true, false},
		{"external canceled with unpublished sibling", true, true, false},
		{"managed live", false, false, true},
		{"managed canceled", true, false, true},
		{"managed live with unpublished sibling", false, true, true},
		{"managed canceled with unpublished sibling", true, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			independentReady := make(chan Thread, 1)
			registrationDone := make(chan struct{})
			var threads []Thread
			var independentRan, taskRan, siblingRan atomic.Bool
			var cleaned atomic.Int32
			register := func() {
				defer close(registrationDone)
				var tasks []Task
				if test.sibling {
					tasks = append(tasks, Task{Run: func(Thread) { siblingRan.Store(true) }})
				}
				tasks = append(tasks, Task{
					RegisterInvocation: func(outer Thread) func() {
						if test.cancel {
							outer.Cancel()
						}
						independent := co.Create("independent", func(me Thread) {
							co.WaitYield(me)
							independentRan.Store(true)
						})
						independentReady <- independent
						co.Join(independent)
						return func() { cleaned.Add(1) }
					},
					Run: func(Thread) { taskRan.Store(true) },
				})
				threads = co.StartBatch(tasks, BatchAsync)
			}
			if test.managed {
				co.Create("registering caller", func(Thread) { register() })
			} else {
				go register()
			}

			var independent Thread
			select {
			case independent = <-independentReady:
			case <-time.After(time.Second):
				t.Fatal("registration did not create its independent task")
			}
			t.Cleanup(func() {
				// Break a regression's dependency cycle before draining.
				independent.Cancel()
				waitForThreadSignal(t, registrationDone, "registration did not finish after cancellation")
				co.RunAfterStop(nil)
			})
			waitForThreadSignal(t, independent.firstSliceDone, "independent task did not yield")
			if siblingRan.Load() {
				t.Fatal("unpublished sibling ran before registration completed")
			}
			updated := make(chan struct{})
			go func() {
				co.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "Update blocked on registration waiting for an independent yield")
			waitForThreadSignal(t, registrationDone, "registration did not finish after the independent task")
			for _, thread := range threads {
				waitForThreadSignal(t, thread.done, "registered task did not finish")
			}
			if !independentRan.Load() || taskRan.Load() == test.cancel || siblingRan.Load() != test.sibling {
				t.Fatalf("completed scripts: independent=%v task=%v sibling=%v", independentRan.Load(), taskRan.Load(), siblingRan.Load())
			}
			if got := cleaned.Load(); got != 1 {
				t.Fatalf("registration cleanup calls = %d, want 1", got)
			}
		})
	}
}

func TestBatchCancellationBeforePublicationWaitsForCleanup(t *testing.T) {
	// Keep registration on one P so Update can reach the canceled task before
	// its goroutine has acquired execution and published its cleanup work.
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	var canceled Thread
	var cleaned atomic.Int32
	co.StartBatch([]Task{
		{
			RegisterInvocation: func(thread Thread) func() {
				canceled = thread
				return func() { cleaned.Add(1) }
			},
			Run: func(Thread) { t.Error("canceled task ran") },
		},
		{
			RegisterInvocation: func(Thread) func() {
				canceled.Cancel()
				return nil
			},
			Run: func(Thread) {},
		},
	}, BatchAsync)
	co.Update()
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("cleanup calls after Update = %d, want 1", got)
	}
}

func TestUnpublishedCancellationTracksCleanup(t *testing.T) {
	for _, duringSetup := range []bool{false, true} {
		name := "after setup"
		if duringSetup {
			name = "during setup"
		}
		t.Run(name, func(t *testing.T) {
			co := New(nil)
			cleanupStarted := make(chan struct{})
			releaseCleanup := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseCleanup) }) }
			t.Cleanup(func() {
				release()
				co.RunAfterStop(nil)
			})
			thread := co.registerTask(co.captureAdmission(), Task{
				RegisterInvocation: func(thread Thread) func() {
					if duringSetup {
						thread.Cancel()
					}
					return func() {
						close(cleanupStarted)
						<-releaseCleanup
					}
				},
				Run: func(Thread) { t.Error("canceled unpublished task ran") },
			})
			thread.Cancel()
			waitForThreadSignal(t, cleanupStarted, "unpublished task did not start cancellation cleanup")
			co.schedulerMu.Lock()
			pending := co.hasPendingExecutionLocked()
			co.schedulerMu.Unlock()
			if !pending {
				t.Fatal("Update cannot observe executable cancellation cleanup")
			}
			release()
			updated := make(chan struct{})
			go func() {
				co.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "Update did not finish after cancellation cleanup")
			waitForThreadSignal(t, thread.done, "canceled unpublished task did not finish")

			// A batch may publish after a canceled member has already exited.
			co.publishThreads([]Thread{thread})
			thread.Cancel()
			co.schedulerMu.Lock()
			pending = co.hasPendingExecutionLocked()
			co.schedulerMu.Unlock()
			if pending {
				t.Fatal("publication resurrected finished cancellation cleanup")
			}
		})
	}
}

func TestBatchCompletionBoundaryCanceledCaller(t *testing.T) {
	co := New(nil)
	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBlocker) }) }
	t.Cleanup(func() {
		release()
		co.RunAfterStop(nil)
	})
	var continued atomic.Bool

	caller := co.Create("caller", func(Thread) {
		co.StartBatch([]Task{{Run: func(Thread) {
			co.Create("blocker", func(Thread) {
				close(blockerStarted)
				// The receiver has returned, but this queued slice must
				// finish before the waiting caller gets its next turn.
				<-releaseBlocker
			})
		}}}, BatchWaitDone)
		continued.Store(true)
	})
	waitForThreadSignal(t, caller.firstSliceDone, "caller did not wait at the completion boundary")
	waitForThreadSignal(t, blockerStarted, "pending runnable task did not start")
	co.Stop(caller)
	release()
	waitForThreadSignal(t, caller.done, "canceled boundary caller did not finish")
	if continued.Load() {
		t.Fatal("canceled caller ran its continuation")
	}

	// Canceling one boundary caller must not strand subsequent batches.
	next := co.Create("next", func(Thread) {
		co.StartBatch([]Task{{Run: func(Thread) {}}}, BatchWaitDone)
	})
	waitForThreadSignal(t, next.done, "later batch did not complete after caller cancellation")
}

func TestBatchWaitDoneWaitsForCleanup(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel bool
	}{
		{"normal completion", false},
		{"external cancellation", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			targetReady := make(chan Thread, 1)
			targetStarted := make(chan struct{})
			cleanupEntered := make(chan struct{})
			releaseCleanup := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseCleanup) }) }
			t.Cleanup(func() {
				release()
				co.RunAfterStop(nil)
			})
			blocked := co.NewLatch()
			var continued atomic.Bool

			caller := co.Create("caller", func(Thread) {
				co.StartBatch([]Task{{
					RegisterInvocation: func(target Thread) func() {
						targetReady <- target
						return func() {
							close(cleanupEntered)
							<-releaseCleanup
						}
					},
					Run: func(Thread) {
						close(targetStarted)
						if test.cancel {
							blocked.Wait()
						}
					},
				}}, BatchWaitDone)
				continued.Store(true)
			})
			waitForThreadSignal(t, targetStarted, "batch receiver did not start")
			target := <-targetReady
			if test.cancel {
				co.Stop(target)
			}
			waitForThreadSignal(t, cleanupEntered, "receiver cleanup did not start")
			if continued.Load() {
				t.Fatal("batch caller continued before receiver cleanup finished")
			}
			release()
			waitForThreadSignal(t, caller.done, "batch caller did not resume after cleanup")
			if !continued.Load() {
				t.Fatal("batch caller lost its continuation")
			}
		})
	}
}
