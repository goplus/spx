package engine

import (
	. "github.com/goplus/spx/v2/pkg/gdspx/internal/wrap"
	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var (
	mgrs                []IManager
	callback            EngineCallbackInfo
	sprites             = make([]ISpriter, 0)
	timer               = float64(0)
	isWebIntepreterMode bool
)

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}

func Link(engineCallback EngineCallbackInfo) {
	isWebIntepreterMode = LinkFFI()
	mgrs = CreateMgrs()
	callback = engineCallback
	infos := bindCallbacks()
	RegisterCallbacks(infos)
	BindMgr(mgrs)
	InternalInitEngine()
	OnLinked()
}

func onEngineStart() {
	for _, mgr := range mgrs {
		mgr.OnStart()
	}
	if callback.OnEngineStart != nil {
		callback.OnEngineStart()
	}
}

func onEngineUpdate(delta float64) {
	for _, mgr := range mgrs {
		mgr.OnUpdate(delta)
	}
	TimeSinceGameStart += delta
	sprites = sprites[:0]
	for _, sprite := range Id2Sprites {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnUpdate(delta)
	}
	if callback.OnEngineUpdate != nil {
		callback.OnEngineUpdate(delta)
	}
	InternalUpdateEngine(delta)
}

func onEngineFixedUpdate(delta float64) {
	for _, mgr := range mgrs {
		mgr.OnFixedUpdate(delta)
	}
	TimeSinceGameStart += delta
	sprites = sprites[:0]
	for _, sprite := range Id2Sprites {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnFixedUpdate(delta)
	}
	if callback.OnEngineFixedUpdate != nil {
		callback.OnEngineFixedUpdate(delta)
	}
}
func onEngineDestroy() {
	if callback.OnEngineDestroy != nil {
		callback.OnEngineDestroy()
	}
	sprites = sprites[:0]
	for _, sprite := range Id2Sprites {
		sprites = append(sprites, sprite)
	}
	for _, sprite := range sprites {
		sprite.OnDestroy()
	}
	for _, mgr := range mgrs {
		mgr.OnDestroy()
	}
}
