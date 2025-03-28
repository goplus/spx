package engine

type delaySpriteCallInfo struct {
	timer    float64
	objectId Object
	callback func()
}

var (
	delaySpriteCalls     = make([]*delaySpriteCallInfo, 0)
	tempDelaySpriteCalls = make([]*delaySpriteCallInfo, 0)
)

func updateTimers(delta float64) {
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
	count := len(delaySpriteCalls)
	for i := 0; i < count; i++ {
		tempDelaySpriteCalls = append(tempDelaySpriteCalls, delaySpriteCalls[i])
	}
	delaySpriteCalls = delaySpriteCalls[:0]
	for i := 0; i < count; i++ {
		tempDelaySpriteCalls[i].timer -= delta
		if tempDelaySpriteCalls[i].timer > 0 {
			delaySpriteCalls = append(delaySpriteCalls, tempDelaySpriteCalls[i])
		}
	}
	for i := 0; i < count; i++ {
		if tempDelaySpriteCalls[i].timer <= 0 {
			id := tempDelaySpriteCalls[i].objectId
			if id == 0 {
				tempDelaySpriteCalls[i].callback()
			} else if isNodeExist(id) {
				// ensure the sprite is still alive
				tempDelaySpriteCalls[i].callback()
			}
		}
	}
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
}

func DelayCall(delay float64, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{delay, 0, callback})
}

func DealySpriteCall(delay float64, sprite ISpriter, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{delay, sprite.GetId(), callback})
}
