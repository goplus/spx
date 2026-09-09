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
	"sync/atomic"
	"testing"
	"time"
)

func TestTryRunManagedBetweenScriptsRejectedByAbortBarrier(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)
	co.OnInited()

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	blocker := co.Create("blocker", func(Thread) int {
		close(blockerStarted)
		<-releaseBlocker
		return 0
	})
	waitForThreadSignal(t, blockerStarted, "blocking coroutine did not start")

	callbackRan := atomic.Bool{}
	dispatchDone := make(chan struct{})
	go func() {
		co.TryRunManagedBetweenScripts("dispatcher", func() {
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

	abortResult := make(chan bool, 1)
	go func() { abortResult <- co.RunAfterAbortAll(20*time.Millisecond, nil) }()
	select {
	case completed := <-abortResult:
		if completed {
			t.Fatal("abort barrier completed while blocker was still running")
		}
	case <-time.After(time.Second):
		t.Fatal("abort barrier did not time out")
	}

	close(releaseBlocker)
	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		t.Fatal("rejected managed dispatcher did not clean up")
	}
	if callbackRan.Load() {
		t.Fatal("managed dispatcher callback ran after abort admission closed")
	}
	select {
	case <-blocker.done:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish")
	}
	if !co.RunAfterAbortAll(time.Second, nil) {
		t.Fatal("explicit recovery barrier did not complete")
	}
	if co.hasThreadsOtherThan(nil) {
		t.Fatal("rejected managed dispatcher leaked from thread registry")
	}
}
