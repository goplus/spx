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

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"
)

func TestExecuteNativeRunsInlineOutsideCoroutine(t *testing.T) {
	called := false

	ExecuteNative(func(ctx context.Context, owner any) {
		called = true
		if ctx == nil {
			t.Fatal("ExecuteNative provided nil context")
		}
		if owner != nil {
			t.Fatalf("owner = %v, want nil outside coroutine", owner)
		}
		if IsInCoroutine() {
			t.Fatal("native function should not run in an SPX coroutine outside coroutine context")
		}
	})

	if !called {
		t.Fatal("ExecuteNative did not run the function")
	}
}

func TestExecuteNativeFromCoroutineResumesWhenNativeWorkCompletes(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		SetCoroutines(original)
	})

	owner := &struct{ name string }{name: "owner"}
	nativeStarted := make(chan struct{})
	releaseNative := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseNative)
		})
	}

	type result struct {
		ctxNil              bool
		ctxCanceled         bool
		ownerMatches        bool
		nativeInCoroutine   bool
		returnedInCoroutine bool
	}
	done := make(chan result, 1)

	th := co.CreateAndStart(true, owner, func(me coroutine.Thread) int {
		var got result
		ExecuteNative(func(ctx context.Context, gotOwner any) {
			close(nativeStarted)
			<-releaseNative

			got.ctxNil = ctx == nil
			if ctx != nil {
				got.ctxCanceled = ctx.Err() != nil
			}
			got.ownerMatches = gotOwner == owner
			got.nativeInCoroutine = IsInCoroutine()
		})
		got.returnedInCoroutine = IsInCoroutine()
		done <- got
		return 0
	})
	t.Cleanup(func() {
		release()
		co.StopIf(func(candidate coroutine.Thread) bool {
			return candidate == th
		})
	})

	select {
	case <-nativeStarted:
	case <-time.After(time.Second):
		t.Fatal("native function did not start")
	}

	select {
	case <-done:
		t.Fatal("ExecuteNative returned before native work completed")
	default:
	}

	release()

	select {
	case got := <-done:
		if got.ctxNil {
			t.Fatal("ExecuteNative provided nil coroutine context")
		}
		if got.ctxCanceled {
			t.Fatal("coroutine context should not be canceled while native work runs")
		}
		if !got.ownerMatches {
			t.Fatal("ExecuteNative did not pass the current coroutine owner")
		}
		if got.nativeInCoroutine {
			t.Fatal("native function should run outside the SPX coroutine")
		}
		if !got.returnedInCoroutine {
			t.Fatal("ExecuteNative should resume the caller coroutine")
		}
	case <-time.After(time.Second):
		t.Fatal("ExecuteNative did not resume after native work completed")
	}
}
