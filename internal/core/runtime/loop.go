package runtime

import (
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
)

func RunEventLoop[T any](me coroutine.Thread, events chan T, handle func(T)) int {
	for {
		var ev T
		engine.WaitForChan(events, &ev)
		handle(ev)
	}
}

type InputFrame struct {
	Point                    mathf.Vec2
	LastMousePos             mathf.Vec2
	LastLeftButtonPressed    bool
	CurrentLeftButtonPressed bool
	KeyEvents                []engine.KeyEvent
	MouseMovementThreshold   float64
}

type InputFrameHooks struct {
	FireLeftButtonDown func(mathf.Vec2)
	FireLeftButtonUp   func(mathf.Vec2)
	SetMousePos        func(mathf.Vec2)
	OnMouseMove        func(mathf.Vec2)
	OnKeyPressed       func(int64)
}

func ProcessInputFrame(frame InputFrame, hooks InputFrameHooks) (mathf.Vec2, bool) {
	if frame.CurrentLeftButtonPressed != frame.LastLeftButtonPressed {
		if frame.LastLeftButtonPressed {
			hooks.FireLeftButtonUp(frame.Point)
		} else {
			hooks.FireLeftButtonDown(frame.Point)
		}
	}

	lastMousePos := frame.LastMousePos
	dx := frame.Point.X - frame.LastMousePos.X
	dy := frame.Point.Y - frame.LastMousePos.Y
	if math.Abs(dx) > frame.MouseMovementThreshold || math.Abs(dy) > frame.MouseMovementThreshold {
		hooks.SetMousePos(frame.Point)
		hooks.OnMouseMove(frame.Point)
		lastMousePos = frame.Point
	}

	for _, ev := range frame.KeyEvents {
		if ev.IsPressed {
			hooks.OnKeyPressed(ev.Id)
		}
	}

	return lastMousePos, frame.CurrentLeftButtonPressed
}

type InputLoopConfig struct {
	CurrentMousePos        func() mathf.Vec2
	IsLeftButtonPressed    func() bool
	FireLeftButtonDown     func(mathf.Vec2)
	FireLeftButtonUp       func(mathf.Vec2)
	SetMousePos            func(mathf.Vec2)
	OnMouseMove            func(mathf.Vec2)
	GetKeyEvents           func([]engine.KeyEvent) []engine.KeyEvent
	OnKeyPressed           func(int64)
	MouseMovementThreshold float64
}

func RunInputLoop(me coroutine.Thread, cfg InputLoopConfig) int {
	lastLeftButtonPressed := false
	lastMousePos := mathf.Vec2{}
	keyEvents := make([]engine.KeyEvent, 0)

	for {
		point := cfg.CurrentMousePos()
		// Keep the cached mouse position in sync with the engine every frame,
		// so callers don't need a second engine-side mouse query elsewhere.
		cfg.SetMousePos(point)
		keyEvents = cfg.GetKeyEvents(keyEvents)
		lastMousePos, lastLeftButtonPressed = ProcessInputFrame(
			InputFrame{
				Point:                    point,
				LastMousePos:             lastMousePos,
				LastLeftButtonPressed:    lastLeftButtonPressed,
				CurrentLeftButtonPressed: cfg.IsLeftButtonPressed(),
				KeyEvents:                keyEvents,
				MouseMovementThreshold:   cfg.MouseMovementThreshold,
			},
			InputFrameHooks{
				FireLeftButtonDown: cfg.FireLeftButtonDown,
				FireLeftButtonUp:   cfg.FireLeftButtonUp,
				SetMousePos:        cfg.SetMousePos,
				OnMouseMove:        cfg.OnMouseMove,
				OnKeyPressed:       cfg.OnKeyPressed,
			},
		)
		keyEvents = keyEvents[:0]
		engine.WaitNextFrame()
	}
}

type LogicFrameConfig[T any] struct {
	Items                    []T
	TempAudios               []string
	TempAnimations           []string
	FlushPendingAudio        func(T, []string) []string
	FlushCompletedAnimations func(T, []string) []string
	NextTimer                func() (float64, bool)
	FireTimer                func(float64)
}

func ProcessLogicFrame[T any](cfg LogicFrameConfig[T]) ([]string, []string) {
	tempAudios := cfg.TempAudios
	for _, item := range cfg.Items {
		tempAudios = cfg.FlushPendingAudio(item, tempAudios)
	}

	tempAnimations := cfg.TempAnimations
	for _, item := range cfg.Items {
		tempAnimations = cfg.FlushCompletedAnimations(item, tempAnimations)
	}

	for {
		targetTimer, ok := cfg.NextTimer()
		if !ok {
			break
		}
		cfg.FireTimer(targetTimer)
	}
	return tempAudios, tempAnimations
}

type LogicLoopConfig[T any] struct {
	Items                    func() []T
	FlushPendingAudio        func(T, []string) []string
	FlushCompletedAnimations func(T, []string) []string
	NextTimer                func() (float64, bool)
	FireTimer                func(float64)
	ShowDebugPanel           func()
}

func RunLogicLoop[T any](me coroutine.Thread, cfg LogicLoopConfig[T]) int {
	tempAudios := []string{}
	tempAnimations := []string{}

	for {
		tempAudios, tempAnimations = ProcessLogicFrame(LogicFrameConfig[T]{
			Items:                    cfg.Items(),
			TempAudios:               tempAudios,
			TempAnimations:           tempAnimations,
			FlushPendingAudio:        cfg.FlushPendingAudio,
			FlushCompletedAnimations: cfg.FlushCompletedAnimations,
			NextTimer:                cfg.NextTimer,
			FireTimer:                cfg.FireTimer,
		})
		engine.WaitNextFrame()
		cfg.ShowDebugPanel()
	}
}

func InitLoops(
	create func(coroutine.ThreadObj, func(coroutine.Thread) int) coroutine.Thread,
	eventLoop func(coroutine.Thread) int,
	inputLoop func(coroutine.Thread) int,
	logicLoop func(coroutine.Thread) int,
) {
	create("eventLoop", eventLoop)
	create("inputEventLoop", inputLoop)
	create("logicLoop", logicLoop)
}
