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
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestResolveSayBubbleLayoutsEscapesJointFlipLocalMinimum(t *testing.T) {
	winSize := mathf.NewVec2(480, 360)
	layouts := []SayBubbleLayout{
		newSayBubbleLayout(winSize, mathf.NewVec2(-40, 0), "same message", StyleSay, true),
		newSayBubbleLayout(winSize, mathf.NewVec2(40, 0), "same message", StyleSay, false),
	}
	layouts[0].stableID = 1
	layouts[1].stableID = 2
	// Make the anchor distance exactly half the common width: with the safety
	// gap included, inward and either single-flip arrangement have equal area.
	layouts[0].extent = mathf.NewVec2(160, 77)
	layouts[1].extent = mathf.NewVec2(160, 77)
	component := []int{0, 1}

	// Bit 0 is the first bubble's direction and bit 1 is the second's.
	// The preferred inward arrangement is 01. Flipping either bubble alone
	// leaves the overlap unchanged, so coordinate descent cannot make progress;
	// only the joint outward flip to 10 removes the overlap.
	preferred := scoreSayBubbleMask(layouts, component, 0b01)
	for _, singleFlip := range []uint64{0b00, 0b11} {
		if score := scoreSayBubbleMask(layouts, component, singleFlip); score.less(preferred) {
			t.Fatalf("single-flip score %+v unexpectedly improves preferred score %+v", score, preferred)
		}
	}
	outward := scoreSayBubbleMask(layouts, component, 0b10)
	if !outward.less(preferred) || outward.overlapArea != 0 {
		t.Fatalf("joint-flip score = %+v, want zero-overlap improvement over %+v", outward, preferred)
	}

	ResolveSayBubbleLayouts(layouts)
	if layouts[0].isLeft || !layouts[1].isLeft {
		t.Fatalf("directions = [%t, %t], want joint outward flip [false, true]", layouts[0].isLeft, layouts[1].isLeft)
	}
}

