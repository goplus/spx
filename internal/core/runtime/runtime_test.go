package runtime

import (
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

func TestProcessInputFrame(t *testing.T) {
	var (
		downs []mathf.Vec2
		moves []mathf.Vec2
		keys  []int64
	)

	lastMousePos, lastPressed := ProcessInputFrame(
		InputFrame{
			Point:                    mathf.Vec2{X: 10, Y: 20},
			LastMousePos:             mathf.Vec2{},
			LastLeftButtonPressed:    false,
			CurrentLeftButtonPressed: true,
			KeyEvents: []engine.KeyEvent{
				{Id: 1, IsPressed: true},
				{Id: 2, IsPressed: false},
			},
			MouseMovementThreshold: 1,
		},
		InputFrameHooks{
			FireLeftButtonDown: func(pos mathf.Vec2) { downs = append(downs, pos) },
			FireLeftButtonUp:   func(mathf.Vec2) {},
			SetMousePos:        func(pos mathf.Vec2) { moves = append(moves, pos) },
			OnMouseMove:        func(mathf.Vec2) {},
			OnKeyPressed:       func(key int64) { keys = append(keys, key) },
		},
	)

	if len(downs) != 1 || downs[0].X != 10 || downs[0].Y != 20 {
		t.Fatalf("unexpected button downs: %+v", downs)
	}
	if len(moves) != 1 || moves[0].X != 10 || moves[0].Y != 20 {
		t.Fatalf("unexpected mouse moves: %+v", moves)
	}
	if len(keys) != 1 || keys[0] != 1 {
		t.Fatalf("unexpected key presses: %+v", keys)
	}
	if lastMousePos.X != 10 || lastMousePos.Y != 20 {
		t.Fatalf("lastMousePos = %+v, want {10 20}", lastMousePos)
	}
	if !lastPressed {
		t.Fatal("expected left button state to update")
	}
}

func TestFindClickTarget(t *testing.T) {
	selection, ok := FindClickTarget([]int{1, 2, 3}, func(item int) (ClickSelection[int, int], bool) {
		if item >= 2 {
			return ClickSelection[int, int]{Target: item, SwipeTarget: item * 10}, true
		}
		return ClickSelection[int, int]{}, false
	})
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != 3 || selection.SwipeTarget != 30 {
		t.Fatalf("selection = %+v, want {Target:3 SwipeTarget:30}", selection)
	}
}

func TestHandleLeftButtonDownTarget(t *testing.T) {
	var (
		begins []int
		gates  []int
		hits   []int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{X: 1, Y: 2}, ClickDownHooks[int, int, int]{
		FindTarget: func(point mathf.Vec2) (ClickSelection[int, int], bool) {
			if point.X != 1 || point.Y != 2 {
				t.Fatalf("point = %+v, want {1 2}", point)
			}
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(point mathf.Vec2, target int) {
			if point.X != 1 || point.Y != 2 {
				t.Fatalf("point = %+v, want {1 2}", point)
			}
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return true
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(target int) {
			hits = append(hits, target)
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 70 {
		t.Fatalf("begins = %+v, want [70]", begins)
	}
	if len(gates) != 2 || gates[0] != -1 || gates[1] != 7 {
		t.Fatalf("gates = %+v, want [-1 7]", gates)
	}
	if len(hits) != 1 || hits[0] != 7 {
		t.Fatalf("hits = %+v, want [7]", hits)
	}
	if stages != 0 {
		t.Fatalf("stages = %d, want 0", stages)
	}
}

func TestHandleLeftButtonDownBlockedByGlobalGate(t *testing.T) {
	var (
		begins []int
		gates  []int
		hits   int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(_ mathf.Vec2, target int) {
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return id != -1
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(int) {
			hits++
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 70 {
		t.Fatalf("begins = %+v, want [70]", begins)
	}
	if len(gates) != 1 || gates[0] != -1 {
		t.Fatalf("gates = %+v, want [-1]", gates)
	}
	if hits != 0 || stages != 0 {
		t.Fatalf("hits=%d stages=%d, want 0/0", hits, stages)
	}
}

func TestHandleLeftButtonDownStage(t *testing.T) {
	var (
		begins []int
		gates  []int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{}, false
		},
		BeginSwipe: func(_ mathf.Vec2, target int) {
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return true
		},
		GlobalID: -1,
		StageID:  0,
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 0 {
		t.Fatalf("begins = %+v, want [0]", begins)
	}
	if len(gates) != 2 || gates[0] != -1 || gates[1] != 0 {
		t.Fatalf("gates = %+v, want [-1 0]", gates)
	}
	if stages != 1 {
		t.Fatalf("stages = %d, want 1", stages)
	}
}

func TestHandleLeftButtonDownBlockedTargetGate(t *testing.T) {
	var (
		gates  []int
		hits   int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(mathf.Vec2, int) {},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return id != 7
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(int) {
			hits++
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(gates) != 2 || gates[0] != -1 || gates[1] != 7 {
		t.Fatalf("gates = %+v, want [-1 7]", gates)
	}
	if hits != 0 || stages != 0 {
		t.Fatalf("hits=%d stages=%d, want 0/0", hits, stages)
	}
}

func TestProcessLogicFrame(t *testing.T) {
	var fired []float64
	nextTimers := []float64{2.5, 3}
	nextTimerIndex := 0
	audios, animations := ProcessLogicFrame(LogicFrameConfig[int]{
		Items:          []int{1, 2},
		TempAudios:     []string{"seed"},
		TempAnimations: []string{"seed"},
		FlushPendingAudio: func(item int, acc []string) []string {
			return append(acc, "a")
		},
		FlushCompletedAnimations: func(item int, acc []string) []string {
			return append(acc, "n")
		},
		NextTimer: func() (float64, bool) {
			if nextTimerIndex >= len(nextTimers) {
				return 0, false
			}
			timer := nextTimers[nextTimerIndex]
			nextTimerIndex++
			return timer, true
		},
		FireTimer: func(v float64) { fired = append(fired, v) },
	})

	if len(audios) != 3 {
		t.Fatalf("audios len = %d, want 3", len(audios))
	}
	if len(animations) != 3 {
		t.Fatalf("animations len = %d, want 3", len(animations))
	}
	if len(fired) != 2 || fired[0] != 2.5 || fired[1] != 3 {
		t.Fatalf("unexpected fired timers: %+v", fired)
	}
}

func TestMainSchedTimedOut(t *testing.T) {
	now := time.Unix(20, 0)
	if !MainSchedTimedOut(ScheduleState{
		IsSchedInMain:   true,
		MainSchedTime:   time.Unix(10, 0),
		Now:             now,
		MainExecTimeout: 5 * time.Second,
	}) {
		t.Fatal("expected main scheduler timeout")
	}
	if MainSchedTimedOut(ScheduleState{
		IsSchedInMain:   false,
		MainSchedTime:   time.Unix(10, 0),
		Now:             now,
		MainExecTimeout: 5 * time.Second,
	}) {
		t.Fatal("did not expect timeout when not in main sched")
	}
}

func TestSchedNow(t *testing.T) {
	scheduled := false
	err := SchedNow(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		SchedulerHooks{
			SchedCurrent: func() { scheduled = true },
		},
	)
	if err != nil {
		t.Fatalf("SchedNow error: %v", err)
	}
	if !scheduled {
		t.Fatal("expected current thread to be scheduled")
	}

	err = SchedNow(
		ScheduleState{
			IsSchedInMain:   true,
			MainSchedTime:   time.Unix(1, 0),
			Now:             time.Unix(5, 0),
			MainExecTimeout: 2 * time.Second,
		},
		SchedulerHooks{},
	)
	if err != ErrMainExecutionTimedOut {
		t.Fatalf("SchedNow timeout error = %v, want %v", err, ErrMainExecutionTimedOut)
	}
}

func TestSched(t *testing.T) {
	called := false
	err := Sched(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		3000,
		SchedulerHooks{
			IsSchedTimeout: func(ms float64) bool {
				return ms == 3000
			},
			OnSchedTimeout: func() {
				called = true
			},
		},
	)
	if err != ErrLoopExecutionTimedOut {
		t.Fatalf("Sched error = %v, want %v", err, ErrLoopExecutionTimedOut)
	}
	if !called {
		t.Fatal("expected sched timeout hook to run")
	}

	err = Sched(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		3000,
		SchedulerHooks{
			IsSchedTimeout: func(float64) bool { return true },
		},
	)
	if err != ErrLoopExecutionTimedOut {
		t.Fatalf("Sched timeout error = %v, want %v", err, ErrLoopExecutionTimedOut)
	}
}

func TestRunMain(t *testing.T) {
	var (
		flags []bool
		at    time.Time
		calls int
	)
	now := time.Unix(123, 0)
	RunMain(
		func() { calls++ },
		now,
		func(v bool) { flags = append(flags, v) },
		func(t time.Time) { at = t },
	)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if len(flags) != 2 || !flags[0] || flags[1] {
		t.Fatalf("flags = %+v, want [true false]", flags)
	}
	if !at.Equal(now) {
		t.Fatalf("at = %v, want %v", at, now)
	}
}

func TestRepeatAndWaitUntil(t *testing.T) {
	var (
		repeatCalls int
		waitCalls   int
	)
	Repeat(3, func() {
		repeatCalls++
	}, func() {
		waitCalls++
	})
	if repeatCalls != 3 || waitCalls != 3 {
		t.Fatalf("repeatCalls=%d waitCalls=%d, want 3/3", repeatCalls, waitCalls)
	}

	condCalls := 0
	bodyCalls := 0
	waitCalls = 0
	RepeatUntil(
		func() bool {
			condCalls++
			return condCalls > 2
		},
		func() {
			bodyCalls++
		},
		func() {
			waitCalls++
		},
	)
	if bodyCalls != 2 || waitCalls != 2 {
		t.Fatalf("RepeatUntil body=%d wait=%d, want 2/2", bodyCalls, waitCalls)
	}

	condCalls = 0
	waitCalls = 0
	WaitUntil(
		func() bool {
			condCalls++
			return condCalls > 2
		},
		func() {
			waitCalls++
		},
	)
	if waitCalls != 2 {
		t.Fatalf("WaitUntil waitCalls=%d, want 2", waitCalls)
	}
}

func TestSyncBatchPositions(t *testing.T) {
	type target struct {
		id   int64
		x, y float64
	}

	items := []*target{{id: 1}, {id: 2}, {id: 3}}
	SyncBatchPositions(
		items,
		func(item *target) bool { return item.id != 2 },
		func(item *target) int64 { return item.id },
		func(ids []int64) []float32 {
			if len(ids) != 2 || ids[0] != 1 || ids[1] != 3 {
				t.Fatalf("unexpected ids: %+v", ids)
			}
			return []float32{11, 12, 31, 32}
		},
		func(item *target, x, y float64) {
			item.x, item.y = x, y
		},
	)

	if items[0].x != 11 || items[0].y != 12 {
		t.Fatalf("item 0 = %+v, want synced coordinates", items[0])
	}
	if items[1].x != 0 || items[1].y != 0 {
		t.Fatalf("item 1 should remain untouched: %+v", items[1])
	}
	if items[2].x != 31 || items[2].y != 32 {
		t.Fatalf("item 2 = %+v, want synced coordinates", items[2])
	}
}

func TestProcessTriggerPairs(t *testing.T) {
	pairs := []engine.TriggerEvent{
		{
			Src: &engine.Sprite{Target: "src"},
			Dst: &engine.Sprite{Target: "dst"},
		},
		{
			Src: &engine.Sprite{Target: "bad"},
			Dst: &engine.Sprite{Target: "dst"},
		},
	}

	var (
		touches  [][2]string
		invalids int
	)
	ProcessTriggerPairs(
		pairs,
		func(target any) (string, bool) {
			v, ok := target.(string)
			return v, ok && v != "bad"
		},
		func(v string) bool { return v != "dst-blocked" },
		func(src, dst string) { touches = append(touches, [2]string{src, dst}) },
		func() { invalids++ },
	)

	if len(touches) != 1 || touches[0] != [2]string{"src", "dst"} {
		t.Fatalf("unexpected touches: %+v", touches)
	}
	if invalids != 1 {
		t.Fatalf("invalids = %d, want 1", invalids)
	}
}
