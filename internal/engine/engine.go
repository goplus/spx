package engine

import (
	"fmt"
	"sync"
	stime "time"

	"github.com/goplus/spx/v2/internal/engine/platform"
	"github.com/goplus/spx/v2/internal/engine/profiler"
	"github.com/goplus/spx/v2/internal/enginewrap"
	gde "github.com/goplus/spx/v2/internal/gdengine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"

	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// Shared engine managers.
var (
	platformMgr enginewrap.PlatformMgrImpl
	resMgr      enginewrap.ResMgrImpl
	extMgr      enginewrap.ExtMgrImpl
)

type Object = gdx.Object
type Array = gdx.Array

type layerSortMode int

const (
	layerSortModeNone layerSortMode = iota
	layerSortModeVertical
)

type LayerSortInfo struct {
	X      float64
	Y      float64
	Sprite *Sprite
}

var curLayerSortMode layerSortMode

// SetLayerSortMode sets sprite layer sorting.
// Supported modes:
//   - "" or "none": disable sorting (default)
//   - "vertical": sort by Y, then X, both descending
//
// When enabled, manual layer changes are disabled.
func SetLayerSortMode(s string) error {
	switch s {
	case "", "none":
		curLayerSortMode = layerSortModeNone
	case "vertical":
		curLayerSortMode = layerSortModeVertical
	default:
		return fmt.Errorf("unknown layer sort mode: %s", s)
	}

	extMgr.SetLayerSorterMode(int64(curLayerSortMode))
	return nil
}

func HasLayerSortMethod() bool {
	return curLayerSortMode != layerSortModeNone
}

const Float2IntFactor = gdx.Float2IntFactor

func ConvertToFloat64(val int64) float64 {
	return float64(val) / Float2IntFactor
}
func ConvertToInt64(val float64) int64 {
	return int64(val * Float2IntFactor)
}

type TriggerEvent struct {
	Src *Sprite
	Dst *Sprite
}
type KeyEvent struct {
	Id        int64
	IsPressed bool
}

var (
	game              IGame
	triggerEventsTemp []TriggerEvent
	triggerEvents     []TriggerEvent
	triggerMutex      sync.Mutex

	keyEventsTemp []KeyEvent
	keyEvents     []KeyEvent
	keyMutex      sync.Mutex

	logicMutex sync.Mutex
)

func Lock() {
	logicMutex.Lock()
}

func Unlock() {
	logicMutex.Unlock()
}

type IGame interface {
	OnEngineStart()
	OnEngineUpdate(delta float64)
	OnEngineRender(delta float64)
	OnEngineDestroy()
	OnEngineReset()
	OnEnginePause(isPaused bool)
}

func Main(g IGame) {
	enginewrap.Init(WaitMainThread)
	game = g
	gde.Link(gdx.CoreCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
		OnEngineReset:   onReset,
		OnEnginePause:   onPaused,
		OnMousePressed:  onMousePressed,
		OnMouseReleased: onMouseReleased,
		OnKeyPressed:    onKeyPressed,
		OnKeyReleased:   onKeyReleased,
	})
}

func OnGameStarted() {
	gco.OnInited()
}

// Engine callbacks.
func onStart() {
	defer CheckPanic()
	resetMouseButtonStates()
	triggerEventsTemp = make([]TriggerEvent, 0)
	triggerEvents = make([]TriggerEvent, 0)
	keyEventsTemp = make([]KeyEvent, 0)
	keyEvents = make([]KeyEvent, 0)

	time.Start(func(scale float64) {
		platformMgr.SetTimeScale(scale)
	})
	game.OnEngineStart()
}

func onUpdate(delta float64) {
	defer CheckPanic()
	profiler.BeginSample()
	updateTime(float64(delta))
	cacheTriggerEvents()
	cacheKeyEvents()
	profiler.MeasureFunctionTime("GameUpdate", func() {
		game.OnEngineUpdate(delta)
	})
	profiler.MeasureFunctionTime("CoroUpdateJobs", func() {
		gco.Update()
	})
	profiler.MeasureFunctionTime("GameRender", func() {
		game.OnEngineRender(delta)
	})
	profiler.EndSample()
}

func onDestroy() {
	game.OnEngineDestroy()
}

func onPaused(isPaused bool) {
	game.OnEnginePause(isPaused)
}

func onReset() {
	game.OnEngineReset()
	gde.Unlink()
}

func onKeyPressed(id int64) {
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: true})
}

func onKeyReleased(id int64) {
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: false})
}

func onMousePressed(id int64) {
	setMouseButtonPressed(id, true)
}

func onMouseReleased(id int64) {
	setMouseButtonPressed(id, false)
}

func updateTime(delta float64) {
	time.Update(delta, profiler.Calcfps())
}

func cacheTriggerEvents() {
	triggerMutex.Lock()
	triggerEvents = append(triggerEvents, triggerEventsTemp...)
	triggerMutex.Unlock()
	triggerEventsTemp = triggerEventsTemp[:0]
}

func GetTriggerEvents(lst []TriggerEvent) []TriggerEvent {
	triggerMutex.Lock()
	lst = append(lst, triggerEvents...)
	triggerEvents = triggerEvents[:0]
	triggerMutex.Unlock()
	return lst
}

func cacheKeyEvents() {
	keyMutex.Lock()
	keyEvents = append(keyEvents, keyEventsTemp...)
	keyMutex.Unlock()
	keyEventsTemp = keyEventsTemp[:0]
}

func GetKeyEvents(lst []KeyEvent) []KeyEvent {
	keyMutex.Lock()
	lst = append(lst, keyEvents...)
	keyEvents = keyEvents[:0]
	keyMutex.Unlock()
	return lst
}

// DeferPanic recovers a panic, reports it, and optionally exits.
func DeferPanic(name, stack string, exitOnPanic bool) {
	if e := recover(); e != nil {
		handlePanic(name, stack, e, exitOnPanic)
	}
}

// CheckPanic is a shorthand panic handler for engine callbacks.
func CheckPanic() {
	if e := recover(); e != nil {
		handlePanic("", "", e, true)
	}
}

// OnPanic reports a panic with an optional name and stack.
func OnPanic(name, stack string) {
	handlePanic(name, stack, nil, true)
}

// handlePanic reports a panic and optionally exits.
func handlePanic(name, stack string, err any, exitOnPanic bool) {
	var msg string
	if err != nil {
		msg = fmt.Sprintf("panic: %v", err)
	}
	if name != "" {
		if msg != "" {
			msg = name + ": " + msg
		} else {
			msg = name
		}
	}
	if stack != "" {
		msg += "\nstack:\n" + stack
	}

	extMgr.OnRuntimePanic(msg)

	if exitOnPanic {
		RequestExit(1)
	}
}

// Panic reports a panic message through the engine.
func Panic(args ...any) {
	msg := fmt.Sprint(args...)
	OnPanic(msg, "")
}

// Panicf reports a formatted panic message through the engine.
func Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	OnPanic(msg, "")
}

// abortCoroutinesAndReset aborts coroutines and resets the engine.
// Used on web, where the process cannot exit.
func abortCoroutinesAndReset(exitCode int64) {
	completed := gco.AbortAllAndWait(2 * stime.Second)
	spxlog.Debug("AbortAllAndWait completed: %v. Requesting engine reset.", completed)
	extMgr.RequestReset(exitCode)
}

func RequestExit(exitCode int64) {
	if platform.IsWeb() {
		// Web resets instead of exiting.
		abortCoroutinesAndReset(exitCode)
		return
	}
	extMgr.RequestExit(exitCode)
}
