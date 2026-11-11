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
	"fmt"
	"testing"

	"github.com/goplus/spx/v4/internal/core/clock"
)

func TestWaitResumesAtLogicalDeadline(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	resumedFrame := int64(-1)
	th := scheduler.Create(nil, func(Thread) {
		scheduler.Wait(1)
		resumedFrame = clock.Frame()
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	clock.Update(1, 30)
	scheduler.Update()
	if resumedFrame != 1 {
		t.Fatalf("wait resumed at frame %d, want 1", resumedFrame)
	}
}

func TestWaitWholeSecondAtFixedFPSDoesNotDriftOneFrame(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	resumedFrame := int64(-1)
	th := scheduler.Create(nil, func(Thread) {
		scheduler.Wait(1)
		resumedFrame = clock.Frame()
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	for frame := 1; frame <= 31; frame++ {
		clock.Update(1.0/30, 30)
		scheduler.Update()
		if resumedFrame >= 0 {
			break
		}
	}
	if resumedFrame != 30 {
		t.Fatalf("wait resumed at frame %d, want 30", resumedFrame)
	}
}

func TestConsecutiveWholeSecondWaitsDoNotAccumulateFrameDrift(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	var resumedFrames []int64
	th := scheduler.Create(nil, func(Thread) {
		for range 3 {
			scheduler.Wait(1)
			resumedFrames = append(resumedFrames, clock.Frame())
		}
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	for range 90 {
		clock.Update(1.0/30, 30)
		scheduler.Update()
	}
	for i, want := range []int64{30, 60, 90} {
		if len(resumedFrames) <= i || resumedFrames[i] != want {
			t.Fatalf("resume frames = %v, want [30 60 90]", resumedFrames)
		}
	}
}

func TestLongFixedStepWaitDoesNotDrift(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	resumedFrame := int64(-1)
	th := scheduler.Create(nil, func(Thread) {
		scheduler.Wait(2400)
		resumedFrame = clock.Frame()
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	for range 71_999 {
		clock.Update(1.0/30, 30)
	}
	scheduler.Update()
	if resumedFrame >= 0 {
		t.Fatalf("wait resumed early at frame %d", resumedFrame)
	}

	clock.Update(1.0/30, 30)
	scheduler.Update()
	if resumedFrame != 72_000 {
		t.Fatalf("wait resumed at frame %d, want 72000", resumedFrame)
	}
}

func TestWaitDoesNotResumeBeforeDeadline(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	resumed := false
	th := scheduler.Create(nil, func(Thread) {
		scheduler.Wait(1)
		resumed = true
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()

	clock.Update(1-5e-10, 30)
	scheduler.Update()
	if resumed {
		t.Fatal("wait resumed before its deadline")
	}

	clock.Update(5e-10, 30)
	scheduler.Update()
	if !resumed {
		t.Fatal("wait did not resume at its deadline")
	}
}

func TestWaitAlwaysCrossesTheIssuingFrame(t *testing.T) {
	for _, duration := range []float64{-1, 0, 1e-12, 1e-9} {
		t.Run(fmt.Sprint(duration), func(t *testing.T) {
			scheduler := New(nil)
			clock.Start()
			resumed := false
			th := scheduler.Create(nil, func(Thread) {
				scheduler.Wait(duration)
				resumed = true
			})
			scheduler.JoinFirstSlice(th)
			scheduler.Update()
			if resumed {
				t.Fatal("wait resumed in its issuing frame")
			}

			clock.Update(0, 30)
			scheduler.Update()
			if want := duration <= 0; resumed != want {
				t.Fatalf("resumed after zero delta = %v, want %v", resumed, want)
			}
			if resumed {
				return
			}

			clock.Update(duration, 30)
			scheduler.Update()
			if !resumed {
				t.Fatal("positive wait did not resume at its deadline")
			}
		})
	}
}
