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

func TestLatchResumesManagedWaitersOnce(t *testing.T) {
	co := New(nil)
	co.OnInited()
	latch := co.NewLatch()
	var order []string

	waiter := co.Create("waiter", func(Thread) int {
		order = append(order, "before")
		latch.Wait()
		order = append(order, "after")
		return 0
	})
	co.JoinYieldedOrDone(waiter)
	if want := []string{"before"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order before Open = %v, want %v", order, want)
	}

	latch.Open()
	latch.Open()
	co.Join(waiter)
	if want := []string{"before", "after"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order after Open = %v, want %v", order, want)
	}
}

func TestLatchAlreadyOpenReturns(t *testing.T) {
	co := New(nil)
	co.OnInited()
	latch := co.NewLatch()
	latch.Open()

	thread := co.Create("waiter", func(Thread) int {
		latch.Wait()
		return 0
	})
	co.Join(thread)
}

func TestLatchCanceledWaiterIsRemoved(t *testing.T) {
	co := New(nil)
	co.OnInited()
	latch := co.NewLatch()

	waiter := co.Create("waiter", func(Thread) int {
		latch.Wait()
		return 0
	})
	co.JoinYieldedOrDone(waiter)
	co.StopIf(func(thread Thread) bool { return thread == waiter })
	co.Join(waiter)

	latch.mu.Lock()
	defer latch.mu.Unlock()
	if len(latch.waiters) != 0 {
		t.Fatalf("canceled latch waiters = %d, want 0", len(latch.waiters))
	}
}

func TestLatchWaitsOutsideCoroutine(t *testing.T) {
	co := New(nil)
	latch := co.NewLatch()
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		latch.Wait()
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("Wait returned before Open")
	default:
	}

	latch.Open()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after Open")
	}
}
