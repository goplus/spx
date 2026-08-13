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
	"runtime"
	"testing"
	"time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
	main bool
}

func (p mainThreadTestPlatform) IsMainThread() bool {
	return p.main
}

func setMainThreadForTest(t *testing.T, main bool) {
	t.Helper()
	previous := gdx.PlatformMgr
	gdx.PlatformMgr = mainThreadTestPlatform{main: main}
	t.Cleanup(func() { gdx.PlatformMgr = previous })
}

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)
	called := false

	co.WaitMainThread(func() {
		called = true
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
	}
}

func TestWaitMainThreadNestedFastPath(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)
	called := false

	co.WaitMainThread(func() {
		co.WaitMainThread(func() { called = true })
	})

	if !called {
		t.Fatal("nested WaitMainThread call did not execute")
	}
}

func TestWaitMainThreadQueuesFromWorker(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	returned := make(chan struct{})

	go func() {
		co.WaitMainThread(func() {})
		close(returned)
	}()

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}

	select {
	case <-returned:
		t.Fatal("WaitMainThread returned before the queued call ran")
	default:
	}

	co.Update()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("WaitMainThread did not return after Update")
	}
}
