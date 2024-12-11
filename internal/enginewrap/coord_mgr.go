package enginewrap

import (
	. "github.com/realdream-ai/mathf"
)

// --------------- camera ------------------
func (pself *cameraMgrImpl) GetLocalPosition(pos Vec2) Vec2 {
	camPos := pself.GetCameraPosition()
	return pos.Sub(camPos)
}
func (pself *cameraMgrImpl) GetPosition() Vec2 {
	pos := pself.GetCameraPosition()
	return NewVec2(pos.X, -pos.Y)
}

func (pself *cameraMgrImpl) SetPosition(position Vec2) {
	pself.SetCameraPosition(NewVec2(position.X, -position.Y))
}
