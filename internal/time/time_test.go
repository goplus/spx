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

package time

import (
	"math"
	"slices"
	"sync"
	"testing"
	stdtime "time"
)

func resetStateForTest() {
	realTimeSinceLevelLoad = 0
	timeSinceLevelLoad = 0
	logicalTimeCompensation = 0
	deltaTime = 0
	realDeltaTime = 0
	fixedDeltaTimeBits.Store(0)
	timeScaleBits.Store(0)
	curFrame.Store(0)
	setTimeScaleCallback = nil
	startTimestamp = stdtime.Time{}
	lastTimestamp = stdtime.Time{}
	fps = 0
	timerBaseTime = 0
	registeredTimers = nil
	pendingTimers = nil
}

func TestStartInitializesTimeState(t *testing.T) {
	resetStateForTest()
	realTimeSinceLevelLoad = 9
	timeSinceLevelLoad = 8
	logicalTimeCompensation = 9
	deltaTime = 7
	realDeltaTime = 6
	timeScaleBits.Store(math.Float64bits(5))
	curFrame.Store(4)
	fps = 3
	timerBaseTime = 1
	registeredTimers = []int64{100, 200}
	pendingTimers = []int64{200}

	Start(nil)

	if got := TimeScale(); got != 1 {
		t.Fatalf("TimeScale() = %v, want 1", got)
	}
	if got := DeltaTime(); got != 0 {
		t.Fatalf("DeltaTime() = %v, want 0", got)
	}
	if got := UnscaledDeltaTime(); got != 0 {
		t.Fatalf("UnscaledDeltaTime() = %v, want 0", got)
	}
	if got := TimeSinceLevelLoad(); got != 0 {
		t.Fatalf("TimeSinceLevelLoad() = %v, want 0", got)
	}
	if logicalTimeCompensation != 0 {
		t.Fatalf("logical time compensation = %v, want 0", logicalTimeCompensation)
	}
	if got := UnscaledTimeSinceLevelLoad(); got != 0 {
		t.Fatalf("UnscaledTimeSinceLevelLoad() = %v, want 0", got)
	}
	if got := Frame(); got != 0 {
		t.Fatalf("Frame() = %v, want 0", got)
	}
	if got := FPS(); got != DefaultFPS {
		t.Fatalf("FPS() = %v, want %v", got, DefaultFPS)
	}
	if got := Timer(); got != 0 {
		t.Fatalf("Timer() = %v, want 0", got)
	}
	if startTimestamp.IsZero() {
		t.Fatal("expected non-zero start timestamp")
	}
	if !slices.Equal(pendingTimers, registeredTimers) {
		t.Fatalf("pending timers = %v, want %v", pendingTimers, registeredTimers)
	}
}

func TestUpdateCompensatesFixedStepRounding(t *testing.T) {
	resetStateForTest()
	Start(nil)
	for range 72_000 {
		Update(1.0/30, 30)
	}
	if got := TimeSinceLevelLoad(); got != 2400 {
		t.Fatalf("TimeSinceLevelLoad() = %.17g, want 2400", got)
	}
}

func TestUpdateRefreshesRealTimeState(t *testing.T) {
	resetStateForTest()
	now := stdtime.Now()
	startTimestamp = now.Add(-2500 * stdtime.Millisecond)
	lastTimestamp = now.Add(-1250 * stdtime.Millisecond)

	Update(1.5, 60)

	if diff := math.Abs(UnscaledTimeSinceLevelLoad() - 2.5); diff > 0.1 {
		t.Fatalf("UnscaledTimeSinceLevelLoad() = %v, want about 2.5", UnscaledTimeSinceLevelLoad())
	}
	if diff := math.Abs(UnscaledDeltaTime() - 1.25); diff > 0.1 {
		t.Fatalf("UnscaledDeltaTime() = %v, want about 1.25", UnscaledDeltaTime())
	}
	if got := Frame(); got != 1 {
		t.Fatalf("Frame() = %v, want 1", got)
	}
}

func TestOnReloadKeepsEngineFrame(t *testing.T) {
	resetStateForTest()
	Start(nil)
	Update(0.1, 60)
	before := Frame()

	OnReload()

	if got := Frame(); got != before {
		t.Fatalf("Frame() after OnReload = %d, want unchanged %d", got, before)
	}
}

