//go:build canvas
// +build canvas

package font

import (
	"golang.org/x/image/font"
)

type Font interface {
	Family() string
	PointSize() int
}

type Options struct {
	Size    float64
	DPI     float64
	Hinting font.Hinting
}

func NewDefault(options *Options) Font {
	return &Default{
		family: "Times New Roman,Times,serif",
		size:   int(options.Size * options.DPI / 72),
	}
}

type Default struct {
	family string
	size   int
}

func (font Default) Family() string {
	return font.family
}

func (font Default) PointSize() int {
	return font.size
}
