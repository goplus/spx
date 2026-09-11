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

package ui

import (
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

const (
	sayBubbleGap               = 8.0
	sayBubbleMinWidth          = 57.0
	sayBubbleHorizontalPadding = 56.0
	sayBubbleCharWidth         = 8.0
	thinkBubbleMinWidth        = 67.0
	thinkBubbleHorizontalPad   = 64.0
	thinkBubbleCharWidth       = 12.0
	thinkBubbleDefaultHeight   = 134.0
	sayBubbleExactLayoutLimit  = 12
	sayBubbleFallbackSeeds     = 5
)

// SayBubbleContent caches formatting and geometry that only change when the
// bubble's message or style changes.
type SayBubbleContent struct {
	formattedMessage string
	style            int
	baseExtent       mathf.Vec2
}

// SayBubbleLayoutContext snapshots the camera and display state shared by all
// bubbles in one pass. Candidate rectangles therefore use exactly the same
// scale as UiSay.SetTextLayout without querying the engine for every bubble.
type SayBubbleLayoutContext struct {
	windowScale    float64
	cameraPosition mathf.Vec2
	worldViewScale float64
	sayRenderScale mathf.Vec2
	viewport       sayBubbleRect
}

// SayBubbleLayout is the resolved render state of a Say/Think bubble. Fields
// remain private so the UI cannot apply a layout that disagrees with the
// collision bounds used by ResolveSayBubbleLayouts.
type SayBubbleLayout struct {
	stableID        uint64
	position        mathf.Vec2
	isLeft          bool
	preferredIsLeft bool
	previousIsLeft  bool
	hasPrevious     bool
	content         SayBubbleContent
	extent          mathf.Vec2
	renderScale     mathf.Vec2
	viewport        sayBubbleRect
}

// NewLayout builds the preferred layout for one bubble. stableID must remain
// unchanged for the bubble's lifetime so equal-score layouts never depend on
// shape activation or render order.
func (c SayBubbleLayoutContext) NewLayout(
	stableID uint64,
	worldPosition mathf.Vec2,
	spriteSize mathf.Vec2,
	content SayBubbleContent,
) SayBubbleLayout {
	// Transform the sprite's top point as a whole. Adding half the unscaled
	// height after WorldToView detaches the tail when camera zoom is not 1.
	worldTop := worldPosition.Add(mathf.NewVec2(0, float64(spriteSize.Y)/2))
	position := worldTop.Sub(c.cameraPosition).Mulf(c.worldViewScale)

	renderScale := c.sayRenderScale
	if content.style == StyleThink {
		renderScale = renderScale.Mulf(thinkScale)
	}
	// ViewToUI multiplies positions by WindowScale, whereas Control sizes are
	// already UI pixels. Divide by WindowScale to compare both in view units.
	extent := content.baseExtent.Mul(renderScale).Divf(c.windowScale)
	if clampUIPositionInScreen {
		position = clampSayPositionToExtent(position, c.viewport, extent)
	}

	preferredIsLeft := position.X <= 0
	return SayBubbleLayout{
		stableID:        stableID,
		position:        position,
		isLeft:          preferredIsLeft,
		preferredIsLeft: preferredIsLeft,
		content:         content,
		extent:          extent,
		renderScale:     renderScale,
		viewport:        c.viewport,
	}
}

// WithPreviousDirection records the last rendered direction as a late
// tie-breaker. Preferred stage-side placement still takes precedence.
func (l SayBubbleLayout) WithPreviousDirection(previous SayBubbleLayout) SayBubbleLayout {
	l.previousIsLeft = previous.isLeft
	l.hasPrevious = true
	return l
}

// SameInput reports whether resolving another layout can change the result.
func (l SayBubbleLayout) SameInput(other SayBubbleLayout) bool {
	return l.stableID == other.stableID &&
		l.position == other.position &&
		l.preferredIsLeft == other.preferredIsLeft &&
		l.content == other.content &&
		l.extent == other.extent &&
		l.renderScale == other.renderScale &&
		l.viewport == other.viewport
}

// Equal reports whether applying another layout would change the rendered UI.
func (l SayBubbleLayout) Equal(other SayBubbleLayout) bool {
	return l.position == other.position &&
		l.isLeft == other.isLeft &&
		l.content.formattedMessage == other.content.formattedMessage &&
		l.content.style == other.content.style &&
		l.renderScale == other.renderScale
}

// NewSayBubbleContent prepares the immutable text/style portion of a bubble.
func NewSayBubbleContent(msg string, style int) SayBubbleContent {
	formattedMessage := formatSayMessage(msg)
	return SayBubbleContent{
		formattedMessage: formattedMessage,
		style:            style,
		baseExtent:       estimateSayBubbleExtent(formattedMessage, style),
	}
}

// NewSayBubbleLayoutContext captures the current display state.
func NewSayBubbleLayoutContext(winSize mathf.Vec2) SayBubbleLayoutContext {
	baseSize := mathf.NewVec2(float64(baseScreenWidth), float64(baseScreenHeight))
	return newSayBubbleLayoutContext(
		winSize,
		baseSize,
		engine.Managers().CameraMgr.GetPosition(),
		engine.Managers().CameraMgr.GetCameraZoom(),
		engine.WindowScale(),
	)
}

// NewSayBubbleLayout builds a standalone layout. The shape manager uses a
// shared context and cached SayBubbleContent for multiple bubbles.
func NewSayBubbleLayout(winSize, worldPosition, spriteSize mathf.Vec2, msg string, style int) SayBubbleLayout {
	content := NewSayBubbleContent(msg, style)
	return NewSayBubbleLayoutContext(winSize).NewLayout(0, worldPosition, spriteSize, content)
}

func newSayBubbleLayoutContext(
	winSize mathf.Vec2,
	baseSize mathf.Vec2,
	cameraPosition mathf.Vec2,
	cameraZoom mathf.Vec2,
	windowScale float64,
) SayBubbleLayoutContext {
	windowScale = validWindowScale(windowScale)
	return SayBubbleLayoutContext{
		windowScale:    windowScale,
		cameraPosition: cameraPosition,
		worldViewScale: float64(cameraZoom.X) / windowScale,
		sayRenderScale: calculateSayRenderScale(winSize, baseSize, cameraZoom, windowScale, false),
		viewport: sayBubbleRect{
			left:   -float64(winSize.X) / 2,
			right:  float64(winSize.X) / 2,
			bottom: -float64(winSize.Y) / 2,
			top:    float64(winSize.Y) / 2,
		},
	}
}

func newSayBubbleLayout(winSize, position mathf.Vec2, formattedMessage string, style int, preferredIsLeft bool) SayBubbleLayout {
	content := SayBubbleContent{
		formattedMessage: formattedMessage,
		style:            style,
		baseExtent:       estimateSayBubbleExtent(formattedMessage, style),
	}
	return SayBubbleLayout{
		position:        position,
		isLeft:          preferredIsLeft,
		preferredIsLeft: preferredIsLeft,
		content:         content,
		extent:          content.baseExtent,
		renderScale:     mathf.NewVec2(1, 1),
		viewport: sayBubbleRect{
			left:   -float64(winSize.X) / 2,
			right:  float64(winSize.X) / 2,
			bottom: -float64(winSize.Y) / 2,
			top:    float64(winSize.Y) / 2,
		},
	}
}
