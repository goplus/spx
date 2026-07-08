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
	if err := EnqueueCapture("queued.png", CaptureIntentSnapshot); err != nil {
		t.Fatal(err)
	}
	ResetFrameRuntime()

	RunFrameCallbacks(base + 1)
	if ran {
		t.Fatal("ResetFrameRuntime did not clear pending callbacks")
	}

	SetCaptureHandler(func(req CaptureRequest) error {
		t.Fatalf("ResetFrameRuntime did not clear pending capture %q", req.Name)
		return nil
	})
	defer SetCaptureHandler(nil)
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRequestQueuesForActiveGame(t *testing.T) {
	var got CaptureRequest
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetFrameRuntime()
	defer ResetFrameRuntime()
	expectedFrame := CurrentFrame()
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := EnqueueCapture("step_001.png", CaptureIntentSnapshot); err != nil {
		t.Fatal(err)
	}
	if got.Name != "" {
		t.Fatalf("EnqueueCapture ran immediately: %+v", got)
	}

	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.Name != "step_001.png" {
		t.Fatalf("capture name = %q, want step_001.png", got.Name)
	}
	if got.Intent != CaptureIntentSnapshot {
		t.Fatalf("capture intent = %q, want %q", got.Intent, CaptureIntentSnapshot)
	}
	if got.Frame != expectedFrame {
		t.Fatalf("capture frame = %d, want %d", got.Frame, expectedFrame)
	}
	if got.Sequence == 0 {
		t.Fatal("capture sequence was not assigned")
	}
}
