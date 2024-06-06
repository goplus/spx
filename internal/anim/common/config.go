package common

import (
	"github.com/goplus/spx/internal/math32"
)

type AnimatorConfig struct {
	Name        string           `json:"name"`
	Prefab      string           `json:"prefab"`
	Image       string           `json:"image"`
	Scale       math32.Vector2   `json:"scale"`
	Offset      math32.Vector2   `json:"offset"`
	DefaultClip string           `json:"defaultClip"`
	Clips       []AnimClipConfig `json:"clips"`
	Type        string           `json:"type"`
}

type AnimClipConfig struct {
	Name  string  `json:"name"`
	Loop  bool    `json:"loop"`
	Speed float64 `json:"speed"`
	Path  string  `json:"path"`
}
