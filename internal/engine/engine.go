package engine

import (
	"fmt"
	"sync"

	stime "time"

	"github.com/goplus/spx/v2/internal/engine/profiler"
	"github.com/goplus/spx/v2/internal/enginewrap"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	gde "github.com/goplus/spx/v2/pkg/gdspx/pkg/gdspx"
)

// copy these variable to any namespace you want
var (
	audioMgr    enginewrap.AudioMgrImpl
	cameraMgr   enginewrap.CameraMgrImpl
	inputMgr    enginewrap.InputMgrImpl
	physicMgr   enginewrap.PhysicMgrImpl
	platformMgr enginewrap.PlatformMgrImpl
	resMgr      enginewrap.ResMgrImpl
	extMgr      enginewrap.ExtMgrImpl
	sceneMgr    enginewrap.SceneMgrImpl
	spriteMgr   enginewrap.SpriteMgrImpl
	uiMgr       enginewrap.UiMgrImpl
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

// SetLayerSortMode configures automatic layer sorting for sprites.
// Supported modes:
//   - "" or "none": Disables automatic sorting (default)
//   - "vertical": Sorts by Y-coordinate (descending), then X-coordinate (descending)
//
// When enabled, manual layer control methods are disabled to prevent conflicts.
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
	return float64(val) / engine.Float2IntFactor
}
func ConvertToInt64(val float64) int64 {
	return int64(val * engine.Float2IntFactor)
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

	// time
	startTimestamp     stime.Time
	lastTimestamp      stime.Time
	timeSinceLevelLoad float64

	logicMutex sync.Mutex
	// statistic info
	fps float64
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
	gde.LinkEngine(gdx.EngineCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
		OnEngineReset:   onReset,
		OnEnginePause:   onPaused,
		OnKeyPressed:    onKeyPressed,
		OnKeyReleased:   onKeyReleased,
	})
}

func RegisterFuncs() {

}
func OnGameStarted() {
	gco.OnInited()
}

// callbacks
func onStart() {
	defer CheckPanic()
	triggerEventsTemp = make([]TriggerEvent, 0)
	triggerEvents = make([]TriggerEvent, 0)
	keyEventsTemp = make([]KeyEvent, 0)
	keyEvents = make([]KeyEvent, 0)

	time.Start(func(scale float64) {
		platformMgr.SetTimeScale(scale)
	})

	startTimestamp = stime.Now()
	lastTimestamp = stime.Now()
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
	engine.ClearAllSprites()
	game.OnEngineReset()
	gco.AbortAll()
	gde.UnlinkEngine()
}

func onKeyPressed(id int64) {
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: true})
}

func onKeyReleased(id int64) {
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: false})
}

func updateTime(delta float64) {
	deltaTime := delta
	timeSinceLevelLoad += deltaTime

	curTime := stime.Now()
	unscaledTimeSinceLevelLoad := curTime.Sub(startTimestamp).Seconds()
	unscaledDeltaTime := curTime.Sub(lastTimestamp).Seconds()
	lastTimestamp = curTime
	timeScale := SyncGetTimeScale()
	fps = profiler.Calcfps()
	time.Update(float64(timeScale), unscaledTimeSinceLevelLoad, timeSinceLevelLoad, deltaTime, unscaledDeltaTime, fps)
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

func CheckPanic() {
	if e := recover(); e != nil {
		OnPanic("", "")
		//panic(e)
	}
}

func OnPanic(name, stack string) {
	// on coro panic, exit game
	msg := name
	if stack != "" {
		msg += " stack:\n" + stack
	}
	extMgr.OnRuntimePanic(msg)
	RequestExit(1)
}

func RequestExit(exitCode int64) {
	extMgr.RequestReset(exitCode)
}
