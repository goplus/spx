package gdi

import (
	"bytes"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/goplus/spx/internal/gdi/text"
	"github.com/hajimehoshi/ebiten/v2"

	svgo "github.com/ajstarks/svgo"
	svg "github.com/goplus/spx/internal/svgr"
)

// -------------------------------------------------------------------------------------

// TextRender represents a text rendering engine.
//
type TextRender struct {
	*text.Render
}

// NewTextRender creates a text rendering engine.
//
func NewTextRender(face font.Face, width int, dy int) TextRender {
	r := text.NewRender(face, fixed.I(width), fixed.I(dy))
	return TextRender{r}
}

// Size returns width and height of rendered text.
//
func (p TextRender) Size() (int, int) {
	w, h := p.Render.Size()
	return w.Ceil(), h.Ceil()
}

// Draw draws rendered text.
//
func (p TextRender) Draw(target *ebiten.Image, x, y int, clr color.Color, mode int) {
	p.Render.Draw(target, fixed.I(x), fixed.I(y), clr, mode)
}

// DrawText draws input text.
//
func DrawText(target *ebiten.Image, f font.Face, x, y int, text string, clr color.Color, mode int) {
	render := NewTextRender(f, 0x80000, 0)
	render.AddText(text)
	render.Draw(target, x, y, clr, mode)
}

// DrawLines draws multiline text.
//
func DrawLines(target *ebiten.Image, f font.Face, x, y int, width int, text string, clr color.Color, mode int) {
	render := NewTextRender(f, width, 0)
	render.AddText(text)
	render.Draw(target, x, y, clr, mode)
}

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------
