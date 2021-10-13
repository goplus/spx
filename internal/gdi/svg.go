package gdi

import (
	"bytes"
	"image"

	svgo "github.com/ajstarks/svgo"
	svg "github.com/goplus/spx/internal/svgr"
	"github.com/hajimehoshi/ebiten/v2"
)

// Canvas represents a gdi object.
//
type Canvas struct {
	*svgo.SVG
	Target *ebiten.Image
}

// Start creates a canvas object.
//
func Start(target *ebiten.Image) Canvas {
	w := new(bytes.Buffer)
	s := svgo.New(w)
	cx, cy := target.Size()
	s.Start(cx, cy)
	return Canvas{s, target}
}

// End draws canvas data onto the target image.
//
func (p Canvas) End() {
	p.SVG.End()
	img, err := svg.Decode(p.SVG.Writer.(*bytes.Buffer))
	if err != nil {
		panic(err)
	}
	img2 := ebiten.NewImageFromImage(img)
	defer img2.Dispose()
	p.Target.DrawImage(img2, nil)
}

type SVG struct {
	*svgo.SVG
}

func NewSVG(cx, cy int) SVG {
	w := new(bytes.Buffer)
	s := svgo.New(w)
	s.Start(cx, cy)
	return SVG{s}
}

func (p SVG) ToImage() (image.Image, error) {
	return svg.Decode(p.SVG.Writer.(*bytes.Buffer))
}
