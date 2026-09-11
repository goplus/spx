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
	"math"
	"strings"

	"github.com/goplus/spbase/mathf"
)

type sayBubbleRect struct {
	left   float64
	right  float64
	bottom float64
	top    float64
}

func (l SayBubbleLayout) candidate(isLeft bool) sayBubbleRect {
	x := float64(l.position.X)
	y := float64(l.position.Y)
	width := float64(l.extent.X)
	height := float64(l.extent.Y)
	if isLeft {
		return sayBubbleRect{left: x, right: x + width, bottom: y, top: y + height}
	}
	return sayBubbleRect{left: x - width, right: x, bottom: y, top: y + height}
}

func (r sayBubbleRect) overlapArea(other sayBubbleRect) float64 {
	width := math.Min(r.right, other.right) - math.Max(r.left, other.left)
	height := math.Min(r.top, other.top) - math.Max(r.bottom, other.bottom)
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}

func (r sayBubbleRect) outsideArea(viewport sayBubbleRect) float64 {
	area := (r.right - r.left) * (r.top - r.bottom)
	insideWidth := math.Max(0, math.Min(r.right, viewport.right)-math.Max(r.left, viewport.left))
	insideHeight := math.Max(0, math.Min(r.top, viewport.top)-math.Max(r.bottom, viewport.bottom))
	return area - insideWidth*insideHeight
}

func (r sayBubbleRect) grow(amount float64) sayBubbleRect {
	return sayBubbleRect{
		left:   r.left - amount,
		right:  r.right + amount,
		bottom: r.bottom - amount,
		top:    r.top + amount,
	}
}

func validWindowScale(windowScale float64) float64 {
	if windowScale <= 0 {
		return 1
	}
	return windowScale
}

func calculateSayRenderScale(
	winSize mathf.Vec2,
	baseSize mathf.Vec2,
	cameraZoom mathf.Vec2,
	windowScale float64,
	isThink bool,
) mathf.Vec2 {
	windowScale = validWindowScale(windowScale)
	uniformScale := 1.0
	if baseSize.X > 0 && baseSize.Y > 0 {
		scaleVec := winSize.Div(baseSize)
		uniformScale = math.Min(float64(scaleVec.X), float64(scaleVec.Y))
	}
	if isThink {
		uniformScale *= thinkScale
	}
	return cameraZoom.Divf(windowScale).Mulf(uniformScale)
}

func clampSayPositionToExtent(position mathf.Vec2, viewport sayBubbleRect, extent mathf.Vec2) mathf.Vec2 {
	minY := viewport.bottom
	maxY := viewport.top - float64(extent.Y)
	clampedY := math.Max(minY, math.Min(float64(position.Y), maxY))
	clampedX := math.Max(viewport.left, math.Min(float64(position.X), viewport.right))
	return mathf.NewVec2(clampedX, clampedY)
}

func estimateSayBubbleExtent(msg string, style int) mathf.Vec2 {
	maxLineWidth := 0
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		width := displayWidth(line)
		if width > maxLineWidth {
			maxLineWidth = width
		}
	}
	lineCount := len(lines)

	if style == StyleThink {
		width := math.Max(thinkBubbleMinWidth, thinkBubbleHorizontalPad+float64(maxLineWidth)*thinkBubbleCharWidth)
		height := thinkBubbleDefaultHeight + float64(lineCount-1)*sayMsgLineHeight
		return mathf.NewVec2(width, height)
	}

	width := math.Max(sayBubbleMinWidth, sayBubbleHorizontalPadding+float64(maxLineWidth)*sayBubbleCharWidth)
	height := sayMsgDefaultHeight + float64(lineCount-1)*sayMsgLineHeight
	return mathf.NewVec2(width, height)
}

func displayWidth(text string) int {
	width := 0
	for _, r := range text {
		if r <= 0x7f {
			width++
		} else {
			width += 2
		}
	}
	return width
}
