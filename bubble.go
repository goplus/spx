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
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/base"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/ui"
)

type bubbleComponent struct {
	sprite *SpriteImpl
	mu     sync.Mutex

	textObj  *textBubble  // Text bubble object (Say/Think).
	quoteObj *quoteBubble // Quote bubble object.
}

type bubbleShape interface {
	destroyPanel()
	onUpdate()
	flushPendingRemoval(int64)
}

// bubbleBase provides common functionality for all bubble types.
type bubbleBase struct {
	sprite                *SpriteImpl
	camera                *cameraImpl
	isDirty               bool
	observedSpriteVersion uint64
	observedCameraVersion uint64

	// Removal state is protected by the owning bubbleComponent.mu.
	timedRemovalPending bool
	timedRemovalFrame   int64
	generation          uint64
}

// bubbleToken limits timed removal to one displayed generation.
type bubbleToken struct {
	bubble     *bubbleBase
	generation uint64
}

type textBubble struct {
	bubbleBase // Embedded common bubble functionality
	msg        string
	style      int // styleSay, styleThink
	panel      *ui.UiSay

	layoutID  uint64
	content   ui.SayBubbleContent
	layout    ui.SayBubbleLayout
	hasLayout bool
}

type quoteBubble struct {
	bubbleBase  // Embedded common bubble functionality
	message     string
	description string
	panel       *ui.UiQuote
}

// bubbleLayout caches the ordered inputs and results of scene-wide layout.
type bubbleLayout struct {
	bubbles                []bubbleShape
	textBubbles            []*textBubble
	activeTextBubbles      []*textBubble
	sayLayouts             []ui.SayBubbleLayout
	nextTextBubbleLayoutID uint64
}

func (b *bubbleComponent) onDestroy() {
	b.stopAll()
}

func (b *bubbleComponent) upsertText(msg string, style int) bubbleToken {
	b.sprite.requestRedrawIfVisible()
	b.mu.Lock()
	textObj := b.textObj
	created := textObj == nil
	if created {
		textObj = &textBubble{
			bubbleBase: b.sprite.newBubbleBase(),
			msg:        msg,
			style:      style,
			panel:      ui.NewUiSay(),
			content:    ui.NewSayBubbleContent(msg, style),
		}
		b.textObj = textObj
	} else {
		if textObj.msg != msg || textObj.style != style {
			textObj.content = ui.NewSayBubbleContent(msg, style)
		}
		textObj.msg = msg
		textObj.style = style
		textObj.markDirty()
	}
	token := textObj.renew()
	b.mu.Unlock()

	if created {
		b.sprite.g.scene.add(textObj)
		return token
	}
	b.sprite.g.scene.activateShape(textObj)
	return token
}

func (b *bubbleComponent) upsertQuote(message, description string) bubbleToken {
	b.sprite.requestRedrawIfVisible()
	b.mu.Lock()
	quoteObj := b.quoteObj
	created := quoteObj == nil
	if created {
		quoteObj = &quoteBubble{
			bubbleBase:  b.sprite.newBubbleBase(),
			message:     message,
			description: description,
			panel:       ui.NewUiQuote(),
		}
		b.quoteObj = quoteObj
	} else {
		quoteObj.message = message
		quoteObj.description = description
		quoteObj.markDirty()
	}
	token := quoteObj.renew()
	b.mu.Unlock()

	if created {
		b.sprite.g.scene.add(quoteObj)
		return token
	}
	b.sprite.g.scene.activateShape(quoteObj)
	return token
}

func (b *bubbleComponent) stopText() {
	b.mu.Lock()
	textObj := b.textObj
	b.textObj = nil
	if textObj != nil {
		textObj.timedRemovalPending = false
	}
	b.mu.Unlock()
	if textObj == nil {
		return
	}
	b.stopBubble(textObj)
}

