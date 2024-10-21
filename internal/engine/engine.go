package engine

import (
	. "godot-ext/gdspx/pkg/engine"
	"godot-ext/gdspx/pkg/gdspx"
	"sync"
)

type TriggerPair struct {
	Src *ProxySprite
	Dst *ProxySprite
}

var (
	game             Gamer
	tempTriggerPairs []TriggerPair
	TriggerPairs     []TriggerPair
	mu               sync.Mutex
)

type Gamer interface {
	OnEngineStart()
	OnEngineUpdate(delta float32)
	OnEngineDestroy()
}

func GdspxMain(g Gamer) {
	game = g
	gdspx.LinkEngine(EngineCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
	})
}

// callbacks
func onStart() {
	tempTriggerPairs = make([]TriggerPair, 0)
	TriggerPairs = make([]TriggerPair, 0)
	game.OnEngineStart()
}

func onUpdate(delta float32) {
	cacheTriggerPairs()
	game.OnEngineUpdate(delta)
	handleEngineCoroutines()
}

func onDestroy() {
	game.OnEngineDestroy()
}

func cacheTriggerPairs() {
	mu.Lock()
	TriggerPairs = append(TriggerPairs, tempTriggerPairs...)
	mu.Unlock()
	tempTriggerPairs = tempTriggerPairs[:0]
}

func GetTriggerPairs(lst []TriggerPair) []TriggerPair {
	mu.Lock()
	lst = append(lst, TriggerPairs...)
	TriggerPairs = TriggerPairs[:0]
	mu.Unlock()
	return lst
}
