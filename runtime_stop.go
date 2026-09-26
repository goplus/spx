package spx

import (
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
)

// Stop stops scripts selected by kind.
// Explicit receivers support interpreted direct calls.
func (p *Game) Stop(kind StopKind) {
	p.scriptEventBindings.Stop(kind)
}

// Stop stops scripts selected by kind.
func (p *SpriteImpl) Stop(kind StopKind) {
	p.scriptEventBindings.Stop(kind)
}

func (p *scriptEventBindings) Stop(kind StopKind) {
	// Preserve nil-receiver behavior for ThisScript.
	owner := p.owner
	if kind == ThisScript {
		// Procedure handles this signal without stopping its caller.
		gco.StopThisScript()
		return
	}
	filter, stopCaller := coreevent.ResolveStop(
		kind,
		owner,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter == nil {
		return
	}

	var caller coroutine.Thread
	if gco.IsInCoroutine() {
		caller = gco.Current()
	}
	shouldStop := func(thread coroutine.Thread) bool {
		return filter(thread.Obj, thread == caller)
	}

	if kind == AllStop {
		p.scriptEventRegistry.stopAllEpoch.Add(1)
		gco.StopIf(func(thread coroutine.Thread) bool {
			// Keep the caller for cleanup; pending starts use the new epoch.
			return thread != caller && !p.scriptEventRegistry.isPendingStartThread(thread) && shouldStop(thread)
		})
		if game := p.scriptEventRegistry.game; game != nil {
			game.stopAllResources()
		}
	} else {
		gco.StopIf(shouldStop)
	}
	if stopCaller {
		gco.StopCurrent()
	}
}

// stopAllResources removes clones and resets effects and audio.
// The caller must remain alive for engine calls during cleanup.
func (p *Game) stopAllResources() {
	p.baseObj.clearGraphicEffects()
	p.soundMgr.StopAll()
	p.clearSoundEffects(p.audioState.SoundObj)
	// Destroying clones removes shapes from the live list.
	for _, shape := range p.shapeMgr.getTempShapes() {
		sprite, ok := shape.(*SpriteImpl)
		if !ok {
			continue
		}
		sprite.clearGraphicEffects()
		if sprite.IsCloned() {
			sprite.destroy()
		} else if sound := sprite.components.sound; sound != nil {
			p.clearSoundEffects(sound.soundObj)
		}
	}
}
