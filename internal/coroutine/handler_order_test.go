/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"slices"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestHandlerRestartFrameOrder(t *testing.T) {
	for _, test := range []struct {
		name      string
		policy    HandlerPolicy
		completed bool
		canceled  bool
		restarts  int
		want      []string
	}{
		{"active restart", RestartExisting, false, false, 1, []string{"replacement", "sibling"}},
		{"consecutive restarts", RestartExisting, false, false, 2, []string{"replacement", "sibling"}},
		{"canceled restart", RestartExisting, false, true, 1, []string{"sibling", "replacement"}},
		{"completed restart", RestartExisting, true, false, 1, []string{"sibling", "replacement"}},
		{"ignored overlap", IgnoreWhileRunning, false, false, 1, []string{"first", "sibling"}},
		{"ignore after cancellation", IgnoreWhileRunning, false, true, 1, []string{"sibling", "replacement"}},
		{"ignore after completion", IgnoreWhileRunning, true, false, 1, []string{"sibling", "replacement"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			co.OnInited()
			itime.Start(nil)
			t.Cleanup(func() {
				if !co.StopAllAndWait(time.Second) {
					t.Error("coroutines did not stop")
				}
			})
			state := NewHandlerState(test.policy)
			var order []string
			startHandler := func(name string, completed bool) Thread {
				return co.createThread(co.captureAdmission(), Task{
					Owner: name,
					Setup: state.Start,
					Run: func(Thread) {
						if !completed {
							co.WaitNextFrame()
							order = append(order, name)
						}
					},
				})
			}
			first := startHandler("first", test.completed)
			co.JoinYieldedOrDone(first)
			if test.completed {
				co.Join(first)
			}
			sibling := co.Create("sibling", func(Thread) int {
				co.WaitNextFrame()
				order = append(order, "sibling")
				return 0
			})
			co.JoinYieldedOrDone(sibling)

			var replacement Thread
			for range test.restarts {
				if test.canceled {
					// Keep the old invocation's cleanup pending during registration.
					co.runMu.Lock()
					co.Stop(first)
					replacement = startHandler("replacement", false)
					co.runMu.Unlock()
				} else {
					replacement = startHandler("replacement", false)
				}
				co.JoinYieldedOrDone(replacement)
			}
			if replacement.ID() <= sibling.ID() || first.ID() >= sibling.ID() {
				t.Fatal("restarting a handler changed thread identity order")
			}

			co.Update()
			itime.Update(1.0/30, 30)
			co.Update()
			if !slices.Equal(order, test.want) {
				t.Fatalf("frame order = %v, want %v", order, test.want)
			}
		})
	}
}
