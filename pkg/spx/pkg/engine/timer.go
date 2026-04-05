package engine

func DelayCall(delay float64, callback func()) {
	requireRuntimeBridge().DelayCall(delay, callback)
}

func DelaySpriteCall(delay float64, sprite ISpriter, callback func()) {
	requireRuntimeBridge().DelaySpriteCall(delay, sprite, callback)
}