func TestResolveSayBubbleLayoutsStableIDsIgnoreInputOrder(t *testing.T) {
	winSize := mathf.NewVec2(480, 360)
	base := make([]SayBubbleLayout, 3)
	for i, id := range []uint64{30, 10, 20} {
		base[i] = newSayBubbleLayout(winSize, mathf.NewVec2(0, 0), "same", StyleSay, true)
		base[i].stableID = id
	}

	permutations := [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
	var wantDirections map[uint64]bool
	var wantScore sayBubbleScore
	for permutationIndex, permutation := range permutations {
		layouts := make([]SayBubbleLayout, len(permutation))
		for i, sourceIndex := range permutation {
			layouts[i] = base[sourceIndex]
		}

		ResolveSayBubbleLayouts(layouts)
		directions := make(map[uint64]bool, len(layouts))
		component := make([]int, len(layouts))
		for i, layout := range layouts {
			directions[layout.stableID] = layout.isLeft
			component[i] = i
		}
		score := scoreSayBubbleComponent(layouts, component)
		if permutationIndex == 0 {
			wantDirections = directions
			wantScore = score
			continue
		}
		if !reflect.DeepEqual(directions, wantDirections) {
			t.Fatalf("permutation %v directions = %v, want ID mapping %v", permutation, directions, wantDirections)
		}
		if score != wantScore {
			t.Fatalf("permutation %v score = %+v, want %+v", permutation, score, wantScore)
		}
	}
}

func TestSayBubbleLayoutContextTransformsSpriteTopAndScale(t *testing.T) {
	oldClamp := clampUIPositionInScreen
	clampUIPositionInScreen = false
	t.Cleanup(func() { clampUIPositionInScreen = oldClamp })

	winSize := mathf.NewVec2(960, 540)
	baseSize := mathf.NewVec2(480, 360)
	cameraPosition := mathf.NewVec2(10, 20)
	cameraZoom := mathf.NewVec2(4, 6)
	context := newSayBubbleLayoutContext(winSize, baseSize, cameraPosition, cameraZoom, 2)
	content := NewSayBubbleContent("hello", StyleSay)
	layout := context.NewLayout(
		7,
		mathf.NewVec2(30, 40),
		mathf.NewVec2(12, 20),
		content,
	)

	// The non-480x360 window has a uniform scale of min(2, 1.5) = 1.5.
	// Position uses cameraZoom.X/windowScale = 2 and transforms the sprite's
	// world-space top (30, 50), not its center followed by an unscaled offset.
	assertVec2Near(t, "position", layout.position, mathf.NewVec2(40, 60))
	assertVec2Near(t, "render scale", layout.renderScale, mathf.NewVec2(3, 4.5))
	assertVec2Near(t, "collision extent", layout.extent, mathf.NewVec2(
		float64(content.baseExtent.X)*1.5,
		float64(content.baseExtent.Y)*2.25,
	))
	if layout.stableID != 7 {
		t.Fatalf("stableID = %d, want 7", layout.stableID)
	}
}

func TestResolveSayBubbleLayoutsUsesPreviousDirectionLast(t *testing.T) {
	t.Run("preferred direction beats previous direction", func(t *testing.T) {
		layout := newSayBubbleLayout(mathf.NewVec2(480, 360), mathf.NewVec2(0, 0), "same", StyleSay, true)
		layout.stableID = 1
		previous := layout
		previous.isLeft = false
		layout = layout.WithPreviousDirection(previous)

		layouts := []SayBubbleLayout{layout}
		ResolveSayBubbleLayouts(layouts)
		if !layouts[0].isLeft {
			t.Fatal("previous direction overrode the lower flipped-count preferred direction")
		}
	})

	t.Run("previous direction breaks an otherwise equal score", func(t *testing.T) {
		winSize := mathf.NewVec2(480, 360)
		layouts := []SayBubbleLayout{
			newSayBubbleLayout(winSize, mathf.NewVec2(0, 0), "same", StyleSay, true),
			newSayBubbleLayout(winSize, mathf.NewVec2(0, 0), "same", StyleSay, true),
		}
		layouts[0].stableID = 1
		layouts[1].stableID = 2
		previous0 := layouts[0]
		previous0.isLeft = true
		previous1 := layouts[1]
		previous1.isLeft = false
		layouts[0] = layouts[0].WithPreviousDirection(previous0)
		layouts[1] = layouts[1].WithPreviousDirection(previous1)

		ResolveSayBubbleLayouts(layouts)
		if !layouts[0].isLeft || layouts[1].isLeft {
			t.Fatalf("directions = [%t, %t], want previous tie-break [true, false]", layouts[0].isLeft, layouts[1].isLeft)
		}
	})
}

func TestSayBubbleGeometryAtCoincidentAndExactBoundaries(t *testing.T) {
	coincident := sayBubbleRect{left: -10, right: 10, bottom: -5, top: 5}
	if overlap := coincident.overlapArea(coincident); overlap != 200 {
		t.Fatalf("coincident overlap = %v, want 200", overlap)
	}

	touching := sayBubbleRect{left: 10, right: 20, bottom: -5, top: 5}
	if overlap := coincident.overlapArea(touching); overlap != 0 {
		t.Fatalf("edge-touching overlap = %v, want 0", overlap)
	}

	viewport := sayBubbleRect{left: -10, right: 10, bottom: -5, top: 5}
	if outside := coincident.outsideArea(viewport); outside != 0 {
		t.Fatalf("exact-boundary outside area = %v, want 0", outside)
	}
	clamped := clampSayPositionToExtent(mathf.NewVec2(10, 5), viewport, mathf.NewVec2(4, 3))
	assertVec2Near(t, "clamped boundary anchor", clamped, mathf.NewVec2(10, 2))
	if scale := validWindowScale(0); scale != 1 {
		t.Fatalf("zero window scale fallback = %v, want 1", scale)
	}
}

func assertVec2Near(t *testing.T, name string, got, want mathf.Vec2) {
	t.Helper()
	const epsilon = 1e-9
	if math.Abs(float64(got.X-want.X)) > epsilon || math.Abs(float64(got.Y-want.Y)) > epsilon {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestResolveSayBubbleLayoutsKeepsSeparatedPreferredDirections(t *testing.T) {
	winSize := mathf.NewVec2(480, 360)
	layouts := []SayBubbleLayout{
		newSayBubbleLayout(winSize, mathf.NewVec2(-160, 0), "hello", StyleSay, true),
		newSayBubbleLayout(winSize, mathf.NewVec2(160, 0), "hello", StyleSay, false),
	}

	ResolveSayBubbleLayouts(layouts)

	if !layouts[0].isLeft || layouts[1].isLeft {
		t.Fatalf("directions = [%t, %t], want preferred [true, false]", layouts[0].isLeft, layouts[1].isLeft)
	}
}

func TestResolveSayBubbleLayoutsTurnsInwardPairOutward(t *testing.T) {
	winSize := mathf.NewVec2(480, 360)
	layouts := []SayBubbleLayout{
		newSayBubbleLayout(winSize, mathf.NewVec2(-40, 0), "你好 Kiko", StyleSay, true),
		newSayBubbleLayout(winSize, mathf.NewVec2(40, 0), "你好 Amy", StyleSay, false),
	}

	ResolveSayBubbleLayouts(layouts)

	if layouts[0].isLeft || !layouts[1].isLeft {
		t.Fatalf("directions = [%t, %t], want outward [false, true]", layouts[0].isLeft, layouts[1].isLeft)
	}
	first := layouts[0].candidate(layouts[0].isLeft)
	second := layouts[1].candidate(layouts[1].isLeft)
	if overlap := first.overlapArea(second); overlap != 0 {
		t.Fatalf("outward bubbles still overlap by %v", overlap)
	}
}

func TestResolveSayBubbleLayoutsDoesNotFlipOffscreen(t *testing.T) {
	winSize := mathf.NewVec2(480, 360)
	layouts := []SayBubbleLayout{
		newSayBubbleLayout(winSize, mathf.NewVec2(-220, 0), "a fairly long message", StyleSay, true),
		newSayBubbleLayout(winSize, mathf.NewVec2(-130, 0), "a fairly long message", StyleSay, true),
	}

	ResolveSayBubbleLayouts(layouts)

	if !layouts[0].isLeft {
		t.Fatal("left-edge bubble flipped outside the viewport")
	}
}

func TestEstimateSayBubbleExtentAccountsForWideTextAndLines(t *testing.T) {
	ascii := estimateSayBubbleExtent("abcd", StyleSay)
	wide := estimateSayBubbleExtent("中文中文", StyleSay)
	multiline := estimateSayBubbleExtent("a\nb", StyleSay)

	if wide.X <= ascii.X {
		t.Fatalf("wide text width = %v, want greater than ASCII width %v", wide.X, ascii.X)
	}
	if multiline.Y <= ascii.Y {
		t.Fatalf("multiline height = %v, want greater than single-line height %v", multiline.Y, ascii.Y)
	}
}
