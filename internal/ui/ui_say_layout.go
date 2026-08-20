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
	"sort"
	"strings"

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

// NewSayBubbleContent prepares the immutable text/style portion of a bubble.
func NewSayBubbleContent(msg string, style int) SayBubbleContent {
	formattedMessage := formatSayMessage(msg)
	return SayBubbleContent{
		formattedMessage: formattedMessage,
		style:            style,
		baseExtent:       estimateSayBubbleExtent(formattedMessage, style),
	}
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

// NewSayBubbleLayoutContext captures the current display state.
func NewSayBubbleLayoutContext(winSize mathf.Vec2) SayBubbleLayoutContext {
	baseSize := mathf.NewVec2(float64(baseScreenWidth), float64(baseScreenHeight))
	return newSayBubbleLayoutContext(
		winSize,
		baseSize,
		mgr.CameraMgr.GetPosition(),
		mgr.CameraMgr.GetCameraZoom(),
		engine.WindowScale(),
	)
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

// NewSayBubbleLayout builds a standalone layout. The shape manager uses a
// shared context and cached SayBubbleContent for multiple bubbles.
func NewSayBubbleLayout(winSize, worldPosition, spriteSize mathf.Vec2, msg string, style int) SayBubbleLayout {
	content := NewSayBubbleContent(msg, style)
	return NewSayBubbleLayoutContext(winSize).NewLayout(0, worldPosition, spriteSize, content)
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

// ResolveSayBubbleLayouts chooses a direction for each bubble. Components of
// up to sayBubbleExactLayoutLimit bubbles are solved globally, avoiding local
// minima from one-at-a-time flipping. Larger components use deterministic
// multi-start local search. Both paths use stable IDs, not slice order.
func ResolveSayBubbleLayouts(layouts []SayBubbleLayout) {
	if len(layouts) == 0 {
		return
	}
	for i := range layouts {
		layouts[i].isLeft = layouts[i].preferredIsLeft
	}

	order := canonicalSayBubbleOrder(layouts)
	visited := make([]bool, len(layouts))
	queue := make([]int, 0, len(layouts))
	component := make([]int, 0, len(layouts))
	for _, seed := range order {
		if visited[seed] {
			continue
		}
		queue = append(queue[:0], seed)
		component = component[:0]
		visited[seed] = true
		for len(queue) > 0 {
			index := queue[0]
			queue = queue[1:]
			component = append(component, index)
			for _, other := range order {
				if visited[other] || !sayBubblesMayOverlap(layouts[index], layouts[other]) {
					continue
				}
				visited[other] = true
				queue = append(queue, other)
			}
		}

		sort.Slice(component, func(i, j int) bool {
			return sayBubbleCanonicalLess(layouts[component[i]], layouts[component[j]])
		})
		if len(component) <= sayBubbleExactLayoutLimit {
			resolveSayBubbleComponentExact(layouts, component)
		} else {
			resolveSayBubbleComponentFallback(layouts, component)
		}
	}
}

func canonicalSayBubbleOrder(layouts []SayBubbleLayout) []int {
	order := make([]int, len(layouts))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return sayBubbleCanonicalLess(layouts[order[i]], layouts[order[j]])
	})
	return order
}

func sayBubbleCanonicalLess(a, b SayBubbleLayout) bool {
	if a.stableID != b.stableID {
		return a.stableID < b.stableID
	}
	if a.position.X != b.position.X {
		return a.position.X < b.position.X
	}
	if a.position.Y != b.position.Y {
		return a.position.Y < b.position.Y
	}
	if a.extent.X != b.extent.X {
		return a.extent.X < b.extent.X
	}
	if a.extent.Y != b.extent.Y {
		return a.extent.Y < b.extent.Y
	}
	if a.preferredIsLeft != b.preferredIsLeft {
		return !a.preferredIsLeft
	}
	if a.content.style != b.content.style {
		return a.content.style < b.content.style
	}
	if a.content.formattedMessage != b.content.formattedMessage {
		return a.content.formattedMessage < b.content.formattedMessage
	}
	if a.hasPrevious != b.hasPrevious {
		return !a.hasPrevious
	}
	return !a.previousIsLeft && b.previousIsLeft
}

func sayBubblesMayOverlap(a, b SayBubbleLayout) bool {
	for _, aIsLeft := range [...]bool{false, true} {
		aRect := a.candidate(aIsLeft).grow(sayBubbleGap / 2)
		for _, bIsLeft := range [...]bool{false, true} {
			bRect := b.candidate(bIsLeft).grow(sayBubbleGap / 2)
			if aRect.overlapArea(bRect) > 0 {
				return true
			}
		}
	}
	return false
}

func resolveSayBubbleComponentExact(layouts []SayBubbleLayout, component []int) {
	combinationCount := uint64(1) << uint(len(component))
	var bestMask uint64
	var bestScore sayBubbleScore
	hasBest := false
	for mask := uint64(0); mask < combinationCount; mask++ {
		score := scoreSayBubbleMask(layouts, component, mask)
		if !hasBest || score.less(bestScore) ||
			(score.equal(bestScore) && sayBubbleMaskLess(mask, bestMask, len(component))) {
			bestMask = mask
			bestScore = score
			hasBest = true
		}
	}
	for i, index := range component {
		layouts[index].isLeft = bestMask&(uint64(1)<<uint(i)) != 0
	}
}

func scoreSayBubbleMask(layouts []SayBubbleLayout, component []int, mask uint64) sayBubbleScore {
	score := sayBubbleScore{}
	for i, index := range component {
		isLeft := mask&(uint64(1)<<uint(i)) != 0
		layout := layouts[index]
		rect := layout.candidate(isLeft)
		score.outsideArea += rect.outsideArea(layout.viewport)
		score.flippedCount += boolInt(isLeft != layout.preferredIsLeft)
		if layout.hasPrevious {
			score.changedCount += boolInt(isLeft != layout.previousIsLeft)
		}
		rect = rect.grow(sayBubbleGap / 2)
		for j, otherIndex := range component[i+1:] {
			otherIsLeft := mask&(uint64(1)<<uint(i+j+1)) != 0
			other := layouts[otherIndex].candidate(otherIsLeft).grow(sayBubbleGap / 2)
			score.overlapArea += rect.overlapArea(other)
		}
	}
	return score
}

func sayBubbleMaskLess(a, b uint64, count int) bool {
	for i := 0; i < count; i++ {
		aIsLeft := a&(uint64(1)<<uint(i)) != 0
		bIsLeft := b&(uint64(1)<<uint(i)) != 0
		if aIsLeft != bIsLeft {
			return !aIsLeft
		}
	}
	return false
}

func resolveSayBubbleComponentFallback(layouts []SayBubbleLayout, component []int) {
	bestDirections := make([]bool, len(component))
	directions := make([]bool, len(component))
	var bestScore sayBubbleScore
	hasBest := false

	for seed := 0; seed < sayBubbleFallbackSeeds; seed++ {
		for i, index := range component {
			layout := layouts[index]
			switch seed {
			case 0:
				directions[i] = layout.preferredIsLeft
			case 1:
				if layout.hasPrevious {
					directions[i] = layout.previousIsLeft
				} else {
					directions[i] = layout.preferredIsLeft
				}
			case 2:
				directions[i] = false
			case 3:
				directions[i] = true
			default:
				directions[i] = !layout.preferredIsLeft
			}
			layouts[index].isLeft = directions[i]
		}

		for pass := 0; pass < len(component); pass++ {
			changed := false
			for i, index := range component {
				currentIsLeft := layouts[index].isLeft
				current := scoreSayBubbleAt(layouts, component, index, currentIsLeft)
				alternate := scoreSayBubbleAt(layouts, component, index, !currentIsLeft)
				if alternate.less(current) {
					layouts[index].isLeft = !currentIsLeft
					directions[i] = !currentIsLeft
					changed = true
				}
			}
			if !changed {
				break
			}
		}

		score := scoreSayBubbleComponent(layouts, component)
		if !hasBest || score.less(bestScore) ||
			(score.equal(bestScore) && sayBubbleDirectionsLess(directions, bestDirections)) {
			copy(bestDirections, directions)
			bestScore = score
			hasBest = true
		}
	}

	for i, index := range component {
		layouts[index].isLeft = bestDirections[i]
	}
}

func scoreSayBubbleComponent(layouts []SayBubbleLayout, component []int) sayBubbleScore {
	score := sayBubbleScore{}
	for i, index := range component {
		layout := layouts[index]
		rect := layout.candidate(layout.isLeft)
		score.outsideArea += rect.outsideArea(layout.viewport)
		score.flippedCount += boolInt(layout.isLeft != layout.preferredIsLeft)
		if layout.hasPrevious {
			score.changedCount += boolInt(layout.isLeft != layout.previousIsLeft)
		}
		rect = rect.grow(sayBubbleGap / 2)
		for _, otherIndex := range component[i+1:] {
			other := layouts[otherIndex].candidate(layouts[otherIndex].isLeft).grow(sayBubbleGap / 2)
			score.overlapArea += rect.overlapArea(other)
		}
	}
	return score
}

func scoreSayBubbleAt(layouts []SayBubbleLayout, component []int, index int, isLeft bool) sayBubbleScore {
	layout := layouts[index]
	candidate := layout.candidate(isLeft)
	score := sayBubbleScore{
		outsideArea:  candidate.outsideArea(layout.viewport),
		flippedCount: boolInt(isLeft != layout.preferredIsLeft),
	}
	if layout.hasPrevious {
		score.changedCount = boolInt(isLeft != layout.previousIsLeft)
	}
	candidate = candidate.grow(sayBubbleGap / 2)
	for _, otherIndex := range component {
		if otherIndex == index {
			continue
		}
		occupied := layouts[otherIndex].candidate(layouts[otherIndex].isLeft).grow(sayBubbleGap / 2)
		score.overlapArea += candidate.overlapArea(occupied)
	}
	return score
}

func sayBubbleDirectionsLess(a, b []bool) bool {
	for i := range a {
		if a[i] != b[i] {
			return !a[i]
		}
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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

type sayBubbleRect struct {
	left   float64
	right  float64
	bottom float64
	top    float64
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

type sayBubbleScore struct {
	outsideArea  float64
	overlapArea  float64
	flippedCount int
	changedCount int
}

func (s sayBubbleScore) less(other sayBubbleScore) bool {
	if s.outsideArea != other.outsideArea {
		return s.outsideArea < other.outsideArea
	}
	if s.overlapArea != other.overlapArea {
		return s.overlapArea < other.overlapArea
	}
	if s.flippedCount != other.flippedCount {
		return s.flippedCount < other.flippedCount
	}
	return s.changedCount < other.changedCount
}

func (s sayBubbleScore) equal(other sayBubbleScore) bool {
	return s == other
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
