package runtime

import "github.com/goplus/spbase/mathf"

type ClickSelection[T any, S any] struct {
	Target      T
	SwipeTarget S
}

func FindClickTarget[I any, T any, S any](items []I, match func(I) (ClickSelection[T, S], bool)) (ClickSelection[T, S], bool) {
	for i := len(items) - 1; i >= 0; i-- {
		if selection, ok := match(items[i]); ok {
			return selection, true
		}
	}
	return ClickSelection[T, S]{}, false
}

type ClickDownHooks[T any, S any, ID comparable] struct {
	FindTarget     func(mathf.Vec2) (ClickSelection[T, S], bool)
	BeginSwipe     func(mathf.Vec2, S)
	CanTrigger     func(ID) bool
	GlobalID       ID
	StageID        ID
	TargetID       func(T) (ID, bool)
	DispatchTarget func(T)
	DispatchStage  func()
}

func HandleLeftButtonDown[T any, S any, ID comparable](point mathf.Vec2, hooks ClickDownHooks[T, S, ID]) {
	selection, ok := ClickSelection[T, S]{}, false
	if hooks.FindTarget != nil {
		selection, ok = hooks.FindTarget(point)
	}
	if hooks.BeginSwipe != nil {
		hooks.BeginSwipe(point, selection.SwipeTarget)
	}
	if !allowClick(hooks.CanTrigger, hooks.GlobalID) {
		return
	}
	if ok {
		if hooks.TargetID != nil {
			targetID, ok := hooks.TargetID(selection.Target)
			if !ok || !allowClick(hooks.CanTrigger, targetID) {
				return
			}
		}
		if hooks.DispatchTarget != nil {
			hooks.DispatchTarget(selection.Target)
		}
		return
	}
	if allowClick(hooks.CanTrigger, hooks.StageID) && hooks.DispatchStage != nil {
		hooks.DispatchStage()
	}
}

func allowClick[ID comparable](canTrigger func(ID) bool, id ID) bool {
	if canTrigger == nil {
		return true
	}
	return canTrigger(id)
}
