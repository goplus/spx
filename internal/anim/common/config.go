package common

import (
	"github.com/goplus/spx/internal/math32"
)

type AvatarConfig struct {
	Image    string         `json:"image"`
	Mesh     string         `json:"mesh"`
	Scale    math32.Vector2 `json:"scale"`
	Offset   math32.Vector2 `json:"offset"`
	UvOffset math32.Vector2 `json:"uvOffset"`
}

type AnimatorConfig struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	DefaultClip string           `json:"defaultClip"`
	Clips       []AnimClipConfig `json:"clips"`
	// avatar
}

type AnimClipConfig struct {
	Name      string `json:"name"`
	Loop      bool   `json:"loop"`
	FrameRate int    `json:"frameRate"`
	Path      string `json:"path"`
}
