package engine

import (
	. "github.com/goplus/spbase/mathf"
)

func TweenPos(node ISpriter, pos Vec2, duration float64, callback func()) {
	requireRuntimeBridge().TweenPos(node, pos, duration, callback)
}

func TweenPos2(node ISpriter, pos Vec2, duration float64, pos2 Vec2, duration2 float64, callback func()) {
	requireRuntimeBridge().TweenPos2(node, pos, duration, pos2, duration2, callback)
}
