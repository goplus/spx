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

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestWaitResumesAtLogicalDeadline(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	resumedFrame := int64(-1)
	th := co.Create(nil, func(Thread) int {
		co.Wait(1)
		resumedFrame = itime.Frame()
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	itime.Update(1, 30)
	co.Update()
	if resumedFrame != 1 {
		t.Fatalf("wait resumed at frame %d, want 1", resumedFrame)
	}
}

func TestWaitWholeSecondAtFixedFPSDoesNotDriftOneFrame(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	resumedFrame := int64(-1)
	th := co.Create(nil, func(Thread) int {
		co.Wait(1)
		resumedFrame = itime.Frame()
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	for frame := 1; frame <= 31; frame++ {
		itime.Update(1.0/30, 30)
		co.Update()
		if resumedFrame >= 0 {
			break
		}
	}
	if resumedFrame != 30 {
		t.Fatalf("wait resumed at frame %d, want 30", resumedFrame)
	}
}

func TestConsecutiveWholeSecondWaitsDoNotAccumulateFrameDrift(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	var resumedFrames []int64
	th := co.Create(nil, func(Thread) int {
		for range 3 {
			co.Wait(1)
			resumedFrames = append(resumedFrames, itime.Frame())
		}
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	for range 90 {
		itime.Update(1.0/30, 30)
		co.Update()
	}
	for i, want := range []int64{30, 60, 90} {
		if len(resumedFrames) <= i || resumedFrames[i] != want {
			t.Fatalf("resume frames = %v, want [30 60 90]", resumedFrames)
		}
	}
}

func TestLongFixedStepWaitDoesNotDrift(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	resumedFrame := int64(-1)
	th := co.Create(nil, func(Thread) int {
		co.Wait(2400)
		resumedFrame = itime.Frame()
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	for range 71_999 {
		itime.Update(1.0/30, 30)
	}
	co.Update()
	if resumedFrame >= 0 {
		t.Fatalf("wait resumed early at frame %d", resumedFrame)
	}

	itime.Update(1.0/30, 30)
	co.Update()
	if resumedFrame != 72_000 {
		t.Fatalf("wait resumed at frame %d, want 72000", resumedFrame)
	}
}

func TestWaitDoesNotResumeBeforeDeadline(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	resumed := false
	th := co.Create(nil, func(Thread) int {
		co.Wait(1)
		resumed = true
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()

	itime.Update(1-5e-10, 30)
	co.Update()
	if resumed {
		t.Fatal("wait resumed before its deadline")
	}

	itime.Update(5e-10, 30)
	co.Update()
	if !resumed {
		t.Fatal("wait did not resume at its deadline")
	}
}

func TestWaitAlwaysCrossesTheIssuingFrame(t *testing.T) {
	for _, duration := range []float64{-1, 0, 1e-12, 1e-9} {
		t.Run(fmt.Sprint(duration), func(t *testing.T) {
			co := New(nil)
			co.OnInited()
			itime.Start(nil)
			resumed := false
			th := co.Create(nil, func(Thread) int {
				co.Wait(duration)
				resumed = true
				return 0
			})
			co.JoinYieldedOrDone(th)
			co.Update()
			if resumed {
				t.Fatal("wait resumed in its issuing frame")
			}

			itime.Update(0, 30)
			co.Update()
			if want := duration <= 0; resumed != want {
				t.Fatalf("resumed after zero delta = %v, want %v", resumed, want)
			}
			if resumed {
				return
			}

			itime.Update(duration, 30)
			co.Update()
			if !resumed {
				t.Fatal("positive wait did not resume at its deadline")
			}
		})
	}
}
