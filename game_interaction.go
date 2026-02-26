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
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/ui"
)

// ============================================================================
// Ask/Answer - User Input Dialog
// ============================================================================

func (p *Game) Ask(msg any) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		spxlog.Warn("ask: msg should not be empty")
		return
	}
	p.ask(false, msgStr, func(answer string) {})
}

func (p *Game) Answer() string {
	return p.answerVal
}

func (p *Game) ask(isSprite bool, question string, callback func(string)) {
	if p.askPanel == nil {
		p.askPanel = ui.NewUiAsk()
		p.addShape(p.askPanel)
	}
	hasAnswer := false
	p.askPanel.Show(isSprite, question, func(msg string) {
		p.answerVal = msg
		callback(msg)
		hasAnswer = true
	})
	for {
		if hasAnswer {
			break
		}
		p.askPanel.Update()
		engine.WaitNextFrame()
	}
}

// ============================================================================
// Graphic Effects
// ============================================================================

type EffectKind int

const (
	ColorEffect EffectKind = iota
	FishEyeEffect
	WhirlEffect
	PixelateEffect
	MosaicEffect
	BrightnessEffect
	GhostEffect

	enumNumOfEffect // max index of enum
)

var greffNames = []string{
	ColorEffect:      "color_amount",
	FishEyeEffect:    "fisheye_amount",
	WhirlEffect:      "whirl_amount",
	MosaicEffect:     "uv_amount",
	PixelateEffect:   "pixleate_amount",
	BrightnessEffect: "brightness_amount",
	GhostEffect:      "alpha_amount",
}

func (kind EffectKind) String() string {
	return greffNames[kind]
}

func (p *Game) SetGraphicEffect(kind EffectKind, val float64) {
	p.baseObj.setGraphicEffect(kind, val)
}

func (p *Game) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.baseObj.changeGraphicEffect(kind, delta)
}

func (p *Game) ClearGraphicEffects() {
	p.baseObj.clearGraphicEffects()
}

// ============================================================================
// Broadcasting - Event Messages
// ============================================================================

func (p *Game) doBroadcast(msg string, data any, wait bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Broadcast: msg=%s, wait=%v", msg, wait)
	}
	p.sinkMgr.doWhenIReceive(msg, data, wait)
}

func (p *Game) Broadcast__0(msg string) {
	p.doBroadcast(msg, nil, false)
}

func (p *Game) Broadcast__1(msg string, data any) {
	p.doBroadcast(msg, data, false)
}

func (p *Game) BroadcastAndWait__0(msg string) {
	p.doBroadcast(msg, nil, true)
}

func (p *Game) BroadcastAndWait__1(msg string, data any) {
	p.doBroadcast(msg, data, true)
}

// ============================================================================
// Variable Display - Monitor Visibility
// ============================================================================

func (p *Game) setStageMonitor(target string, val string, visible bool) {
	for _, item := range p.spriteMgr.items {
		if sp, ok := item.(*Monitor); ok && sp.val == val && sp.target == target {
			sp.setVisible(visible)
			return
		}
	}
}

func (p *Game) HideVar(name string) {
	p.setStageMonitor("", getVarPrefix+name, false)
}

func (p *Game) ShowVar(name string) {
	p.setStageMonitor("", getVarPrefix+name, true)
}
