package common

import "github.com/goplus/spx/internal/math32"

type AnimClipState struct {
	AnimClipConfig
	FrameCount int
	Speed      float64
	Time       float64
}

func (pself *AnimClipState) GetLength(pos math32.Vector2) float64 {
	return float64(pself.FrameCount) / float64(pself.FrameRate)
}

func (pself *AnimClipState) GetCurFrame() int {
	frame := (int)(pself.Time * float64(pself.FrameRate))
	if frame >= pself.FrameCount {
		if pself.Loop {
			frame = frame % pself.FrameCount
		} else {
			frame = pself.FrameCount - 1
		}
	}
	return frame
}
