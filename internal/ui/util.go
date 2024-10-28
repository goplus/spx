package ui

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
)

// convert world space position to screen space
func WorldToScreen(x, y float64) Vec2 {
	viewport := engine.SyncCameraGetViewportRect()
	winX := float64(viewport.Size.X)
	winY := float64(viewport.Size.Y)
	return engine.NewVec2(x+winX/2-float64(viewport.Position.X), (-y+winY/2)-float64(viewport.Position.Y))
}
