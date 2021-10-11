package spx

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/goplus/spx/internal/gdi"
	"golang.org/x/image/font"
)

var (
	defaultFont   font.Face
	defaultFont2  font.Face
	defaultFontSm font.Face
)

func init() {
	const dpi = 72
	defaultFont = gdi.NewDefaultFont(&gdi.FontOptions{
		Size:    15,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	defaultFont2 = gdi.NewDefaultFont(&gdi.FontOptions{ // for stageMonitor
		Size:    12,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	defaultFontSm = gdi.NewDefaultFont(&gdi.FontOptions{
		Size:    11,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

// -------------------------------------------------------------------------------------

const (
	styleSay   = 1
	styleThink = 2
)

type sayOrThinker struct {
	sp    *Sprite
	msg   string
	style int // styleSay, styleThink
}

const (
	sayCornerSize = 8
	screenGap     = 4
	leadingWidth  = 15
	gapWidth      = 40
	trackDx       = 5
	trackCx       = gapWidth + trackDx
	trackCy       = 17
	minWidth      = leadingWidth + leadingWidth + gapWidth
)

func (p *sayOrThinker) draw(dc drawContext) {
	var direction int
	var glyphTpl string
	topx, topy := p.sp.getTrackPos()

	render := gdi.NewTextRender(defaultFont, 135, 2)
	render.AddText(p.msg)
	w, h := render.Size()
	x, y := topx+2, topy-h-(trackCy+24)

	pad := 9
	w += (pad << 1)
	h += (pad << 1)

	if w < minWidth {
		w = minWidth
	}

	screenW := p.sp.g.getWidth()
	if x < screenGap {
		x = screenGap
	} else if (x + w + screenGap) > screenW {
		x = topx - w - 2
		direction = 1
	}
	if y < screenGap {
		y = screenGap
	}

	varTable := []string{
		"$x", strconv.Itoa(x),
		"$y8", strconv.Itoa(y + sayCornerSize),
		"$w100", strconv.Itoa(w - (leadingWidth + gapWidth + sayCornerSize)),
		"$w8", strconv.Itoa(w - sayCornerSize*2),
		"$h8", strconv.Itoa(h - sayCornerSize*2),
		"$dx", strconv.Itoa(trackDx),
		"$trx", strconv.Itoa(trackCx),
		"$try", strconv.Itoa(trackCy),
	}
	varRepl := strings.NewReplacer(varTable...)

	if direction > 0 {
		glyphTpl = "M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -7 l $dx $try l -$trx -$try h -$w100 s -8 0 -8 -8 z"
	} else {
		glyphTpl = "M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -$w100 l -$trx $try l $dx -$try h -7 s -8 0 -8 -8 z"
	}
	glyph := varRepl.Replace(glyphTpl)

	canvas := gdi.Start(dc.Image)
	canvas.Path(glyph, "fill:white;stroke-width:3;stroke:rgb(148, 148, 148)")
	canvas.End()

	render.Draw(dc.Image, x+pad, y+pad, color.Black, 0)
}

func (p *sayOrThinker) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}

// -------------------------------------------------------------------------------------
