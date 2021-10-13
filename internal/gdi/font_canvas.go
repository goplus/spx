//go:build canvas
// +build canvas

package gdi

import (
	"golang.org/x/image/font"
)

type Font interface {
	Family() string
	PointSize() int
}

type FontOptions struct {
	Size    float64
	DPI     float64
	Hinting font.Hinting
}

func NewDefaultFont(options *FontOptions) Font {
	return &DefaultFont{
		family: "Times New Roman,Times,serif",
		size:   int(options.Size * options.DPI / 72),
	}
}

type DefaultFont struct {
	family string
	size   int
}

func (font DefaultFont) Family() string {
	return font.family
}

func (font DefaultFont) PointSize() int {
	return font.size
}
