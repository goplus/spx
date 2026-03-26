package time

import (
	"testing"
	stdtime "time"
)

func resetStateForTest() {
	unscaledTimeSinceLevelLoad = 0
	timeSinceLevelLoad = 0
	deltaTime = 0
	unscaledDeltaTime = 0
	timeScale = 0
	curFrame = 0
	setTimeScaleCallback = nil
	startTimestamp = stdtime.Time{}
	lastTimestamp = stdtime.Time{}
	fps = 0
	curFrameRealTimeSinceStart = 0
	timerBaseTime = 0
	timestamps = nil
	nextTimerIndex = 0
}

func TestStartInitializesTimeState(t *testing.T) {
	resetStateForTest()
	unscaledTimeSinceLevelLoad = 9
	timeSinceLevelLoad = 8
	deltaTime = 7
	unscaledDeltaTime = 6
	timeScale = 5
	curFrame = 4
	fps = 3
	curFrameRealTimeSinceStart = 2
	timerBaseTime = 1
	timestamps = []int64{100, 200}
	nextTimerIndex = 1

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
	if got := UnscaledTimeSinceLevelLoad(); got != 0 {
		t.Fatalf("UnscaledTimeSinceLevelLoad() = %v, want 0", got)
	}
	if got := Frame(); got != 0 {
		t.Fatalf("Frame() = %v, want 0", got)
	}
	if got := FPS(); got != defaultFPS {
		t.Fatalf("FPS() = %v, want %v", got, defaultFPS)
	}
	if got := Timer(); got != 0 {
		t.Fatalf("Timer() = %v, want 0", got)
	}
	if startTimestamp.IsZero() {
		t.Fatal("expected non-zero start timestamp")
	}
	if curFrameRealTimeSinceStart != 0 {
		t.Fatalf("curFrameRealTimeSinceStart = %v, want 0", curFrameRealTimeSinceStart)
	}
	if nextTimerIndex != 0 {
		t.Fatalf("nextTimerIndex = %d, want 0", nextTimerIndex)
	}
}

func TestUpdateUsesProvidedRealDuration(t *testing.T) {
	resetStateForTest()

	applyFrameUpdate(2.5, 1.5, 1.5, 1.25, 60)

	if curFrameRealTimeSinceStart != 2.5 {
		t.Fatalf("curFrameRealTimeSinceStart = %v, want 2.5", curFrameRealTimeSinceStart)
	}
	if got := Frame(); got != 1 {
		t.Fatalf("Frame() = %v, want 1", got)
	}
}

func TestTimerTracksTimeRelativeToReset(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(0.5)

	applyFrameUpdate(0.4, 0.4, 0.4, 0.4, 60)
	if got := Timer(); got != 0.4 {
		t.Fatalf("Timer() = %v, want 0.4", got)
	}
	if _, ok := NextTimer(); ok {
		t.Fatal("did not expect timer to fire before reaching target")
	}

	applyFrameUpdate(0.5, 0.5, 0.1, 0.1, 60)
	if got := Timer(); got != 0.5 {
		t.Fatalf("Timer() = %v, want 0.5", got)
	}
	if got, ok := NextTimer(); !ok || got != 0.5 {
		t.Fatalf("NextTimer() = (%v, %v), want (0.5, true)", got, ok)
	}

	ResetTimer()
	if got := Timer(); got != 0 {
		t.Fatalf("Timer() after ResetTimer = %v, want 0", got)
	}
	if _, ok := NextTimer(); ok {
		t.Fatal("did not expect timer to fire immediately after reset")
	}

	applyFrameUpdate(1.0, 1.0, 0.5, 0.5, 60)
	if got := Timer(); got != 0.5 {
		t.Fatalf("Timer() after reset update = %v, want 0.5", got)
	}
	if got, ok := NextTimer(); !ok || got != 0.5 {
		t.Fatalf("NextTimer() after reset = (%v, %v), want (0.5, true)", got, ok)
	}
}

func TestOnReloadClearsRegisteredTimers(t *testing.T) {
	resetStateForTest()
	Start(nil)
	RegisterTimer(0.1)
	applyFrameUpdate(0.2, 0.2, 0.2, 0.2, 60)

	OnReload()

	if got := Timer(); got != 0 {
		t.Fatalf("Timer() = %v, want 0", got)
	}
	if len(timestamps) != 0 {
		t.Fatalf("len(timestamps) = %d, want 0", len(timestamps))
	}
	if _, ok := NextTimer(); ok {
		t.Fatal("did not expect any timers after reload")
	}
}
