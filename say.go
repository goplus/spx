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

	"github.com/goplus/spx/internal/ui"
)

func init() {
	const dpi = 72

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
	panel *ui.UiSay
}

func (p *sayOrThinker) refresh() {
	size := p.sp.Bounds().Size
	x, y := p.sp.x, p.sp.y
	applyRenderOffset(p.sp, &x, &y)
	p.panel.SetText(x, y, size.Width, size.Height, p.msg)
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
		p.sayObj.panel = ui.NewUiSay()
	} else {
		old.msg, old.style = msg, style
		p.g.activateShape(old)
	}
	p.sayObj.refresh()
}

func (p *SpriteImpl) waitStopSay(secs float64) {
	p.g.Wait(secs)
	p.doStopSay()
}

func (p *SpriteImpl) doStopSay() {
	if p.sayObj != nil {
		p.sayObj.panel.Destroy()
		p.sayObj.panel = nil
		p.g.removeShape(p.sayObj)
		p.sayObj = nil
	}
}

// -------------------------------------------------------------------------------------
