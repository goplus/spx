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
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type uiRenderSpy struct {
	askShowUiMgr
	positions map[gdx.Object]mathf.Vec2
	scales    map[gdx.Object]mathf.Vec2
	sizes     map[gdx.Object]mathf.Vec2
	colors    map[gdx.Object]mathf.Color
	rotations map[gdx.Object]float64
}

type uiRenderCamera struct{ gdx.ICameraMgr }

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
	winSize := mathf.NewVec2(960, 540)
	baseSize := mathf.NewVec2(480, 360)
	cameraPosition := mathf.NewVec2(10, 20)
	cameraZoom := mathf.NewVec2(4, 6)
	for _, test := range []struct {
		name          string
		style         int
		scale, extent mathf.Vec2
	}{
		{"say", StyleSay, mathf.NewVec2(3, 4.5), mathf.NewVec2(144, 173.25)},
		{"think", StyleThink, mathf.NewVec2(2.4, 3.6), mathf.NewVec2(148.8, 241.2)},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := newSayBubbleLayoutContext(winSize, baseSize, cameraPosition, cameraZoom, 2, false)
			content := NewSayBubbleContent("hello", test.style)
			layout := context.NewLayout(7, mathf.NewVec2(30, 40), mathf.NewVec2(12, 20), content)

			// min(960/480, 540/360) = 1.5 scales the bubble, not its anchor.
			// The sprite's top (30, 50) is transformed as a whole by zoom.X/2.
			assertVec2Near(t, "tail anchor", layout.position, mathf.NewVec2(40, 60))
			assertVec2Near(t, "render scale", layout.renderScale, test.scale)
			assertVec2Near(t, "collision extent", layout.extent, test.extent)
			if layout.stableID != 7 || layout.viewport != (sayBubbleRect{left: -480, right: 480, bottom: -270, top: 270}) {
				t.Fatalf("layout identity/viewport = %d, %+v", layout.stableID, layout.viewport)
			}

			context.clampPosition = true
			clamped := context.NewLayout(7, mathf.NewVec2(30, 1000), mathf.NewVec2(12, 20), content)
			assertVec2Near(t, "clamped tail anchor", clamped.position, mathf.NewVec2(40, 270-test.extent.Y))
			bounds := clamped.candidate(clamped.isLeft)
			if math.Abs(bounds.top-270) > 1e-9 || math.Abs(bounds.outsideArea(clamped.viewport)) > 1e-9 {
				t.Fatalf("clamped bounds = %+v, want top at viewport edge without double scaling", bounds)
			}
		})
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

func TestSayBubbleLayoutContextKeepsClampPolicy(t *testing.T) {
	oldClamp := clampUIPositionInScreen
	t.Cleanup(func() { clampUIPositionInScreen = oldClamp })
	content := NewSayBubbleContent("hello", StyleSay)
	position := mathf.NewVec2(1000, 1000)
	context := newSayBubbleLayoutContext(mathf.NewVec2(480, 360), mathf.NewVec2(480, 360), mathf.Vec2{}, mathf.NewVec2(1, 1), 1, true)
	first := context.NewLayout(1, position, mathf.Vec2{}, content)
	clampUIPositionInScreen = false
	second := context.NewLayout(1, position, mathf.Vec2{}, content)
	assertVec2Near(t, "snapshot position", second.position, first.position)
	context.clampPosition = false
	unclamped := context.NewLayout(1, position, mathf.Vec2{}, content)
	assertVec2Near(t, "unclamped position", unclamped.position, position)
	if first.position == unclamped.position {
		t.Fatal("clamping did not constrain the position")
	}
}

