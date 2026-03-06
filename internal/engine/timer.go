package engine

import gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"

type delaySpriteCallInfo struct {
	timer    float64
	objectID Object
	callback func()
}

var (
	delaySpriteCalls     = make([]*delaySpriteCallInfo, 0)
	tempDelaySpriteCalls = make([]*delaySpriteCallInfo, 0)
)

func updateTimers(delta float64) {
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
	count := len(delaySpriteCalls)
	tempDelaySpriteCalls = append(tempDelaySpriteCalls, delaySpriteCalls...)
	delaySpriteCalls = delaySpriteCalls[:0]
	for i := range count {
		tempDelaySpriteCalls[i].timer -= delta
		if tempDelaySpriteCalls[i].timer > 0 {
			delaySpriteCalls = append(delaySpriteCalls, tempDelaySpriteCalls[i])
		}
	}
	for i := range count {
		if tempDelaySpriteCalls[i].timer <= 0 {
			id := tempDelaySpriteCalls[i].objectID
			if id == 0 || isNodeExist(id) {
				tempDelaySpriteCalls[i].callback()
			}
		}
	}
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
}

func delayCall(delay float64, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{timer: delay, callback: callback})
}

func delaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{
		timer:    delay,
		objectID: sprite.GetId(),
		callback: callback,
	})
}
