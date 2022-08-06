//go:build !canvas
// +build !canvas

package font

import (
	"fmt"
	"image"
	"io"
	"os"
	"path"
	"sync"

	"github.com/golang/freetype/truetype"
	"github.com/goplus/spx/fs/fsutil"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// -------------------------------------------------------------------------------------

type Default struct {
	ascii  font.Face
	songti font.Face
	done   chan error
	once   sync.Once
}

type Options = truetype.Options

func NewDefault(options *Options) *Default {
	p := &Default{done: make(chan error)}
	go p.init(options)
	return p
}

func (p *Default) Close() (err error) {
	if f := p.ascii; f != nil {
		f.Close()
	}
	if f := p.songti; f != nil {
		f.Close()
	}
	return nil
}

func (p *Default) ensureInited() {
	p.once.Do(func() {
		<-p.done
	})
}

type fontNameInit struct {
	paths  []string
	inited bool
}

func (p *Default) init(options *truetype.Options) {
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

func (p *Default) findFontAtPath(
	name string, findPath string, fontNames []string, options *truetype.Options) bool {
	for _, fontName := range fontNames {
		tryFile := path.Join(findPath, fontName)
		if p.tryFontFile(name, tryFile, options) {
			return true
		}
	}
	return false
}

// Each Default.(Font.Face) object holds all the content of the font file in memory,
// in fact the file content is read-only.
// Different Default-Objects with same file  should share the content,not allocate new memory.
var fontFileCache = struct {
	files map[string][]byte
	sync.Mutex
}{
	files: map[string][]byte{},
}

func (p *Default) tryFontFile(name, tryFile string, options *truetype.Options) bool {
	fontFileCache.Lock()
	var bs []byte
	var ok bool
	if bs, ok = fontFileCache.files[tryFile]; !ok {
		fp, err := fsutil.OpenFile(tryFile)
		if err != nil {
			fontFileCache.Unlock()
			return false
		}
		defer fp.Close()
		fi, err := os.Stat(tryFile)
		if err != nil {
			fontFileCache.Unlock()
			return false
		}
		bs = make([]byte, fi.Size())
		n, err := io.ReadFull(fp, bs)
		if err != nil || n != len(bs) {
			fontFileCache.Unlock()
			return false
		}
		fontFileCache.files[tryFile] = bs
	}
	fontFileCache.Unlock()

	tt, err := truetype.Parse(bs)
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

func (p *Default) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.Glyph(dot, r)
	}
	return p.songti.Glyph(dot, r)
}

func (p *Default) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.GlyphBounds(r)
	}
	return p.songti.GlyphBounds(r)
}

func (p *Default) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	p.ensureInited()
	if r < 0x100 {
		return p.ascii.GlyphAdvance(r)
	}
	return p.songti.GlyphAdvance(r)
}

func (p *Default) Kern(r0, r1 rune) fixed.Int26_6 {
	p.ensureInited()
	return p.ascii.Kern(r0, r1)
}

func (p *Default) Metrics() font.Metrics {
	p.ensureInited()
	return p.ascii.Metrics()
}

// -------------------------------------------------------------------------------------
