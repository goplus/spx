package runtime

import (
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestSwipeStateFinishDispatchesTarget(t *testing.T) {
	var (
		debugged []SwipeEvent[string]
		targets  []string
		stages   []float64
	)
	var state SwipeState[string]
	state.Init()
	state.Begin(mathf.Vec2{}, "sprite")
	state.OnMouseMove(mathf.Vec2{X: 100}, SwipeHooks[string]{})
	state.Finish(mathf.Vec2{X: 100}, SwipeHooks[string]{
		Debug: func(ev SwipeEvent[string]) {
			debugged = append(debugged, ev)
		},
		DispatchTarget: func(direction float64, target string) {
			targets = append(targets, target)
			if direction != 90 {
				t.Fatalf("direction = %v, want 90", direction)
			}
		},
		DispatchStage: func(direction float64) {
			stages = append(stages, direction)
		},
	})

	if len(debugged) != 1 || debugged[0].Target != "sprite" {
		t.Fatalf("debugged = %+v, want target event", debugged)
	}
	if len(targets) != 1 || targets[0] != "sprite" {
		t.Fatalf("targets = %+v, want [sprite]", targets)
	}
	if len(stages) != 0 {
		t.Fatalf("stages = %+v, want none", stages)
	}
	if state.target != "" {
		t.Fatalf("state.target = %q, want cleared", state.target)
	}
}

func TestSwipeStateFinishDispatchesStage(t *testing.T) {
	var (
		debugged []SwipeEvent[*int]
		stageDir []float64
	)
	var state SwipeState[*int]
	state.Init()
	state.Begin(mathf.Vec2{}, nil)
	state.OnMouseMove(mathf.Vec2{X: 100}, SwipeHooks[*int]{})
	state.Finish(mathf.Vec2{X: 100}, SwipeHooks[*int]{
		Debug: func(ev SwipeEvent[*int]) {
			debugged = append(debugged, ev)
		},
		DispatchStage: func(direction float64) {
			stageDir = append(stageDir, direction)
		},
	})

	if len(debugged) != 1 || debugged[0].Target != nil {
		t.Fatalf("debugged = %+v, want stage event", debugged)
	}
	if len(stageDir) != 1 || stageDir[0] != 90 {
		t.Fatalf("stageDir = %+v, want [90]", stageDir)
	}
}

func TestSwipeStateFinishClearsTargetWhenSwipeFails(t *testing.T) {
	var state SwipeState[string]
	state.Init()
	state.Begin(mathf.Vec2{}, "sprite")
	state.Finish(mathf.Vec2{}, SwipeHooks[string]{})

	if state.target != "" {
		t.Fatalf("state.target = %q, want cleared", state.target)
	}
}
