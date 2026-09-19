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

package spx

import (
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestSchedNowWarnsInsteadOfPanickingOnMainExecutionTimeout(t *testing.T) {
	testMainExecutionTimeoutDemotion(t, SchedNow)
}

func TestSchedWarnsInsteadOfPanickingOnMainExecutionTimeout(t *testing.T) {
	testMainExecutionTimeoutDemotion(t, Sched)
}

func testMainExecutionTimeoutDemotion(t *testing.T, sched func() int) {
	t.Helper()
	co := setupRuntimeEventScheduler(t)
	thread := co.Create("main", func(thread coroutine.Thread) int {
		end := thread.BeginMain(time.Now().Add(-2 * time.Duration(mainExecTimeoutSec) * time.Second))
		defer end()
		sched()
		if !thread.MainStartedAt().IsZero() {
			t.Error("timed-out Main execution was not demoted")
		}
		return 0
	})
	co.Join(thread)
	if !thread.Stopped() && thread.Context().Err() == nil {
		t.Fatal("Main timeout test coroutine did not finish")
	}
}

func TestSchedNowExternalCallerDoesNotDriveActiveCoroutine(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	started := make(chan struct{})
	release := make(chan struct{})
	thread := co.Create(game, func(thread coroutine.Thread) int {
		end := thread.BeginMain(time.Now().Add(-2 * time.Duration(mainExecTimeoutSec) * time.Second))
		defer end()
		close(started)
		<-release
		return 0
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not start")
	}

	result := make(chan any, 1)
	go func() {
		defer func() { result <- recover() }()
		SchedNow()
	}()
	select {
	case recovered := <-result:
		if recovered != nil {
			t.Fatalf("external SchedNow panicked: %v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("external SchedNow did not return")
	}
	if thread.Stopped() {
		t.Fatal("external SchedNow stopped the active coroutine")
	}

	close(release)
	co.Join(thread)
}