func TestResolveSayBubbleLayoutsLargeComponent(t *testing.T) {
	base := make([]SayBubbleLayout, sayBubbleExactLayoutLimit+1)
	component := make([]int, len(base))
	for i := range base {
		base[i] = newSayBubbleLayout(mathf.NewVec2(480, 360), mathf.NewVec2(float64(i*4-24), 0), "crowded bubble", StyleSay, i%2 == 0)
		base[i].stableID = uint64(i + 1)
		base[i].hasPrevious = i%3 != 0
		base[i].previousIsLeft = i%2 != 0
		component[i] = i
	}
	preferred := scoreSayBubbleComponent(base, component)
	var want map[uint64]bool
	for _, reverse := range []bool{false, true} {
		layouts := append([]SayBubbleLayout(nil), base...)
		if reverse {
			slices.Reverse(layouts)
		}
		ResolveSayBubbleLayouts(layouts)
		if score := scoreSayBubbleComponent(layouts, component); preferred.less(score) {
			t.Fatalf("fallback worsened preferred layout: %+v -> %+v", preferred, score)
		}
		got := make(map[uint64]bool, len(layouts))
		for i, layout := range layouts {
			got[layout.stableID] = layout.isLeft
			current := scoreSayBubbleAt(layouts, component, i, layout.isLeft)
			if alternate := scoreSayBubbleAt(layouts, component, i, !layout.isLeft); alternate.less(current) {
				t.Fatalf("bubble %d still has an improving single flip: %+v -> %+v", layout.stableID, current, alternate)
			}
		}
		if want == nil {
			want = got
		} else if !reflect.DeepEqual(got, want) {
			t.Fatalf("input order changed fallback result: %v, want %v", got, want)
		}
		previous := append([]SayBubbleLayout(nil), layouts...)
		ResolveSayBubbleLayouts(layouts)
		if !reflect.DeepEqual(layouts, previous) {
			t.Fatal("resolving the same input changed the layout")
		}
	}
}

func TestSayBubbleLayoutEqualityTracksInputsAndRendering(t *testing.T) {
	base := newSayBubbleLayout(mathf.NewVec2(480, 360), mathf.NewVec2(0, 0), "message", StyleSay, true)
	for _, tc := range []struct {
		name             string
		change           func(*SayBubbleLayout)
		sameInput, equal bool
	}{
		{"unchanged", func(*SayBubbleLayout) {}, true, true},
		{"stable ID", func(l *SayBubbleLayout) { l.stableID++ }, false, true},
		{"position", func(l *SayBubbleLayout) { l.position.X++ }, false, false},
		{"preference", func(l *SayBubbleLayout) { l.preferredIsLeft = !l.preferredIsLeft }, false, true},
		{"direction", func(l *SayBubbleLayout) { l.isLeft = !l.isLeft }, true, false},
		{"message", func(l *SayBubbleLayout) { l.content.formattedMessage += "!" }, false, false},
		{"style", func(l *SayBubbleLayout) { l.content.style++ }, false, false},
		{"extent", func(l *SayBubbleLayout) { l.extent.X++ }, false, true},
		{"scale", func(l *SayBubbleLayout) { l.renderScale.X++ }, false, false},
		{"viewport", func(l *SayBubbleLayout) { l.viewport = sayBubbleRect{} }, false, true},
		{"history", func(l *SayBubbleLayout) { *l = l.WithPreviousDirection(base) }, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := base
			tc.change(&other)
			if got := base.SameInput(other); got != tc.sameInput || other.SameInput(base) != got {
				t.Errorf("SameInput = %v, want symmetric %v", got, tc.sameInput)
			}
			if got := base.Equal(other); got != tc.equal || other.Equal(base) != got {
				t.Errorf("Equal = %v, want symmetric %v", got, tc.equal)
			}
		})
	}
}

