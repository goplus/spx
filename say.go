/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"golang.org/x/image/font"

	"github.com/goplus/spx/internal/gdi"
	xfont "github.com/goplus/spx/internal/gdi/font"
)

var (
	defaultFont   gdi.Font
	defaultFont2  gdi.Font
	defaultFontSm gdi.Font
)

func init() {
	const dpi = 72
	defaultFont = xfont.NewDefault(&xfont.Options{
		Size:    15,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	defaultFont2 = xfont.NewDefault(&xfont.Options{ // for stageMonitor
		Size:    12,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	defaultFontSm = xfont.NewDefault(&xfont.Options{
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
	sp    *SpriteImpl
	msg   string
	style int // styleSay, styleThink
}

const (
	sayCornerSize = 8
	thinkRadius   = 5
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
		"$c1x", strconv.Itoa(x + w*2/5),
		"$c1rx", strconv.Itoa(x + w*3/5),
		"$c1y", strconv.Itoa(y + h + thinkRadius + 4),
		"$c2x", strconv.Itoa(x + w*2/5 - 2*thinkRadius - 2),
		"$c2rx", strconv.Itoa(x + w*3/5 + 2*thinkRadius + 2),
		"$c2y", strconv.Itoa(y + h + 2*thinkRadius + 6),
		"$c3x", strconv.Itoa(x + w*2/5 - 4*thinkRadius - 3),
		"$c3rx", strconv.Itoa(x + w*3/5 + 4*thinkRadius + 3),
		"$c3y", strconv.Itoa(y + h + 3*thinkRadius + 6),
	}
	varRepl := strings.NewReplacer(varTable...)

	if direction > 0 {
		if p.style == styleSay {
			glyphTpl = "M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -7 l $dx $try l -$trx -$try h -$w100 s -8 0 -8 -8 z"
		} else {
			glyphTpl = `M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -$w8 s -8 0 -8 -8 z
			M $c1rx, $c1y
			m -5, 0
			a 5,5 0 1,0 10,0
			a 5,5 0 1,0 -10,0
			M $c2rx, $c2y
			m -4, 0
			a 4,4 0 1,0 8,0
			a 4,4 0 1,0 -8,0
			M $c3rx, $c3y
			m -3, 0
			a 3,3 0 1,0 6,0
			a 3,3 0 1,0 -6,0 z`
		}
	} else {
		if p.style == styleSay {
			glyphTpl = "M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -$w100 l -$trx $try l $dx -$try h -7 s -8 0 -8 -8 z"
		} else {
			glyphTpl = `M $x $y8 s 0 -8 8 -8 h $w8 s 8 0 8 8 v $h8 s 0 8 -8 8 h -$w8 s -8 0 -8 -8 z
			M $c1x, $c1y
			m -5, 0
			a 5,5 0 1,0 10,0
			a 5,5 0 1,0 -10,0
			M $c2x, $c2y
			m -4, 0
			a 4,4 0 1,0 8,0
			a 4,4 0 1,0 -8,0
			M $c3x, $c3y
			m -3, 0
			a 3,3 0 1,0 6,0
			a 3,3 0 1,0 -6,0 z`
		}
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

func (p *SpriteImpl) sayOrThink(msgv interface{}, style int) {
	msg, ok := msgv.(string)
	if !ok {
		msg = fmt.Sprint(msgv)
	}

	if msg == "" {
		p.doStopSay()
		return
	}

	old := p.sayObj
	if old == nil {
		p.sayObj = &sayOrThinker{sp: p, msg: msg, style: style}
		p.g.addShape(p.sayObj)
	} else {
		old.msg, old.style = msg, style
		p.g.activateShape(old)
	}
}

func (p *SpriteImpl) waitStopSay(secs float64) {
	p.g.Wait(secs)
	p.doStopSay()
}

func (p *SpriteImpl) doStopSay() {
	if p.sayObj != nil {
		p.g.removeShape(p.sayObj)
		p.sayObj = nil
	}
}

// -------------------------------------------------------------------------------------
