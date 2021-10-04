/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package text

import (
	"image"
	"image/color"
	"image/draw"
	"sync"
	"unicode"

	"github.com/qiniu/x/ctype"

	"github.com/hajimehoshi/ebiten/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// -------------------------------------------------------------------------------------

type glyphCache struct {
	dr   image.Rectangle
	mask *ebiten.Image
}

type faceCache struct {
	font.Metrics
	advances map[rune]fixed.Int26_6
	glyphs   map[rune]*glyphCache
	kerns    map[rune]fixed.Int26_6
	mutex    sync.RWMutex
}

func newFaceCache(f font.Face) *faceCache {
	metrics := f.Metrics()
	return &faceCache{
		Metrics:  metrics,
		advances: make(map[rune]fixed.Int26_6),
		glyphs:   make(map[rune]*glyphCache),
		kerns:    make(map[rune]fixed.Int26_6),
	}
}

func (p *faceCache) glyph(f font.Face, c rune) (dr image.Rectangle, mask *ebiten.Image, ok bool) {
	p.mutex.RLock()
	g, ok := p.glyphs[c]
	p.mutex.RUnlock()

	if !ok {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		g, ok = p.glyphs[c]
		if !ok {
			dot0 := fixed.Point26_6{X: 0, Y: p.Ascent}
			if dr0, mask0, maskp0, _, ok0 := f.Glyph(dot0, c); ok0 {
				var img *ebiten.Image
				if dr0.Dx() > 0 { // maybe is <space> rune
					tmp := image.NewRGBA(dr0)
					draw.DrawMask(tmp, dr0, image.White, image.Point{}, mask0, maskp0, draw.Over)
					img = ebiten.NewImageFromImage(tmp)
				}
				g, ok = &glyphCache{dr: dr0, mask: img}, true
				p.glyphs[c] = g
			} else {
				return
			}
		}
	}
	dr = g.dr
	mask = g.mask
	return
}

func (p *faceCache) glyphAdvance(f font.Face, c rune) (advance fixed.Int26_6, ok bool) {
	p.mutex.RLock()
	advance, ok = p.advances[c]
	p.mutex.RUnlock()

	if !ok {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		advance, ok = p.advances[c]
		if !ok {
			if advance, ok = f.GlyphAdvance(c); ok {
				p.advances[c] = advance
			}
		}
	}
	return
}

func (p *faceCache) kern(f font.Face, r0, r1 rune) fixed.Int26_6 {
	if r0 > 256 || r1 > 256 || r0 <= 0 || r1 <= 0 {
		return 0
	}
	key := (r1 << 8) | r0
	p.mutex.RLock()
	kern, ok := p.kerns[key]
	p.mutex.RUnlock()
	if !ok {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		kern, ok = p.kerns[key]
		if !ok {
			kern = f.Kern(r0, r1)
			p.kerns[key] = kern
		}
	}
	return kern
}

var (
	faceCaches = make(map[font.Face]*faceCache)
	faceMutex  sync.RWMutex
)

func getFaceCache(f font.Face) *faceCache {
	faceMutex.RLock()
	cache, ok := faceCaches[f]
	faceMutex.RUnlock()

	if !ok {
		faceMutex.Lock()
		defer faceMutex.Unlock()
		cache, ok = faceCaches[f]
		if !ok {
			cache = newFaceCache(f)
			faceCaches[f] = cache
		}
	}
	return cache
}

// -------------------------------------------------------------------------------------

// CachedFace represents a cached face
//
type CachedFace struct {
	font.Face
	*faceCache
}

// NewFace creates a cached face object.
//
func NewFace(f font.Face) CachedFace {
	return CachedFace{f, getFaceCache(f)}
}

// -------------------------------------------------------------------------------------

// RenderGlyph represents a rendered text.
//
type RenderGlyph struct {
	X fixed.Int26_6
	C rune
}

// RenderLine represents a rendered line.
//
type RenderLine struct {
	Items []RenderGlyph
	LastX fixed.Int26_6
}

// Render represents a text rendering engine.
//
type Render struct {
	Face    CachedFace
	Width   fixed.Int26_6
	Height  fixed.Int26_6
	LastX   fixed.Int26_6
	Last    []RenderGlyph // last line
	NotLast []RenderLine  // all lines except last
	Prev    rune
}

// NewRender creates a new text rendering engine.
//
func NewRender(face font.Face, width, dy fixed.Int26_6) *Render {
	f := NewFace(face)
	return &Render{
		Face:   f,
		Width:  width,
		Height: f.Height + dy,
		LastX:  0,
	}
}

func isWord(c rune) bool {
	return ctype.Is(ctype.CSYMBOL_NEXT_CHAR, c)
}

func getWordIndex(line []RenderGlyph) int {
	for i := len(line) - 1; i >= 0; i-- {
		if !isWord(line[i].C) {
			return i + 1
		}
	}
	return 0
}

func getPunctSplit(line []RenderGlyph) int {
	n := len(line)
	c := line[n-1].C
	if unicode.IsPunct(c) {
		return 0
	}
	if isWord(c) {
		return getWordIndex(line[:n-1])
	}
	return n - 1
}

// AddText renders inputed text.
//
func (p *Render) AddText(s string) {
	f := p.Face
	for _, c := range s {
	lzRetry:
		advance, ok := f.glyphAdvance(f.Face, c)
		if !ok {
			// TODO: is falling back on the U+FFFD glyph the responsibility of
			// the Drawer or the Face?
			// TODO: set prevC = '\ufffd'?
			panic(string(c) + ": glyphAdvance not ok")
		}
		nextX := p.LastX + f.kern(f.Face, p.Prev, c) + advance
		if nextX > p.Width && !unicode.IsSpace(c) && len(p.Last) > 0 {
			var n = len(p.Last)
			var idx = n
			if n > 0 {
				if isWord(c) {
					idx = getWordIndex(p.Last)
				} else if unicode.IsPunct(c) {
					idx = getPunctSplit(p.Last)
				}
			}
			if idx == n {
				p.NotLast = append(p.NotLast, RenderLine{p.Last, p.LastX})
				p.LastX = 0
				p.Prev = 0
				p.Last = nil
				goto lzRetry
			} else if idx > 0 {
				gly := p.Last[idx]
				glyLast := p.Last[idx-1]
				advance, _ = f.glyphAdvance(f.Face, glyLast.C)
				p.NotLast = append(p.NotLast, RenderLine{p.Last[:idx], glyLast.X + advance})
				p.Prev = p.Last[n-1].C
				for i := idx; i < n; i++ {
					p.Last[i].X -= gly.X
				}
				nextX -= gly.X
				p.LastX -= gly.X
				p.Last = p.Last[idx:]
			}
		}
		p.Last = append(p.Last, RenderGlyph{X: p.LastX, C: c})
		p.LastX = nextX
		p.Prev = c
	}
}

func (p *Render) getWidth() fixed.Int26_6 {
	if len(p.NotLast) > 0 { // multi-lines
		return p.Width
	}
	if len(p.Last) > 0 {
		return p.LastX - p.Last[0].X
	}
	return 0
}

// Size returns width and height of rendered text.
//
func (p *Render) Size() (fixed.Int26_6, fixed.Int26_6) {
	h := p.Face.Descent + p.Face.Ascent + p.Height.Mul(fixed.I(len(p.NotLast)))
	return p.getWidth(), h
}

// Draw draws rendered text.
//
func (p *Render) Draw(dst *ebiten.Image, x, y fixed.Int26_6, clr color.Color, mode int) {
	for _, line := range p.NotLast {
		p.drawLine(dst, line.Items, x, y, clr, mode)
		y += p.Height
	}
	p.drawLine(dst, p.Last, x, y, clr, mode)
}

func (p *Render) drawLine(dst *ebiten.Image, line []RenderGlyph, x, y fixed.Int26_6, clr color.Color, mode int) {
	cr, cg, cb, ca := clr.RGBA()
	if ca == 0 {
		return
	}
	f := p.Face
	for _, item := range line {
		dr, mask, ok := f.glyph(f.Face, item.C)
		if !ok {
			panic(string(item.C) + ": glyph not ok")
		}
		if mask == nil {
			continue
		}
		options := new(ebiten.DrawImageOptions)
		x, y := dr.Min.X+(x+item.X).Round(), dr.Min.Y+y.Round()
		options.GeoM.Translate(float64(x), float64(y))
		rf := float64(cr) / float64(ca)
		gf := float64(cg) / float64(ca)
		bf := float64(cb) / float64(ca)
		af := float64(ca) / 0xffff
		options.ColorM.Scale(rf, gf, bf, af)
		dst.DrawImage(mask, options)
	}
}

// -------------------------------------------------------------------------------------
