package ui

import (
	. "godot-ext/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
)

var (
	WinX float64
	WinY float64
)

func PosGame2UI(x, y float64) Vec2 {
	return engine.NewVec2(x+WinX/2, (-y + WinY/2))
}