func TestUpdateUsesProvidedLogicalDeltaForLogicalTime(t *testing.T) {
	resetStateForTest()
	SetFixedDeltaTime(1.0 / 30)
	defer SetFixedDeltaTime(0)

	Update(0.5, 60)

	if got := DeltaTime(); got != 0.5 {
		t.Fatalf("DeltaTime() = %v, want 0.5", got)
	}
	if got := TimeSinceLevelLoad(); got != 0.5 {
		t.Fatalf("TimeSinceLevelLoad() = %v, want 0.5", got)
	}
	if _, ok := FixedDeltaTime(); !ok {
		t.Fatal("FixedDeltaTime() disabled, want enabled")
	}
}

func TestSetFixedDeltaTimeDisablesNonPositiveValues(t *testing.T) {
	resetStateForTest()
	SetFixedDeltaTime(0.1)
	SetFixedDeltaTime(0)

	if got, ok := FixedDeltaTime(); ok || got != 0 {
		t.Fatalf("FixedDeltaTime() = (%v, %v), want (0, false)", got, ok)
	}
}

func TestEffectiveLogicalDeltaTimeUsesFixedValueWhenEnabled(t *testing.T) {
	resetStateForTest()
	SetFixedDeltaTime(0.1)
	defer SetFixedDeltaTime(0)

	if got := EffectiveLogicalDeltaTime(0.25); got != 0.1 {
		t.Fatalf("EffectiveLogicalDeltaTime() = %v, want 0.1", got)
	}
}

func TestEffectiveLogicalDeltaTimeFallsBackToRawValue(t *testing.T) {
	resetStateForTest()

	if got := EffectiveLogicalDeltaTime(0.25); got != 0.25 {
		t.Fatalf("EffectiveLogicalDeltaTime() = %v, want 0.25", got)
	}
}

func TestFixedDeltaTimeSupportsConcurrentSessionChanges(t *testing.T) {
	resetStateForTest()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				SetFixedDeltaTime(1.0 / 30)
				SetFixedDeltaTime(0)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				_, _ = FixedDeltaTime()
				_ = EffectiveLogicalDeltaTime(0.25)
			}
		}()
	}
	wg.Wait()
}

func TestTimeScaleSupportsConcurrentAccess(t *testing.T) {
	resetStateForTest()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				SetTimeScale(float64(i))
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				_ = TimeScale()
			}
		}()
	}
	wg.Wait()
}

func assertTimers(t *testing.T, want ...int64) {
	t.Helper()
	for _, timestamp := range want {
		if got, ok := NextTimer(); !ok || got != timestamp {
			t.Fatalf("NextTimer() = (%v, %v), want (%v, true)", got, ok, timestamp)
		}
	}
	if got, ok := NextTimer(); ok {
		t.Fatalf("unexpected timer: %v", got)
	}
}

func TestTimerTracksTimeRelativeToReset(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(0.5)

	Update(0.4, 60)
	if got := Timer(); got != 0.4 {
		t.Fatalf("Timer() = %v, want 0.4", got)
	}
	assertTimers(t)

	Update(0.1, 60)
	if got := Timer(); got != 0.5 {
		t.Fatalf("Timer() = %v, want 0.5", got)
	}
	assertTimers(t, 500)

	ResetTimer()
	if got := Timer(); got != 0 {
		t.Fatalf("Timer() after ResetTimer = %v, want 0", got)
	}
	assertTimers(t)

	Update(0.5, 60)
	if got := Timer(); got != 0.5 {
		t.Fatalf("Timer() after reset update = %v, want 0.5", got)
	}
	assertTimers(t, 500)
}

func TestOnReloadClearsRegisteredTimers(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(0.1)
	Update(0.2, 60)

	OnReload()

	if got := Timer(); got != 0 {
		t.Fatalf("Timer() = %v, want 0", got)
	}
	if len(registeredTimers) != 0 {
		t.Fatalf("registered timers = %v, want none", registeredTimers)
	}
	assertTimers(t)
}

func TestTimerRegistrationAfterDispatch(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(1)
	RegisterTimer(2)
	Update(1, 60)
	assertTimers(t, 1000)

	RegisterTimer(0.5)
	RegisterTimer(1.5)
	RegisterTimer(1)
	assertTimers(t, 500)

	Update(1, 60)
	assertTimers(t, 1500, 2000)

	ResetTimer()
	Update(2, 60)
	assertTimers(t, 500, 1000, 1500, 2000)
}

func TestTimerRegistrationAfterDrain(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(0.5)
	Update(1, 60)
	assertTimers(t, 500)

	for _, at := range []float64{0.75, 0.25, 0.5009, 0.25} {
		RegisterTimer(at)
	}
	assertTimers(t, 250, 750)
}