func TestSayBubbleRendering(t *testing.T) {
	mgr := setupUIRendering(t)
	node := func(id gdx.Object) *UiNode { n := &UiNode{}; n.SetId(id); return n }
	say := &UiSay{left: sayNodes{node(2), node(3)}, right: sayNodes{node(4), node(5)}, leftThink: sayNodes{node(6), node(7)}, rightThink: sayNodes{node(8), node(9)}}
	say.SetId(1)
	for _, tc := range []struct {
		name        string
		left, think bool
		vbox, label gdx.Object
	}{{"say left", true, false, 2, 3}, {"say right", false, false, 4, 5}, {"think left", true, true, 6, 7}, {"think right", false, true, 8, 9}} {
		t.Run(tc.name, func(t *testing.T) {
			style := StyleSay
			if tc.think {
				style = StyleThink
			}
			layout := SayBubbleLayout{position: mathf.NewVec2(5, 7), renderScale: mathf.NewVec2(2, 3), isLeft: tc.left, content: NewSayBubbleContent("first\nsecond", style)}
			say.SetTextLayout(layout)
			for _, id := range []gdx.Object{2, 4, 6, 8} {
				if mgr.visible[id] != (id == tc.vbox) {
					t.Errorf("node%d visible=%v", id, mgr.visible[id])
				}
			}
			if mgr.text[tc.label] != "first\nsecond" || mgr.positions[1] != mathf.NewVec2(240, 146) || mgr.scales[1] != layout.renderScale {
				t.Fatalf("incorrect rendering: %+v", mgr)
			}
		})
	}
	win, pos, size := mathf.NewVec2(480, 360), mathf.NewVec2(30, 40), mathf.NewVec2(20, 30)
	for _, test := range []struct {
		style int
		scale float64
	}{{StyleSay, 1}, {StyleThink, 0.8}} {
		// With camera zoom 2 and window scale 2, neither the tail nor the
		// already-resolved Think scale should be multiplied a second time.
		layout := NewSayBubbleLayoutContext(win).NewLayout(0, pos, size, NewSayBubbleContent("hello", test.style))
		assertVec2Near(t, "resolved tail anchor", layout.position, mathf.NewVec2(20, 35))
		assertVec2Near(t, "resolved render scale", layout.renderScale, mathf.NewVec2(test.scale, test.scale))
		say.SetTextLayout(layout)
		assertVec2Near(t, "rendered tail anchor", mgr.positions[1], mathf.NewVec2(270, 90))
		assertVec2Near(t, "rendered scale", mgr.scales[1], mathf.NewVec2(test.scale, test.scale))
	}
}

func TestSayBubbleCanonicalOrdering(t *testing.T) {
	for name, change := range map[string]func(*SayBubbleLayout){
		"identity":           func(l *SayBubbleLayout) { l.stableID = 1 },
		"x":                  func(l *SayBubbleLayout) { l.position.X = 1 },
		"y":                  func(l *SayBubbleLayout) { l.position.Y = 1 },
		"width":              func(l *SayBubbleLayout) { l.extent.X = 1 },
		"height":             func(l *SayBubbleLayout) { l.extent.Y = 1 },
		"direction":          func(l *SayBubbleLayout) { l.preferredIsLeft = true },
		"style":              func(l *SayBubbleLayout) { l.content.style = 1 },
		"message":            func(l *SayBubbleLayout) { l.content.formattedMessage = "a" },
		"history":            func(l *SayBubbleLayout) { l.hasPrevious = true },
		"previous direction": func(l *SayBubbleLayout) { l.previousIsLeft = true },
	} {
		t.Run(name, func(t *testing.T) {
			a, b := SayBubbleLayout{}, SayBubbleLayout{}
			change(&b)
			if !sayBubbleCanonicalLess(a, b) || sayBubbleCanonicalLess(b, a) || sayBubbleCanonicalLess(b, b) {
				t.Fatal("canonical comparison violates strict ordering")
			}
		})
	}
	for _, tc := range []struct {
		a, b []bool
		want bool
	}{{nil, nil, false}, {[]bool{false, true}, []bool{true, false}, true}, {[]bool{true}, []bool{false}, false}, {[]bool{false, true}, []bool{false, true}, false}} {
		if got := sayBubbleDirectionsLess(tc.a, tc.b); got != tc.want {
			t.Errorf("directions %v < %v =%v", tc.a, tc.b, got)
		}
	}
}

