/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/ui"
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
	if p.panel == nil {
		return
	}
	bound := p.sp.Bounds()
	center := bound.Center()
	size := bound.Size
	p.panel.SetText(p.sp.g.getWindowSize(), center, size, p.msg)
}

// -------------------------------------------------------------------------------------

func (p *SpriteImpl) sayOrThink(msg any, style int) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		p.doStopSay()
		return
	}

	old := p.sayObj
	if old == nil {
		p.sayObj = &sayOrThinker{sp: p, msg: msgStr, style: style}
		p.g.addShape(p.sayObj)
		p.sayObj.panel = ui.NewUiSay()
	} else {
		old.msg, old.style = msgStr, style
		p.g.activateShape(old)
	}
	p.sayObj.refresh()
}

func (p *SpriteImpl) waitStopSay(secs float64) {
	engine.Wait(secs)
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
