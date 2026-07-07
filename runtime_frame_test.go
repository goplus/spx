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

	"github.com/goplus/spx/v2/internal/engine"
)

func TestFrameSchedulesCallbackForActiveGame(t *testing.T) {
	ran := false
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := engine.CurrentFrame()

	Frame(int(base+1), func() {
		ran = true
	})
	if ran {
		t.Fatal("Frame ran callback before target frame")
	}

	engine.RunFrameCallbacks(base + 1)

	if !ran {
		t.Fatal("Frame did not run callback at target frame")
	}
}

func TestCaptureUsesConfiguredHandlerAfterBody(t *testing.T) {
	var got []string
	engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	SetCaptureHandler(func(name string, check bool) error {
		got = append(got, "capture:"+name)
		if check {
			got = append(got, "check")
		}
		return nil
	})
	defer SetCaptureHandler(nil)

	Capture("step_001.png", func() error {
		got = append(got, "body")
		return nil
	})

	want := []string{"body", "capture:step_001.png"}
	if len(got) != len(want) {
		t.Fatalf("Capture order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Capture order = %v, want %v", got, want)
		}
	}
}

func TestCaptureAndCheckMarksCheckMode(t *testing.T) {
	var checkMode bool
	engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	SetCaptureHandler(func(name string, check bool) error {
		if name != "step_001.png" {
			t.Fatalf("capture name = %q, want step_001.png", name)
		}
		checkMode = check
		return nil
	})
	defer SetCaptureHandler(nil)

	CaptureAndCheck("step_001.png", nil)

	if !checkMode {
		t.Fatal("CaptureAndCheck did not request check mode")
	}
}

func TestCaptureQueuesRequestForActiveGame(t *testing.T) {
	var got []string
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	SetCaptureHandler(func(name string, check bool) error {
		got = append(got, name)
		return nil
	})
	defer SetCaptureHandler(nil)

	Capture("step_001.png", nil)
	if len(got) != 0 {
		t.Fatalf("Capture ran immediately: %v", got)
	}

	if err := engine.RunCaptureRequests(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "step_001.png" {
		t.Fatalf("runCaptureRequests = %v, want [step_001.png]", got)
	}
}
