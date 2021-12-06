//go:build !canvas
// +build !canvas

package gdi

import (
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/goplus/spx/internal/gdi/text"
	"github.com/hajimehoshi/ebiten/v2"
)

type Font = font.Face

// -------------------------------------------------------------------------------------

// TextRender represents a text rendering engine.
//
type TextRender struct {
	*text.Render
}

// NewTextRender creates a text rendering engine.
//
func NewTextRender(face Font, width int, dy int) TextRender {
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
func DrawText(target *ebiten.Image, f Font, x, y int, text string, clr color.Color, mode int) {
	render := NewTextRender(f, 0x80000, 0)
	render.AddText(text)
	render.Draw(target, x, y, clr, mode)
}

// DrawLines draws multiline text.
//
func DrawLines(target *ebiten.Image, f Font, x, y int, width int, text string, clr color.Color, mode int) {
	render := NewTextRender(f, width, 0)
	render.AddText(text)
	render.Draw(target, x, y, clr, mode)
}

// -------------------------------------------------------------------------------------
