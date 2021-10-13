//go:build canvas
// +build canvas

package gdi

import (
	"bytes"
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	svgo "github.com/ajstarks/svgo"
	"github.com/goplus/canvas"
	svg "github.com/goplus/spx/internal/svgr"
	"github.com/hajimehoshi/ebiten/v2"
)

// -------------------------------------------------------------------------------------

// TextRender represents a text rendering engine.
//
type TextRender struct {
	fnt      *canvas.Font
	tm       *TextMetrics
	img      *ebiten.Image
	maxWidth int
	width    int
	height   int
	dy       int
	dirty    bool
	text     string
	lines    []string
}

// NewTextRender creates a text rendering engine.
//
func NewTextRender(face Font, width int, dy int) TextRender {
	fnt := canvas.NewFont(face.Family(), face.PointSize())
	return TextRender{
		fnt:      fnt,
		tm:       NewTextMetrics(fnt),
		maxWidth: width,
		dy:       dy,
	}
}

// Size returns width and height of rendered text.
//
func (p TextRender) Size() (int, int) {
	return p.width, p.height
}

// Draw draws rendered text.
//
func (p TextRender) Draw(target *ebiten.Image, x, y int, clr color.Color, mode int) {
	if p.dirty {
		ctx := canvas.NewContext2D(p.width, p.height)
		ctx.SetFont(p.fnt)
		ctx.SetFillColor(clr)
		ctx.SetTextBaseline(canvas.AlignTop)
		for i, line := range p.lines {
			ctx.FillText(line, 0, float64(i*(p.fnt.PointSize+p.dy)))
		}
		if p.img != nil {
			p.img.Dispose()
		}
		p.img = ebiten.NewImageFromImage(ctx.Image())
		p.dirty = false
	}
	opt := &ebiten.DrawImageOptions{}
	opt.GeoM.Translate(float64(x), float64(y))
	target.DrawImage(p.img, opt)
}

func (p *TextRender) AddText(s string) {
	p.text += s
	p.lines = p.tm.ParseMultiLine(p.text, float64(p.maxWidth), true)
	p.dirty = true
	var width float64
	for _, s := range p.lines {
		w := p.tm.Metrics(s)
		if width < w {
			width = w
		}
	}
	p.width = int(width)
	p.height = (p.fnt.PointSize + p.dy) * len(p.lines)
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

type Line struct {
	text  string
	width int
}

type TextMetrics struct {
	ctx   canvas.Context2D
	cache map[rune]float64
}

func NewTextMetrics(font *canvas.Font) *TextMetrics {
	ctx := canvas.NewContext2D(10, 10)
	ctx.SetFont(font)
	return &TextMetrics{ctx, make(map[rune]float64)}
}

func (t *TextMetrics) Metrics(text string) float64 {
	return t.ctx.MeasureText(text)
}

func (t *TextMetrics) ParseMultiLine(text string, maxWidth float64, autoBreak bool) (lines []string) {
	ar := strings.Split(text, "\n")
	for _, v := range ar {
		ll := t.PraseSingleLine(v, maxWidth, autoBreak)
		lines = append(lines, ll...)
	}
	return
}

func isWord(r rune) bool {
	return uint32(r) <= unicode.MaxLatin1 && unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.' || r == '\''
}

func (t *TextMetrics) PraseSingleLine(text string, maxWidth float64, autoBreak bool) (lines []string) {
	var chkWidth float64
	var chkBreakWidth float64
	var chkBreakRune rune
	_ = chkBreakRune
	chkBreak := -1
	chkLast := 0
	for i, r := range text {
		if !unicode.IsPrint(r) {
			continue
		}
		w, ok := t.cache[r]
		if !ok {
			w = t.ctx.MeasureText(string(r))
			t.cache[r] = w
		}
		chkWidth += w
		if chkWidth > maxWidth {
			prev := i
			width := w
			if autoBreak && isWord(r) && chkBreak != -1 {
				prev = chkBreak + utf8.RuneLen(chkBreakRune)
				width = chkWidth - chkBreakWidth
			}
			lines = append(lines, text[chkLast:prev])
			chkLast = prev
			chkWidth = width
			chkBreak = -1
		}
		if autoBreak && !isWord(r) {
			chkBreak = i
			chkBreakRune = r
			chkBreakWidth = chkWidth
		}
	}
	lines = append(lines, text[chkLast:])
	return
}
