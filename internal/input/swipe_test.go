package input

import (
	"math"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
)

func TestSwipeRecognizerFinishDetectsRightSwipe(t *testing.T) {
	now := time.Unix(0, 0)
	sr := SwipeRecognizer{
		now: func() time.Time { return now },
	}
	sr.Init()

	start := mathf.NewVec2(0, 0)
	end := mathf.NewVec2(100, 0)

	sr.StartTracking(start)
	now = now.Add(100 * time.Millisecond)
	if _, ok := sr.OnMouseMove(end); ok {
		t.Fatal("expected move-only tracking not to emit swipe before finish")
	}

	now = now.Add(10 * time.Millisecond)
	result, ok := sr.Finish(end)
	if !ok {
		t.Fatal("expected swipe to be detected")
	}
	if sr.IsTracking() {
		t.Fatal("expected tracking to stop after finish")
	}
	if result.Direction != 90 {
		t.Fatalf("unexpected swipe direction: got %v", result.Direction)
	}
	if result.Distance != 100 {
		t.Fatalf("unexpected swipe distance: got %v", result.Distance)
	}
	if result.StartPos != start || result.EndPos != end {
		t.Fatalf("unexpected swipe points: got start=%v end=%v", result.StartPos, result.EndPos)
	}
	if math.Abs(result.Velocity-909.090909090909) > 0.0001 {
		t.Fatalf("unexpected swipe velocity: got %v", result.Velocity)
	}
}

func TestSwipeRecognizerTimeLimitStopsTracking(t *testing.T) {
	now := time.Unix(0, 0)
	sr := SwipeRecognizer{
		now: func() time.Time { return now },
	}
	sr.Init()

	sr.StartTracking(mathf.NewVec2(0, 0))
	now = now.Add(600 * time.Millisecond)

	if _, ok := sr.OnMouseMove(mathf.NewVec2(100, 0)); ok {
		t.Fatal("expected expired swipe not to emit a result")
	}
	if sr.IsTracking() {
		t.Fatal("expected expired swipe tracking to stop")
	}
}
