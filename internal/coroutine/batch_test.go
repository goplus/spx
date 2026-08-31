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
		order      []string
		registered int
		threads    []Thread
	)

	caller := co.Create("caller", func(Thread) int {
		threads = co.StartBatch([]BatchTask{
			{
				Owner: "first",
				Run: func(Thread) {
					registered = len(co.snapshotThreads())
					order = append(order, "first")
					var signal struct{}
					WaitForChan(co, releaseFirst, &signal)
				},
			},
			{
				Owner: "second",
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
