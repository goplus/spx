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
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"
)

func TestAbortCoroutinesForWebResetFromCoroutineDoesNotWaitForPeers(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		SetCoroutines(original)
	})

	type resetState struct {
		completed    bool
		abortCurrent bool
	}

	result := make(chan resetState, 1)
	peerYielding := make(chan struct{})
	peerDone := make(chan struct{})

	co.CreateAndStart(true, "peer", func(peer coroutine.Thread) int {
		defer close(peerDone)
		close(peerYielding)
		co.Yield(peer)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerYielding:
	case <-timer.C:
		t.Fatal("peer coroutine did not reach yield")
	}

	co.CreateAndStart(true, "caller", func(me coroutine.Thread) int {
		completed, abortCurrent := abortCoroutinesForWebReset(time.Hour)
		result <- resetState{completed: completed, abortCurrent: abortCurrent}
		return 0
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case got := <-result:
		if got.completed {
			t.Fatal("web reset should not wait for peers from a coroutine")
		}
		if !got.abortCurrent {
			t.Fatal("web reset should request current coroutine abort")
		}
	case <-timer.C:
		t.Fatal("web reset waited for a peer coroutine")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerDone:
	case <-timer.C:
		t.Fatal("peer coroutine did not exit after caller returned")
	}
}
