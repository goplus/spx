//go:build !canvas
// +build !canvas

package font

import (
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"path"
	"sync"

	"github.com/goplus/spx/fs/fsutil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// -------------------------------------------------------------------------------------

const (
	fontTimesNewRoman = "Times New Roman"
	fontSimSun        = "SimSun"
)

type fontInfo struct {
	font   *opentype.Font
	closer io.Closer
}

type fontCache struct {
	cache map[string]*fontInfo
	once  sync.Once
}

var (
	cache fontCache
)

func (p *fontCache) clear() {
	for k, info := range p.cache {
		if info.closer != nil {
			info.closer.Close()
		}
		delete(p.cache, k)
	}
}

func (p *fontCache) init() {
	p.once.Do(p._init)
}

func (p *fontCache) _init() {
	fontFaceNames := map[string]*fontNameInit{
		fontTimesNewRoman: {paths: []string{"Times New Roman Bold.ttf", "Times New Roman.ttf", "Times.ttf"}},
		fontSimSun:        {paths: []string{"SimSun.ttf", "SimSun.ttc", "Songti.ttc"}},
	}
	p.cache = make(map[string]*fontInfo)
	for _, findPath := range fontFindPaths {
		for name, fontInit := range fontFaceNames {
			if !fontInit.inited {
				if p.findFontAtPath(name, findPath, fontInit.paths) {
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
}

func (p *fontCache) findFontAtPath(
	name string, findPath string, fontNames []string) bool {
	for _, fontName := range fontNames {
		tryFile := path.Join(findPath, fontName)
		if fnt, err := p.loadFile(tryFile); err == nil {
			p.cache[name] = fnt
			return true
		}
	}
	return false
}

func parseFont(r io.ReadSeekCloser, keepUse *bool) (*sfnt.Font, error) {
	if reader, ok := r.(io.ReaderAt); ok {
		s, err := opentype.ParseCollectionReaderAt(reader)
		if err != nil {
			return nil, err
		}
		f, err := s.Font(0)
		if err == nil {
			*keepUse = true
		}
		return f, err
	} else {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		s, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, err
		}
		return s.Font(0)
	}
}

func (p *fontCache) loadFile(file string) (*fontInfo, error) {
	r, err := fsutil.OpenFile(file)
	if err != nil {
		return nil, err
	}
	var keepUse bool
	defer func() {
		if !keepUse {
			r.Close()
		}
	}()
	font, err := parseFont(r, &keepUse)
	if err != nil {
		return nil, err
	}
	info := &fontInfo{font: font}
	if keepUse {
		info.closer = r
	}
	return info, nil
}

type Default struct {
	ascii  font.Face
	songti font.Face
	done   chan error
	once   sync.Once
}

type Options = opentype.FaceOptions

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

func (p *Default) init(options *Options) {
	cache.init()
	for name, info := range cache.cache {
		f, _ := opentype.NewFace(info.font, options)
		switch name {
		case "Times New Roman":
			p.ascii = f
		case "SimSun":
			p.songti = f
		}
	}
	p.done <- nil
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
