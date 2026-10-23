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

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

func setupCaptureTest(t testing.TB) {
	t.Helper()
	clear := func() {
		t.Helper()
		engine.SetCaptureHandler(func(engine.CaptureRequest) error { return nil })
		if err := engine.FlushCaptures(); err != nil {
			t.Fatal(err)
		}
		engine.SetCaptureHandler(nil)
	}
	clear()
	t.Cleanup(clear)
}

func TestSnapshotUsesConfiguredHandlerAfterBody(t *testing.T) {
	var got []string
	engine.SetGame(nil)
	setupCaptureTest(t)
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, "capture:"+req.Name)
		return nil
	})
	defer engine.SetCaptureHandler(nil)

	Snapshot("step_001.png", func() error {
		got = append(got, "body")
		return nil
	})

	want := []string{"body", "capture:step_001.png"}
	if len(got) != len(want) {
		t.Fatalf("Snapshot order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Snapshot order = %v, want %v", got, want)
		}
	}
}

func TestSnapshotQueuesRequestForActiveGame(t *testing.T) {
	var got []string
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	setupCaptureTest(t)
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req.Name)
		return nil
	})
	defer engine.SetCaptureHandler(nil)

	Snapshot("step_001.png", nil)
	if len(got) != 0 {
		t.Fatalf("Snapshot ran immediately: %v", got)
	}

	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "step_001.png" {
		t.Fatalf("FlushCaptures = %v, want [step_001.png]", got)
	}
}

func TestSnapshotBodyMayWaitForNextFrame(t *testing.T) {
	for _, stopOwner := range []bool{false, true} {
		name := "resume"
		if stopOwner {
			name = "stop owner"
		}
		t.Run(name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			setupCaptureTest(t)
			base := itime.Frame()

			bodyStarted, bodyCompleted := false, false
			var captured []engine.CaptureRequest
			engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
				captured = append(captured, req)
				return nil
			})
			t.Cleanup(func() { engine.SetCaptureHandler(nil) })

			thread := co.Create(game, func(coroutine.Thread) {
				Snapshot("yielded.png", func() error {
					bodyStarted = true
					engine.WaitNextFrame()
					bodyCompleted = true
					return nil
				})
			})
			co.JoinFirstSlice(thread)
			if !bodyStarted || bodyCompleted {
				t.Fatal("capture body did not suspend at its next-frame wait")
			}
			if engine.HasPendingCaptures() {
				t.Fatal("capture was queued before its body completed")
			}

			if stopOwner {
				game.Stop(OtherScriptsInGame)
			}
			itime.Update(0, 0)
			co.Update()
			co.Join(thread)
			if stopOwner {
				if !thread.Stopped() || bodyCompleted || engine.HasPendingCaptures() {
					t.Fatal("stopped owner's capture body resumed or queued a screenshot")
				}
			} else if !bodyCompleted || !engine.HasPendingCaptures() {
				t.Fatal("capture was not queued after its body completed")
			}
			if err := engine.FlushCaptures(); err != nil {
				t.Fatal(err)
			}
			if stopOwner {
				if len(captured) != 0 {
					t.Fatalf("stopped owner's captured requests = %+v, want none", captured)
				}
			} else if len(captured) != 1 || captured[0].Name != "yielded.png" || captured[0].Frame != base+1 {
				t.Fatalf("captured requests = %+v, want yielded.png at frame %d", captured, base+1)
			}
		})
	}
}