func TestDecorativeUIRenderingUsesViewportAndScale(t *testing.T) {
	mgr := setupUIRendering(t)
	node := func(id engine.Object) *UiNode { n := &UiNode{}; n.SetId(id); return n }
	debug := &UiDebug{input: node(2)}
	debug.SetId(1)
	for _, text := range []string{"diagnostic", ""} {
		debug.Show(text)
		if mgr.visible[2] != (text != "") || mgr.text[2] != text || mgr.scales[1] != mathf.NewVec2(2, 2) {
			t.Fatal("debug panel did not update visibility, text and scale")
		}
	}
	measure := &UiMeasure{container: node(3), imageLine: node(4), labelContainer: node(5), labelValue: node(6)}
	measure.SetId(7)
	color := mathf.NewColorRGBAi(255, 0, 0, 255)
	measure.UpdateInfo(mathf.NewVec2(5, 7), 10, 90, "10", color)
	if mgr.positions[3] != mathf.NewVec2(233, 146) || mgr.positions[5] != mathf.NewVec2(240, 146) || mgr.sizes[3] != mathf.NewVec2(18, 26) || mgr.rotations[3] != 0 || mgr.colors[3] != color || mgr.colors[5] != color || mgr.text[6] != "10" || mgr.scales[7] != mathf.NewVec2(2, 2) {
		t.Fatalf("measurement rendering mismatch:%+v", mgr)
	}
	quote := &UiQuote{container: node(8), labelMsg: node(9), labelDes: node(10)}
	quote.SetId(11)
	quote.SetText(mathf.NewVec2(30, 40), mathf.NewVec2(5, 6), "message", "description")
	if mgr.positions[8] != mathf.NewVec2(260, 108) || mgr.sizes[8] != mathf.NewVec2(10, 12) || mgr.text[9] != "message" || mgr.text[10] != "description" || mgr.scales[11] != mathf.NewVec2(2, 2) {
		t.Fatalf("quote rendering mismatch:%+v", mgr)
	}
}

func (m *uiRenderSpy) SetPosition(id gdx.Object, v mathf.Vec2) { m.positions[id] = v }

func (m *uiRenderSpy) SetScale(id gdx.Object, v mathf.Vec2) { m.scales[id] = v }

func (m *uiRenderSpy) SetGlobalPosition(id gdx.Object, v mathf.Vec2) { m.positions[id] = v }

func (m *uiRenderSpy) SetSize(id gdx.Object, v mathf.Vec2) { m.sizes[id] = v }

func (m *uiRenderSpy) SetColor(id gdx.Object, v mathf.Color) { m.colors[id] = v }

func (m *uiRenderSpy) SetRotation(id gdx.Object, v float64) { m.rotations[id] = v }

func (uiRenderCamera) GetViewportRect() mathf.Rect2 {
	return mathf.Rect2{Position: mathf.NewVec2(10, 20), Size: mathf.NewVec2(480, 360)}
}

func (uiRenderCamera) GetCameraPosition() mathf.Vec2 { return mathf.NewVec2(10, 20) }

func (uiRenderCamera) GetCameraZoom() mathf.Vec2 { return mathf.NewVec2(2, 2) }

func assertVec2Near(t *testing.T, name string, got, want mathf.Vec2) {
	t.Helper()
	const epsilon = 1e-9
	if math.Abs(float64(got.X-want.X)) > epsilon || math.Abs(float64(got.Y-want.Y)) > epsilon {
		t.Fatalf("%s = %v, want %v", name, got, want)
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

func setupUIRendering(t *testing.T) *uiRenderSpy {
	t.Helper()
	oldUI, oldCamera := gdx.UiMgr, gdx.CameraMgr
	oldScale := engine.WindowScale()
	oldWidth, oldHeight, oldClamp := baseScreenWidth, baseScreenHeight, clampUIPositionInScreen
	enginewrap.Init(func(f func()) { f() })
	mgr := &uiRenderSpy{askShowUiMgr: askShowUiMgr{visible: map[gdx.Object]bool{}, text: map[gdx.Object]string{}}, positions: map[gdx.Object]mathf.Vec2{}, scales: map[gdx.Object]mathf.Vec2{}, sizes: map[gdx.Object]mathf.Vec2{}, colors: map[gdx.Object]mathf.Color{}, rotations: map[gdx.Object]float64{}}
	gdx.UiMgr, gdx.CameraMgr = mgr, uiRenderCamera{}
	engine.SetWindowScale(2)
	SetBaseScreenSize(480, 360)
	ClampUIPositionInScreen(true)
	t.Cleanup(func() {
		gdx.UiMgr, gdx.CameraMgr = oldUI, oldCamera
		engine.SetWindowScale(oldScale)
		SetBaseScreenSize(oldWidth, oldHeight)
		ClampUIPositionInScreen(oldClamp)
		enginewrap.Init(engine.WaitMainThread)
	})
	return mgr
}
