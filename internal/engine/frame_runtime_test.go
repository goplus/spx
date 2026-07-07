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

import "testing"

func TestFrameCallbacksRunAtTargetFrameOnce(t *testing.T) {
	var got []string
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+3, func() {
		got = append(got, "frame3")
	})
	ScheduleFrame(base+2, func() {
		got = append(got, "frame2")
	})

	RunFrameCallbacks(base + 1)
	if len(got) != 0 {
		t.Fatalf("RunFrameCallbacks ran callbacks early: %v", got)
	}

	RunFrameCallbacks(base + 2)
	RunFrameCallbacks(base + 2)
	if len(got) != 1 || got[0] != "frame2" {
		t.Fatalf("RunFrameCallbacks = %v, want [frame2]", got)
	}

	RunFrameCallbacks(base + 3)
	if len(got) != 2 || got[1] != "frame3" {
		t.Fatalf("RunFrameCallbacks = %v, want [frame2 frame3]", got)
	}
}

func TestFrameCallbacksRunMissedFramesInFrameOrder(t *testing.T) {
	var got []string
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+5, func() {
		got = append(got, "frame5-a")
	})
	ScheduleFrame(base+3, func() {
		got = append(got, "frame3")
	})
	ScheduleFrame(base+5, func() {
		got = append(got, "frame5-b")
	})

	RunFrameCallbacks(base + 6)

	want := []string{"frame3", "frame5-a", "frame5-b"}
	if len(got) != len(want) {
		t.Fatalf("RunFrameCallbacks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RunFrameCallbacks = %v, want %v", got, want)
		}
	}
}

func TestResetFrameRuntimeClearsPendingWork(t *testing.T) {
	ran := false
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	base := CurrentFrame()

	ScheduleFrame(base+1, func() {
		ran = true
	})
	if err := RequestCapture("queued.png", false); err != nil {
		t.Fatal(err)
	}
	ResetFrameRuntime()

	RunFrameCallbacks(base + 1)
	if ran {
		t.Fatal("ResetFrameRuntime did not clear pending callbacks")
	}

	SetCaptureHandler(func(name string, check bool) error {
		t.Fatalf("ResetFrameRuntime did not clear pending capture %q", name)
		return nil
	})
	defer SetCaptureHandler(nil)
	if err := RunCaptureRequests(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRequestQueuesForActiveGame(t *testing.T) {
	var got []string
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	SetCaptureHandler(func(name string, check bool) error {
		got = append(got, name)
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := RequestCapture("step_001.png", false); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("RequestCapture ran immediately: %v", got)
	}

	if err := RunCaptureRequests(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "step_001.png" {
		t.Fatalf("RunCaptureRequests = %v, want [step_001.png]", got)
	}
}
