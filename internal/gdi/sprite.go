package gdi

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
)

// -------------------------------------------------------------------------------------

// Sprite type.
type Sprite image.RGBA

// NewSprite func.
func NewSprite(screen *ebiten.Image, rect image.Rectangle) *Sprite {
	data := image.NewRGBA(rect)
	for j := rect.Min.Y; j < rect.Max.Y; j++ {
		for i := rect.Min.X; i < rect.Max.X; i++ {
			clr := screen.At(i, j)
			data.Set(i, j, clr)
		}
	}
	return (*Sprite)(data)
}

// NewSpriteFromRect func.
func NewSpriteFromRect(x1, y1, x2, y2 int) *Sprite {
	return NewSpriteFromRectangle(image.Rect(x1, y1, x2, y2))
}

// NewSpriteFromRectangle func.
func NewSpriteFromRectangle(rect image.Rectangle) *Sprite {
	data := image.NewRGBA(rect)
	pix := data.Pix
	for i := range pix {
		pix[i] = 0xff
	}
	return (*Sprite)(data)
}

// NewSpriteFromScreen func.
func NewSpriteFromScreen(screen *ebiten.Image) *Sprite {
	var bx, by, ex, ey int
	var w, h = screen.Size()

	for by = 0; by < h; by++ {
		for i := 0; i < w; i++ {
			_, _, _, a := screen.At(i, by).RGBA()
			if a > 0 {
				goto lzNext1
			}
		}
	}
lzNext1:
	for ey = h - 1; ey > by; ey-- {
		for i := 0; i < w; i++ {
			_, _, _, a := screen.At(i, ey).RGBA()
			if a > 0 {
				goto lzNext2
			}
		}
	}
lzNext2:
	for bx = 0; bx < w; bx++ {
		for j := 0; j < h; j++ {
			_, _, _, a := screen.At(bx, j).RGBA()
			if a > 0 {
				goto lzNext3
			}
		}
	}
lzNext3:
	for ex = w - 1; ex > bx; ex-- {
		for j := 0; j < h; j++ {
			_, _, _, a := screen.At(ex, j).RGBA()
			if a > 0 {
				goto lzNext4
			}
		}
	}
lzNext4:
	return NewSprite(screen, image.Rect(bx, by, ex+1, ey+1))
}

// -------------------------------------------------------------------------------------

// TouchingColor func.
func TouchingColor(sp1 *Sprite, pt1 image.Point, sp2 *Sprite, pt2 image.Point, clr color.RGBA) bool {
	if sp1 == nil || sp2 == nil {
		return false
	}
	panic("todo")
}

// -------------------------------------------------------------------------------------

// Touching func.
func Touching(sp1 *Sprite, pt1 image.Point, sp2 *Sprite, pt2 image.Point) bool {
	if sp1 == nil || sp2 == nil {
		return false
	}

	src1 := (*image.RGBA)(sp1)
	src2 := (*image.RGBA)(sp2)

	rect1 := src1.Rect.Add(pt1)
	rect2 := src2.Rect.Add(pt2)
	rectd := rect1.Intersect(rect2)
	if rectd.Empty() {
		return false
	}

	dst := image.NewRGBA(rectd)
	draw.DrawMask(dst, rectd, src1, rectd.Min.Sub(pt1), src2, rectd.Min.Sub(pt2), draw.Src)
	return rgbaNotNil(dst.Pix)
}

// TouchingPoint func.
func TouchingPoint(sp *Sprite, pt image.Point, x, y int) bool {
	return TouchingRect(sp, pt, x, y, x+1, y+1)
}

// TouchingRect func.
func TouchingRect(sp *Sprite, pt image.Point, x1, y1, x2, y2 int) bool {
	return TouchingRectangle(sp, pt, image.Rect(x1, y1, x2, y2))
}

// TouchingRectangle func.
func TouchingRectangle(sp1 *Sprite, pt1 image.Point, rect2 image.Rectangle) bool {
	src1 := (*image.RGBA)(sp1)
	rect1 := src1.Rect.Add(pt1)
	rectd := rect1.Intersect(rect2)
	if rectd.Empty() {
		return false
	}

	dst := image.NewRGBA(rectd)
	draw.Draw(dst, rectd, src1, rectd.Min.Sub(pt1), draw.Src)
	return rgbaNotNil(dst.Pix)
}

func rgbaNotNil(pix []uint8) bool {
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0 {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------------------------

// Image func.
func (p *Sprite) Image() *image.RGBA {
	return (*image.RGBA)(p)
}

// GetTrackPos func.
func (p *Sprite) GetTrackPos() image.Point {
	off := rgbaFirstPix(p.Pix)
	if off < 0 {
		return p.Rect.Min
	}
	w := p.Rect.Dx()
	return p.Rect.Min.Add(image.Pt(off%w, off/w))
}

func rgbaFirstPix(pix []uint8) (off int) {
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0 {
			return
		}
		off++
	}
	return -1
}

// -------------------------------------------------------------------------------------
