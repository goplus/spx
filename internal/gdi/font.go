//go:build !canvas
// +build !canvas

package gdi

import (
	"fmt"
	"image"
	"io/ioutil"
	"path"
	"sync"

	"github.com/golang/freetype/truetype"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// -------------------------------------------------------------------------------------

type Font = font.Face

type DefaultFont struct {
	ascii  font.Face
	songti font.Face
	done   chan error
	once   sync.Once
}

type FontOptions = truetype.Options

func NewDefaultFont(options *FontOptions) *DefaultFont {
	p := &DefaultFont{done: make(chan error)}
	go p.init(options)
	return p
}

func (p *DefaultFont) Close() (err error) {
	if f := p.ascii; f != nil {
		f.Close()
	}
	if f := p.songti; f != nil {
		f.Close()
	}
	return nil
}

func (p *DefaultFont) ensureInited() {
	p.once.Do(func() {
		<-p.done
	})
}

type fontNameInit struct {
	paths  []string
	inited bool
}

func (p *DefaultFont) init(options *truetype.Options) {
	fontFaceNames := map[string]*fontNameInit{
		"Times New Roman": {paths: []string{"Times New Roman Bold.ttf", "Times New Roman.ttf", "Times.ttf"}},
		"SimSun":          {paths: []string{"SimSun.ttf", "SimSun.ttc", "Songti.ttc"}},
	}
	for _, findPath := range fontFindPaths {
		for name, fontInit := range fontFaceNames {
			if !fontInit.inited {
				if p.findFontAtPath(name, findPath, fontInit.paths, options) {
					fontInit.inited = true
				}
			}
		}
	}
	for name, fontInit := range fontFaceNames {
		if !fontInit.inited {
			panic(fmt.Sprintf("Font not found: %s (%v not in %v)", name, fontInit.paths, fontFindPaths))
		}
	}
	p.done <- nil
}

func (p *DefaultFont) findFontAtPath(
	name string, findPath string, fontNames []string, options *truetype.Options) bool {
	for _, fontName := range fontNames {
		tryFile := path.Join(findPath, fontName)
		if p.tryFontFile(name, tryFile, options) {
			return true
		}
	}
	return false
}

func (p *DefaultFont) tryFontFile(name, tryFile string, options *truetype.Options) bool {
	fp, err := ebitenutil.OpenFile(tryFile)
	if err != nil {
		return false
	}
	defer fp.Close()

	b, err := ioutil.ReadAll(fp)
	if err != nil {
		return false
	}

	tt, err := truetype.Parse(b)
	if err != nil {
		return false
	}

	f := truetype.NewFace(tt, options)
	switch name {
	case "Times New Roman":
		p.ascii = f
	case "SimSun":
		p.songti = f
	}
	return true
}

func (p *DefaultFont) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.Glyph(dot, r)
	}
	return p.songti.Glyph(dot, r)
}

func (p *DefaultFont) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.GlyphBounds(r)
	}
	return p.songti.GlyphBounds(r)
}

func (p *DefaultFont) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.GlyphAdvance(r)
	}
	return p.songti.GlyphAdvance(r)
}

func (p *DefaultFont) Kern(r0, r1 rune) fixed.Int26_6 {
	p.ensureInited()
	return p.ascii.Kern(r0, r1)
}

func (p *DefaultFont) Metrics() font.Metrics {
	p.ensureInited()
	return p.ascii.Metrics()
}

// -------------------------------------------------------------------------------------