// expireText defers removal so the next frame can renew the bubble.
func (b *bubbleComponent) expireText(token bubbleToken) {
	b.mu.Lock()
	if b.textObj == nil || !b.textObj.scheduleRemoval(token, clock.Frame()) {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	engine.RequestRedraw()
}

// flushPendingTextRemoval commits expirations after their grace frame.
func (b *bubbleComponent) flushPendingTextRemoval(textObj *textBubble, frame int64) {
	b.mu.Lock()
	if b.textObj != textObj || !textObj.removalDue(frame) {
		b.mu.Unlock()
		return
	}
	b.textObj = nil
	textObj.timedRemovalPending = false
	b.mu.Unlock()

	b.stopBubble(textObj)
}

// expireQuote defers removal so the next frame can renew the bubble.
func (b *bubbleComponent) expireQuote(token bubbleToken) {
	b.mu.Lock()
	if b.quoteObj == nil || !b.quoteObj.scheduleRemoval(token, clock.Frame()) {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	engine.RequestRedraw()
}

// flushPendingQuoteRemoval commits expirations after their grace frame.
func (b *bubbleComponent) flushPendingQuoteRemoval(quoteObj *quoteBubble, frame int64) {
	b.mu.Lock()
	if b.quoteObj != quoteObj || !quoteObj.removalDue(frame) {
		b.mu.Unlock()
		return
	}
	b.quoteObj = nil
	quoteObj.timedRemovalPending = false
	b.mu.Unlock()

	b.stopBubble(quoteObj)
}

func (b *bubbleComponent) stopQuote() {
	b.mu.Lock()
	quoteObj := b.quoteObj
	b.quoteObj = nil
	if quoteObj != nil {
		quoteObj.timedRemovalPending = false
	}
	b.mu.Unlock()
	if quoteObj == nil {
		return
	}
	b.stopBubble(quoteObj)
}

func (b *bubbleComponent) stopAll() {
	b.stopText()
	b.stopQuote()
}

func (b *bubbleComponent) stopBubble(obj bubbleShape) {
	engine.RequestRedraw()
	obj.destroyPanel()
	b.sprite.g.scene.removeShape(obj)
}

// renew supersedes older timed removals, even when the content is unchanged.
func (b *bubbleBase) renew() bubbleToken {
	b.generation++
	if b.generation == 0 {
		// Zero represents an invalid token.
		b.generation++
	}
	b.timedRemovalPending = false
	return bubbleToken{bubble: b, generation: b.generation}
}

// scheduleRemoval validates the token against the component's current bubble.
func (b *bubbleBase) scheduleRemoval(token bubbleToken, frame int64) bool {
	if token.bubble != b || token.generation != b.generation {
		return false
	}
	b.timedRemovalPending = true
	b.timedRemovalFrame = frame
	return true
}

func (b *bubbleBase) removalDue(frame int64) bool {
	return b.timedRemovalPending && b.timedRemovalFrame < frame
}

// needsUpdate checks if the bubble needs to be refreshed.
func (b *bubbleBase) needsUpdate() bool {
	if !b.sprite.Visible() {
		return false
	}
	return b.isDirty ||
		b.observedSpriteVersion != b.sprite.spriteState.visualVersion ||
		b.observedCameraVersion != b.camera.dirtyVersion
}

// getBounds returns the sprite's bounds information.
func (b *bubbleBase) getBounds() (center, size mathf.Vec2) {
	bound := b.sprite.bubbleBounds()
	center = bound.Center()
	size = bound.Size
	return
}

func (b *bubbleBase) markClean() {
	b.observedSpriteVersion = b.sprite.spriteState.visualVersion
	b.observedCameraVersion = b.camera.dirtyVersion
	b.isDirty = false
}

func (b *bubbleBase) markDirty() {
	b.isDirty = true
}

func (p *textBubble) destroyPanel() {
	panel := p.panel
	p.panel = nil
	if panel != nil {
		panel.Destroy()
	}
}

func (p *textBubble) flushPendingRemoval(frame int64) {
	p.sprite.bubble().flushPendingTextRemoval(p, frame)
}

func (p *textBubble) onUpdate() {
	if p.needsUpdate() {
		if p.panel != nil {
			// The shape manager resolves visible text layouts before this update.
			p.panel.SetTextLayout(p.layout)
		}
		p.markClean()
	}
}

func (p *textBubble) setLayout(layout ui.SayBubbleLayout) {
	renderChanged := !p.hasLayout || !p.layout.Equal(layout)
	p.layout = layout
	p.hasLayout = true
	if renderChanged {
		p.markDirty()
	}
}

func (p *quoteBubble) destroyPanel() {
	panel := p.panel
	p.panel = nil
	if panel != nil {
		panel.Destroy()
	}
}

func (p *quoteBubble) flushPendingRemoval(frame int64) {
	p.sprite.bubble().flushPendingQuoteRemoval(p, frame)
}

func (p *quoteBubble) onUpdate() {
	if p.needsUpdate() {
		p.refresh()
		p.markClean()
	}
}

func (p *quoteBubble) refresh() {
	if p.panel == nil {
		return
	}

	center, size := p.getBounds()
	extSize := 10.0
	p.panel.SetText(center, size.Divf(2).Addf(extSize), p.message, p.description)
}

func (s *bubbleLayout) reset() {
	s.bubbles = base.Reset(s.bubbles, 0)
	s.textBubbles = base.Reset(s.textBubbles, 0)
	s.activeTextBubbles = base.Reset(s.activeTextBubbles, 0)
	s.sayLayouts = base.Reset(s.sayLayouts, 0)
	s.nextTextBubbleLayoutID = 0
}

func (s *bubbleLayout) flushBubbleVisuals(items []Shape) {
	s.collectBubbles(items)

	frame := clock.Frame()
	// Remove expired bubbles before resolving this frame's layouts.
	for _, bubble := range s.bubbles {
		bubble.flushPendingRemoval(frame)
	}

	s.layoutTextBubbles()

	for _, bubble := range s.bubbles {
		bubble.onUpdate()
	}
}

func (s *bubbleLayout) collectBubbles(items []Shape) {
	s.bubbles = base.Reset(s.bubbles, 0)
	s.textBubbles = base.Reset(s.textBubbles, 0)
	for _, item := range items {
		bubble, ok := item.(bubbleShape)
		if !ok {
			continue
		}
		s.bubbles = append(s.bubbles, bubble)
		if textBubble, ok := bubble.(*textBubble); ok {
			s.textBubbles = append(s.textBubbles, textBubble)
		}
	}
}

func (s *bubbleLayout) layoutTextBubbles() {
	s.sayLayouts = base.Reset(s.sayLayouts, 0)

	s.textBubbles = slices.DeleteFunc(s.textBubbles, func(bubble *textBubble) bool {
		return bubble.panel == nil || !bubble.sprite.Visible()
	})

	slices.SortStableFunc(s.textBubbles, func(a, b *textBubble) int {
		return cmp.Compare(a.layoutID, b.layoutID)
	})
	topologyChanged := !slices.Equal(s.textBubbles, s.activeTextBubbles)
	if topologyChanged {
		s.activeTextBubbles = base.CopyInto(s.activeTextBubbles, s.textBubbles, 0)
	}
	if len(s.textBubbles) == 0 {
		return
	}

	width, height := s.textBubbles[0].sprite.g.windowSize()
	context := ui.NewSayBubbleLayoutContext(mathf.NewVec2(float64(width), float64(height)))
	needsResolve := topologyChanged
	for _, bubble := range s.textBubbles {
		center, size := bubble.getBounds()
		layout := context.NewLayout(bubble.layoutID, center, size, bubble.content)
		if bubble.hasLayout {
			layout = layout.WithPreviousDirection(bubble.layout)
		}
		needsResolve = needsResolve || !bubble.hasLayout || !bubble.layout.SameInput(layout)
		s.sayLayouts = append(s.sayLayouts, layout)
	}
	if !needsResolve {
		return
	}

	ui.ResolveSayBubbleLayouts(s.sayLayouts)
	for i, bubble := range s.textBubbles {
		bubble.setLayout(s.sayLayouts[i])
	}
}

func dialogText(msg any) string {
	if msgStr, ok := msg.(string); ok {
		return msgStr
	}
	return fmt.Sprint(msg)
}
