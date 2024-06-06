package common

import (
	"github.com/goplus/spx/internal/math32"
)

type AnimatorConfig struct {
	Name        string           `json:"Name"`
	Prefab      string           `json:"Prefab"`
	Image       string           `json:"Image"`
	Scale       math32.Vector2   `json:"Scale"`
	Offset      math32.Vector2   `json:"Offset"`
	DefaultClip string           `json:"DefaultClip"`
	Clips       []AnimClipConfig `json:"Clips"`
	Type        string           `json:"Type"`
}

type AnimClipConfig struct {
	Name  string  `json:"Name"`
	Loop  bool    `json:"Loop"`
	Speed float64 `json:"Speed"`
	Path  string  `json:"Path"`
}
