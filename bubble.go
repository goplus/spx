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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/ui"
)

// -------------------------------------------------------------------------------------
// Common Bubble System - Shared functionality for Say/Think/Quote bubbles
// -------------------------------------------------------------------------------------

// bubbleBase provides common functionality for all bubble types.
type bubbleBase struct {
	sprite  *SpriteImpl
	camera  *cameraImpl
	isDirty bool
}

// checkNeedsUpdate checks if the bubble needs to be refreshed.
func (b *bubbleBase) checkNeedsUpdate() bool {
	if !b.sprite.Visible() {
		return false
	}
	return b.isDirty || b.sprite.spriteState.IsDirty || b.camera.isDirty
}

// getBounds returns the sprite's bounds information.
func (b *bubbleBase) getBounds() (center, size mathf.Vec2) {
	bound := b.sprite.bounds()
	center = bound.Center()
	size = bound.Size
	return
}

// markClean marks the bubble as no longer needing a refresh.
func (b *bubbleBase) markClean() {
	b.isDirty = false
}

// markDirty marks the bubble as needing a refresh.
func (b *bubbleBase) markDirty() {
	b.isDirty = true
}

// waitAndStop is a helper function for waiting and then stopping a bubble.
func waitAndStop(secs float64, stopFunc func()) {
	engine.Wait(secs)
	stopFunc()
}

type textBubble struct {
	bubbleBase // Embedded common bubble functionality
	msg        string
	style      int // styleSay, styleThink
	panel      *ui.UiSay
}

func (pself *textBubble) destroyPanel() {
	panel := pself.panel
	pself.panel = nil
	if panel != nil {
		panel.Destroy()
	}
}

func (pself *textBubble) onUpdate(delta float64) {
	if pself.checkNeedsUpdate() {
		pself.refresh()
		pself.markClean()
	}
}

func (pself *textBubble) refresh() {
	if pself.panel == nil {
		return
	}
	center, size := pself.getBounds()
	pself.panel.SetText(pself.sprite.g.getWindowSize(), center, size, pself.msg, pself.style)
}

type quoterBubble struct {
	bubbleBase  // Embedded common bubble functionality
	message     string
	description string
	panel       *ui.UiQuote
}

func (pself *quoterBubble) destroyPanel() {
	panel := pself.panel
	pself.panel = nil
	if panel != nil {
		panel.Destroy()
	}
}

func (pself *quoterBubble) onUpdate(delta float64) {
	if pself.checkNeedsUpdate() {
		pself.refresh()
		pself.markClean()
	}
}

func (pself *quoterBubble) refresh() {
	if pself.panel == nil {
		return
	}

	center, size := pself.getBounds()
	extSize := 10.0
	pself.panel.SetText(center, size.Divf(2).Addf(extSize), pself.message, pself.description)
}

func (p *SpriteImpl) newBubbleBase() bubbleBase {
	return bubbleBase{sprite: p, camera: p.g.camera, isDirty: true}
}

func (p *SpriteImpl) sayOrThink(msg any, style int) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		p.doStopText()
		return
	}

	bubble := p.components.Bubble()
	bubble.upsertText(msgStr, style)
}

func (p *SpriteImpl) waitStopText(secs float64) {
	waitAndStop(secs, p.doStopText)
}

func (p *SpriteImpl) doStopText() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopText()
	}
}

func (p *SpriteImpl) quote(message, description string) {
	bubble := p.components.Bubble()
	bubble.upsertQuote(message, description)
}

func (p *SpriteImpl) waitStopQuote(secs float64) {
	waitAndStop(secs, p.doStopQuote)
}

func (p *SpriteImpl) doStopQuote() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopQuote()
	}
}
