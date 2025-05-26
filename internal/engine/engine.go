package engine

import (
	"sync"

	stime "time"

	"github.com/goplus/spx/internal/engine/profiler"
	"github.com/goplus/spx/internal/enginewrap"
	"github.com/goplus/spx/internal/time"
	gdx "github.com/goplus/spx/pkg/gdspx/pkg/engine"
	gde "github.com/goplus/spx/pkg/gdspx/pkg/gdspx"
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

	// statistic info
	fps float64
)

type IGame interface {
	OnEngineStart()
	OnEngineUpdate(delta float64)
	OnEngineRender(delta float64)
	OnEngineDestroy()
}

func Main(g IGame) {
	enginewrap.Init(WaitMainThread)
	game = g
	gde.LinkEngine(gdx.EngineCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
		OnKeyPressed:    onKeyPressed,
		OnKeyReleased:   onKeyReleased,
	})
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
		panic(e)
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
	extMgr.RequestExit(exitCode)
}
