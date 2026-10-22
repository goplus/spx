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
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

func newTestTransformSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{
		g:    &Game{},
		name: "TestSprite",
	}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		X:             x,
		Y:             y,
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	return sprite
}

func TestMotionInputsTreatNaNAsZero(t *testing.T) {
	tests := []struct {
		name         string
		apply        func(*SpriteImpl)
		wantX, wantY float64
	}{
		{
			name:  "set position",
			apply: func(sprite *SpriteImpl) { sprite.SetXYpos(math.NaN(), math.NaN()) },
			wantX: 0,
			wantY: 0,
		},
		{
			name:  "set x",
			apply: func(sprite *SpriteImpl) { sprite.SetXpos(math.NaN()) },
			wantX: 0,
			wantY: -8,
		},
		{
			name:  "set y",
			apply: func(sprite *SpriteImpl) { sprite.SetYpos(math.NaN()) },
			wantX: 12,
			wantY: 0,
		},
		{
			name:  "change position",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXYpos(math.NaN(), math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change x",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change y",
			apply: func(sprite *SpriteImpl) { sprite.ChangeYpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(12, -8)
			tt.apply(sprite)

			if gotX, gotY := sprite.getXY(); gotX != tt.wantX || gotY != tt.wantY {
				t.Fatalf("position = (%v, %v), want (%v, %v)", gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestScratchImmediateGlide(t *testing.T) {
	for _, tt := range []struct {
		name         string
		duration     float64
		x, y         float64
		wantX, wantY float64
	}{
		{name: "zero", duration: 0, x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "negative", duration: -1, x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "NaN duration", duration: math.NaN(), x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "NaN x", x: math.NaN(), y: 40, wantX: 0, wantY: 40},
		{name: "NaN y", x: 30, y: math.NaN(), wantX: 30, wantY: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			co := setupRuntimeScheduler(t)
			sprite := newTestTransformSprite(12, -8)
			finished := false
			thread := co.Create(sprite, func(coroutine.Thread) {
				sprite.GlideToXYpos(tt.x, tt.y, tt.duration)
				finished = true
			})
			co.JoinFirstSlice(thread)
			if !finished || sprite.Xpos() != tt.wantX || sprite.Ypos() != tt.wantY {
				t.Fatalf("immediate glide: finished=%v position=(%v,%v)", finished, sprite.Xpos(), sprite.Ypos())
			}
			if sprite.animation().getCurTweenState() != nil {
				t.Fatal("immediate glide left an active tween")
			}
		})
	}
}

func TestSpriteGlidePositiveDurationInterpolates(t *testing.T) {
	itime.Start(nil)
	t.Cleanup(func() { itime.Start(nil) })
	co := setupRuntimeScheduler(t)
	sprite := newTestTransformSprite(12, -8)
	finished := false
	itime.Update(0.5, 30)
	thread := co.Create(sprite, func(coroutine.Thread) {
		sprite.GlideToXYpos(30, 40, 1)
		finished = true
	})
	co.Update()
	if finished || sprite.Xpos() != 21 || sprite.Ypos() != 16 {
		t.Fatalf("halfway glide: finished=%v position=(%v,%v)", finished, sprite.Xpos(), sprite.Ypos())
	}
	if sprite.animation().getCurTweenState() == nil {
		t.Fatal("positive-duration glide has no active tween")
	}
	itime.Update(0.5, 30)
	co.Update()
	if sprite.Xpos() != 30 || sprite.Ypos() != 40 {
		t.Fatalf("final glide position=(%v,%v), want (30,40)", sprite.Xpos(), sprite.Ypos())
	}
	itime.Update(0, 30)
	co.Update()
	if !finished || sprite.animation().getCurTweenState() != nil {
		t.Fatal("glide did not finish and release its tween after the final frame")
	}
	co.Join(thread)
}

func TestSpriteDirectionToPosNormalizesAngle(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	tests := []struct {
		name string
		x    float64
		y    float64
		want Direction
	}{
		{name: "right", x: 10, y: 0, want: 90},
		{name: "up", x: 0, y: 10, want: 0},
		{name: "left", x: -10, y: 0, want: -90},
		{name: "down", x: 0, y: -10, want: 180},
		{name: "bottom left stays normalized", x: -10, y: -10, want: -135},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.DirectionTo__4(tt.x, tt.y); !(math.Abs(got-tt.want) <= 1e-9) {
				t.Fatalf("DirectionTo__4(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestSpriteTurnToXYposFacesCoordinateTarget(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	sprite.TurnToXYpos(-10, -10, nil)

	if got := sprite.Heading(); got != -135 {
		t.Fatalf("Heading() = %v, want -135", got)
	}
}

func TestSpriteHeadingMultipleRevolutions(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.SetHeading(1080.5)
	if got := sprite.Heading(); got != 0.5 {
		t.Fatalf("SetHeading: got %v, want 0.5", got)
	}
	sprite.ChangeHeading(-720.5)
	if got := sprite.Heading(); got != 0 {
		t.Fatalf("ChangeHeading: got %v, want 0", got)
	}
	version := sprite.spriteState.DirtyVersion
	sprite.SetHeading(1080)
	if sprite.spriteState.DirtyVersion != version {
		t.Fatal("equivalent heading dirtied the sprite proxy")
	}
}

func TestSpriteHeadingIgnoresNonFiniteInputs(t *testing.T) {
	for _, input := range []struct {
		name  string
		value float64
	}{
		{"NaN", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
	} {
		for _, action := range []struct {
			name  string
			apply func(*SpriteImpl, Direction)
		}{
			{"set", (*SpriteImpl).SetHeading},
			{"change", (*SpriteImpl).ChangeHeading},
		} {
			t.Run(action.name+"/"+input.name, func(t *testing.T) {
				sprite := newTestTransformSprite(0, 0)
				sprite.SetHeading(45)
				dirty, visual := sprite.spriteState.DirtyVersion, sprite.spriteState.VisualVersion

				action.apply(sprite, input.value)

				if sprite.Heading() != 45 || sprite.spriteState.DirtyVersion != dirty ||
					sprite.spriteState.VisualVersion != visual {
					t.Fatalf("invalid input changed heading or versions: heading=%v dirty=%v visual=%v",
						sprite.Heading(), sprite.spriteState.DirtyVersion, sprite.spriteState.VisualVersion)
				}
				sprite.ChangeHeading(405)
				if sprite.Heading() != 90 {
					t.Fatalf("heading after valid change = %v, want 90", sprite.Heading())
				}
			})
		}
	}
}

func TestSpriteTurnIgnoresNonFiniteInputs(t *testing.T) {
	for _, animation := range []string{"without animation", "with animation"} {
		for _, input := range []struct {
			name  string
			value float64
		}{
			{"NaN", math.NaN()},
			{"positive infinity", math.Inf(1)},
			{"negative infinity", math.Inf(-1)},
		} {
			for _, action := range []struct {
				name  string
				apply func(*SpriteImpl, Direction)
			}{
				{"turn", (*SpriteImpl).Turn__0},
				{"turn to", (*SpriteImpl).TurnTo__2},
			} {
				t.Run(animation+"/"+action.name+"/"+input.name, func(t *testing.T) {
					co := setupRuntimeScheduler(t)
					sprite := newTestTransformSprite(0, 0)
					sprite.SetHeading(45)
					version := sprite.spriteState.DirtyVersion
					if animation == "with animation" {
						sprite.animation().shared.animations[StateTurn] = &animationEntry{
							name: StateTurn, config: &coreproject.AniConfig{TurnToDuration: 1},
						}
					}
					finished := false
					thread := co.Create(sprite, func(coroutine.Thread) {
						action.apply(sprite, input.value)
						finished = true
					})
					co.JoinFirstSlice(thread)
					if !finished || sprite.Heading() != 45 || sprite.spriteState.DirtyVersion != version ||
						sprite.animation().getCurTweenState() != nil {
						t.Fatalf("invalid turn: finished=%v heading=%v dirty=%v tween=%v",
							finished, sprite.Heading(), sprite.spriteState.DirtyVersion, sprite.animation().getCurTweenState())
					}
					co.Join(thread)
				})
			}
		}
	}
}

func TestSpriteTurnToDirMultipleRevolutionsUsesShortestAnimation(t *testing.T) {
	t.Cleanup(func() { itime.Start(nil) })
	co := setupRuntimeScheduler(t)
	sprite := newTestTransformSprite(0, 0)
	sprite.SetHeading(-10)
	sprite.animation().shared.animations[StateTurn] = &animationEntry{
		name:   StateTurn,
		config: &coreproject.AniConfig{TurnToDuration: 360},
	}

	// At one degree per second, the shortest turn passes through zero after
	// ten seconds and reaches the target after twenty.
	itime.Update(10, 30)
	returned := false
	thread := co.Create(sprite, func(coroutine.Thread) {
		sprite.TurnToDir(-710, nil)
		returned = true
	})
	co.Update()
	if got := sprite.Heading(); got != 0 {
		t.Fatalf("halfway heading = %v, want 0", got)
	}
	itime.Update(10, 30)
	co.Update()
	if got := sprite.Heading(); got != 10 {
		t.Fatalf("final heading = %v, want 10", got)
	}
	itime.Update(0, 30)
	co.Update()
	if !returned {
		t.Fatal("TurnToDir did not return after its final animation frame")
	}
	co.Join(thread)
}

func TestMotionOptions(t *testing.T) {
	tests := []struct {
		name          string
		opts          *MotionOptions
		wantSpeed     Speed
		wantAnimation SpriteAnimationName
	}{
		{
			name:      "nil uses defaults",
			opts:      nil,
			wantSpeed: 1,
		},
		{
			name: "zero speed keeps default and animation",
			opts: &MotionOptions{
				Animation: "walk",
			},
			wantSpeed:     1,
			wantAnimation: "walk",
		},
		{
			name: "positive speed overrides default",
			opts: &MotionOptions{
				Speed:     2.5,
				Animation: "run",
			},
			wantSpeed:     2.5,
			wantAnimation: "run",
		},
		{
			name: "negative speed falls back to default",
			opts: &MotionOptions{
				Speed:     -3,
				Animation: "run",
			},
			wantSpeed:     1,
			wantAnimation: "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpeed, gotAnimation := motionOptions(tt.opts)
			if gotSpeed != tt.wantSpeed {
				t.Fatalf("speed = %v, want %v", gotSpeed, tt.wantSpeed)
			}
			if gotAnimation != tt.wantAnimation {
				t.Fatalf("animation = %q, want %q", gotAnimation, tt.wantAnimation)
			}
		})
	}
}

func TestSpriteStepToRandomUsesRandomPositionTarget(t *testing.T) {
	sprite := newTestTransformSprite(1000, 1000)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360

	SetRandomSeed(1)
	defer ResetRandomSeed()
	sprite.StepTo__c(Random)

	gotX, gotY := sprite.getXY()
	if gotX == 1000 && gotY == 1000 {
		t.Fatal("StepTo__c(Random) did not move the sprite")
	}

	minX := float64(-(sprite.g.displayState.WorldWidth >> 1))
	maxX := float64(sprite.g.displayState.WorldWidth - (sprite.g.displayState.WorldWidth >> 1) - 1)
	minY := float64((sprite.g.displayState.WorldHeight >> 1) - (sprite.g.displayState.WorldHeight - 1))
	maxY := float64(sprite.g.displayState.WorldHeight >> 1)
	if gotX < minX || gotX > maxX || gotY < minY || gotY > maxY {
		t.Fatalf(
			"StepTo__c(Random) moved to (%v, %v), want x in [%v, %v], y in [%v, %v]",
			gotX,
			gotY,
			minX,
			maxX,
			minY,
			maxY,
		)
	}
}

func TestFixWorldRange(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		scale          float64
		heading        Direction
		startX, startY float64
		x, y           float64
		wantX, wantY   float64
	}{
		{
			name:  "oversized stays centered",
			width: 480, height: 360, scale: 3,
			startY: 45, y: 45,
			wantY: 45,
		},
		{
			name:  "normal sprite is fenced",
			width: 100, height: 80, scale: 1, heading: 90,
			x: 500, y: 300,
			wantX: 275, wantY: 205,
		},
		{
			name:  "oversized is fenced when fully outside",
			width: 480, height: 360, scale: 3,
			startY: 45, x: 1000, y: 45, heading: 90,
			wantX: 945, wantY: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(tt.startX, tt.startY)
			sprite.g.displayState.WorldWidth = 480
			sprite.g.displayState.WorldHeight = 360
			sprite.g.displayState.MinWorldX = -240
			sprite.g.displayState.MinWorldY = -180
			sprite.costumes = []*costume{{
				width:            tt.width,
				height:           tt.height,
				bitmapResolution: 1,
				center:           mathf.NewVec2(float64(tt.width)/2, float64(tt.height)/2),
			}}
			sprite.costumeIndex = 0
			sprite.runtimeState.Scale = tt.scale
			sprite.transform().direction = tt.heading

			x, y := sprite.transform().fixWorldRange(tt.x, tt.y)
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("fixWorldRange(%v, %v) = (%v, %v), want (%v, %v)", tt.x, tt.y, x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestFixWorldRangeUsesRenderedCostumeBoundsInsteadOfAutoTrigger(t *testing.T) {
	sprite := newTestTransformSprite(-186, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{{
		width:            55,
		height:           54,
		bitmapResolution: 2,
		center:           mathf.NewVec2(52, 89),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.transform().direction = 90
	sprite.physics().triggerInfo.Type = physicsColliderAuto
	sprite.physics().triggerInfo.Pivot = mathf.NewVec2(0.25, 0)
	sprite.physics().triggerInfo.Params = []float64{29.5, 29}

	gotX, gotY := sprite.transform().fixWorldRange(1000, 0)
	if gotX != 253 || gotY != 0 {
		t.Fatalf("fixWorldRange(1000, 0) = (%v, %v), want (253, 0)", gotX, gotY)
	}
}

func TestFenceBoundsIncludesCostumeRotation(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.baseObj.costumes = []*costume{{
		width:            100,
		height:           40,
		bitmapResolution: 1,
		center:           mathf.NewVec2(50, 20),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.transform().direction = 0

	got := sprite.fenceBounds()
	if got == nil {
		t.Fatal("fenceBounds() = nil")
	}
	if !(math.Abs(got.Position.X+20) <= 1e-9 && math.Abs(got.Position.Y+50) <= 1e-9 &&
		math.Abs(got.Size.X-40) <= 1e-9 && math.Abs(got.Size.Y-100) <= 1e-9) {
		t.Fatalf("fenceBounds() = %+v, want position (-20, -50), size (40, 100)", *got)
	}
}

func TestClampSpriteScaleToMaxScale(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.baseObj.costumes = []*costume{{
		width:            480,
		height:           360,
		bitmapResolution: 1,
		center:           mathf.NewVec2(240, 180),
	}}
	sprite.baseObj.costumeIndex = 0

	got := sprite.transform().clampSpriteScale(3)

	if got != 1.5 {
		t.Fatalf("clampSpriteScale(3) = %v, want 1.5", got)
	}
}

func TestFractionalCostumeFenceUsesLogicalGeometry(t *testing.T) {
	sprite := newTestTransformSprite(-393, 5)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{newCostume(&coreproject.CostumeConfig{
		ImageWidth: 247.24325, ImageHeight: 476.42829,
		X: 76.0323911558041, Y: 208.79720845345383,
		BitmapResolution: 1,
	})}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.transform().direction = 90
	sprite.transform().changeX(-5)
	if got := sprite.Xpos(); got != -396 {
		t.Fatalf("fenced X = %v, want -396", got)
	}
}

func TestNormalizeDirection(t *testing.T) {
	for _, tt := range []struct {
		input, want float64
	}{
		{0, 0}, {90, 90}, {-90, -90}, {180, 180}, {-180, 180},
		{181, -179}, {-181, 179}, {360, 0}, {-360, 0},
		{540, 180}, {-540, 180}, {1080, 0}, {-1080, 0},
		{0.5, 0.5}, {-0.5, -0.5}, {720.5, 0.5}, {-720.5, -0.5},
		{1080.5, 0.5}, {-1080.5, -0.5},
		{math.MaxFloat64, 128}, {-math.MaxFloat64, -128},
		{math.Inf(1), math.Inf(1)}, {math.Inf(-1), math.Inf(-1)},
	} {
		got := normalizeDirection(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDirection(%v) = %v, want %v", tt.input, got, tt.want)
		}
		if again := normalizeDirection(got); again != got {
			t.Errorf("normalization is not idempotent: %v -> %v -> %v", tt.input, got, again)
		}
	}
	if got := normalizeDirection(math.NaN()); !math.IsNaN(got) {
		t.Errorf("normalizeDirection(NaN) = %v, want NaN", got)
	}
}

func TestNormalizeDirectionPeriodicity(t *testing.T) {
	for _, angle := range []float64{-180, -90.5, 0, 90.5, 180} {
		want := normalizeDirection(angle)
		for turns := -10; turns <= 10; turns++ {
			input := angle + float64(turns)*fullCircleDegrees
			if got := normalizeDirection(input); got != want {
				t.Errorf("normalizeDirection(%v) = %v, want %v", input, got, want)
			}
		}
	}
}

func TestNormalizeAngleRangePreservesShortestPathAndHalfTurnDirection(t *testing.T) {
	for _, tt := range []struct {
		from, to, wantDelta float64
	}{
		{-10, -710, 20}, {10, 710, -20},
		{-730, 730, 20}, {730, -730, -20},
		{90, -90, 180}, {-90, 90, -180},
		{0, -180, 180}, {180, 0, -180},
		{-180, 540, 0},
		{0, 1e-14, 1e-14},
		{math.MaxFloat64, -math.MaxFloat64, 104},
		{-math.MaxFloat64, math.MaxFloat64, -104},
	} {
		from, to := normalizeAngleRange(tt.from, tt.to)
		if delta := to - from; delta != tt.wantDelta {
			t.Errorf("normalizeAngleRange(%v, %v) turns %v degrees, want %v", tt.from, tt.to, delta, tt.wantDelta)
		}
	}
	for _, angle := range []float64{math.Inf(-1), math.Inf(1), math.NaN()} {
		from, to := normalizeAngleRange(angle, 90)
		if !math.IsNaN(from) || to != 90 {
			t.Errorf("normalizeAngleRange(%v, 90) = (%v, %v), want (NaN, 90)", angle, from, to)
		}
		from, to = normalizeAngleRange(90, angle)
		if from != 90 || !math.IsNaN(to) {
			t.Errorf("normalizeAngleRange(90, %v) = (%v, %v), want (90, NaN)", angle, from, to)
		}
	}
}

func TestSpriteMotionIgnoresMissingTarget(t *testing.T) {
	for _, action := range []struct {
		name string
		run  func(*cloneLimitSprite)
	}{
		{"step", func(sprite *cloneLimitSprite) { sprite.StepTo__1("missing") }},
		{"glide zero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 0) }},
		{"glide nonzero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 60) }},
		{"turn", func(sprite *cloneLimitSprite) { sprite.TurnTo__1("missing") }},
	} {
		t.Run(action.name, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			sprite := newCloneLimitSprite(game, "source")
			sprite.transform().x, sprite.transform().y, sprite.transform().direction = 30, 40, 60
			dirtyVersion := sprite.spriteState.DirtyVersion
			finished := false
			thread := gco.Create(sprite, func(coroutine.Thread) {
				action.run(sprite)
				finished = true
			})
			gco.JoinFirstSlice(thread)

			if !finished || sprite.Xpos() != 30 || sprite.Ypos() != 40 || sprite.Heading() != 60 || sprite.spriteState.DirtyVersion != dirtyVersion {
				t.Fatalf("missing target altered sprite: finished=%v position=(%v,%v) heading=%v dirty=%v", finished, sprite.Xpos(), sprite.Ypos(), sprite.Heading(), sprite.spriteState.DirtyVersion)
			}
		})
	}
}

func TestSpriteTurnWithoutAnimationFinishesImmediately(t *testing.T) {
	for _, mode := range []struct {
		name  string
		debug bool
	}{
		{"normal", false},
		{"debug instructions", true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, action := range []struct {
				name string
				run  func(*cloneLimitSprite)
			}{
				{"named target", func(sprite *cloneLimitSprite) { sprite.TurnToTarget("target", nil) }},
				{"coordinates", func(sprite *cloneLimitSprite) { sprite.TurnToXYpos(-10, -10, nil) }},
				{"direction", func(sprite *cloneLimitSprite) { sprite.TurnToDir(945, nil) }},
			} {
				t.Run(action.name, func(t *testing.T) {
					game := setupCloneLimitGame(t)
					originalDebug := game.debugState.DebugInstr
					game.debugState.DebugInstr = mode.debug
					t.Cleanup(func() { game.debugState.DebugInstr = originalDebug })
					sprite := newCloneLimitSprite(game, "source")
					sprite.SetHeading(60)
					target := newCloneLimitSprite(game, "target")
					target.SetXYpos(-10, -10)
					wantDirty := sprite.spriteState.DirtyVersion + 1

					// Both the initial turn and the unchanged heading must return
					// without waiting; only the initial turn publishes a change.
					for attempt := 0; attempt < 2; attempt++ {
						finished := false
						thread := gco.Create(sprite, func(coroutine.Thread) {
							action.run(sprite)
							finished = true
						})
						gco.JoinFirstSlice(thread)
						if !finished {
							t.Fatalf("turn %d yielded without an animation", attempt+1)
						}
						gco.Join(thread)
						if sprite.Heading() != -135 || sprite.spriteState.DirtyVersion != wantDirty {
							t.Fatalf("turn %d: heading=%v dirty=%v, want -135 and %v", attempt+1, sprite.Heading(), sprite.spriteState.DirtyVersion, wantDirty)
						}
						if sprite.Xpos() != 0 || sprite.Ypos() != 0 {
							t.Fatalf("turn moved sprite to (%v, %v)", sprite.Xpos(), sprite.Ypos())
						}
					}
				})
			}
		})
	}
}

func TestSpriteDistanceToMissingTarget(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	if got := sprite.DistanceTo__1("missing"); got != 10000 {
		t.Fatalf("DistanceTo__1(missing) = %v, want 10000", got)
	}
}

func TestSpriteNamedTargetIgnoresClones(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().x, sprite.transform().y = 30, 40
	target := newCloneLimitSprite(game, "target")
	clone := createRuntimeClone(&target.SpriteImpl)
	clone.transform().x = 300

	if got := sprite.DistanceTo__1("target"); got != 50 {
		t.Fatalf("DistanceTo__1(target) = %v, want 50", got)
	}
}

func TestSpriteNilTargetIsMissing(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	var target *cloneLimitSprite
	if got := sprite.DistanceTo__0(target); got != 10000 {
		t.Fatalf("DistanceTo__0(nil) = %v, want 10000", got)
	}
}

func TestSpriteDirectionToMissingTargetKeepsHeading(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().direction = 60
	if got := sprite.DirectionTo__1("missing"); got != 60 {
		t.Fatalf("DirectionTo__1(missing) = %v, want 60", got)
	}
}

func TestSpriteMotionOverloadsKeepTargetAndOptionSemantics(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	game.displayState.WorldWidth, game.displayState.WorldHeight = 1, 1
	target := newCloneLimitSprite(game, "target")
	sprite := newCloneLimitSprite(game, "source")
	// A one-pixel world makes Random deterministic without changing the RNG.
	// Remove costumes so movement is not fenced by that small test world.
	sprite.costumes = nil
	options := &MotionOptions{Speed: 2, Animation: "missing"}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"step sprite", func() { sprite.StepTo__0(target) }},
		{"step mouse", func() { sprite.StepTo__2(Mouse) }},
		{"step xy", func() { sprite.StepTo__3(0, 0) }},
		{"step sprite speed", func() { sprite.StepTo__4(target, 2) }},
		{"step name speed", func() { sprite.StepTo__5("target", 2) }},
		{"step mouse speed", func() { sprite.StepTo__6(Mouse, 2) }},
		{"step xy speed", func() { sprite.StepTo__7(0, 0, 2) }},
		{"step sprite animation", func() { sprite.StepTo__8(target, 2, "missing") }},
		{"step name animation", func() { sprite.StepTo__9("target", 2, "missing") }},
		{"step mouse animation", func() { sprite.StepTo__a(Mouse, 2, "missing") }},
		{"step xy animation", func() { sprite.StepTo__b(0, 0, 2, "missing") }},
		{"step random speed", func() { sprite.StepTo__d(Random, 2) }},
		{"step random animation", func() { sprite.StepTo__e(Random, 2, "missing") }},
		{"step target options", func() { sprite.StepToTarget(target, options) }},
		{"step xy options", func() { sprite.StepToXYpos(0, 0, options) }},
		{"glide sprite", func() { sprite.Glide__0(target, 0) }},
		{"glide mouse", func() { sprite.Glide__2(Mouse, 0) }},
		{"glide random", func() { sprite.Glide__3(Random, 0) }},
		{"glide xy", func() { sprite.Glide__4(0, 0, 0) }},
		{"glide target", func() { sprite.GlideToTarget(target, 0) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(-10, -20)
			tt.run()
			if sprite.Xpos() != 0 || sprite.Ypos() != 0 {
				t.Fatalf("position = (%v, %v), want origin", sprite.Xpos(), sprite.Ypos())
			}
		})
	}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"move float", func() { sprite.Move__0(2) }},
		{"move integer", func() { sprite.Move__1(2) }},
		{"step speed", func() { sprite.Step__1(2, 3) }},
		{"step animation", func() { sprite.Step__2(2, 3, "missing") }},
		{"step options", func() { sprite.StepWith(2, options) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(0, 0)
			sprite.SetHeading(90)
			tt.run()
			if !(math.Abs(sprite.Xpos()-2) <= 1e-12 && math.Abs(sprite.Ypos()) <= 1e-12) {
				t.Fatalf("forward movement = (%v, %v), want (2, 0)", sprite.Xpos(), sprite.Ypos())
			}
		})
	}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"turn", func() { sprite.Turn__0(90) }},
		{"turn speed", func() { sprite.Turn__1(90, 2) }},
		{"turn animation", func() { sprite.Turn__2(90, 2, "missing") }},
		{"turn options", func() { sprite.TurnWith(90, options) }},
		{"turn sprite", func() { sprite.TurnTo__0(target) }},
		{"turn direction", func() { sprite.TurnTo__2(90) }},
		{"turn mouse", func() { sprite.TurnTo__3(Mouse) }},
		{"turn sprite speed", func() { sprite.TurnTo__4(target, 2) }},
		{"turn name speed", func() { sprite.TurnTo__5("target", 2) }},
		{"turn direction speed", func() { sprite.TurnTo__6(90, 2) }},
		{"turn mouse speed", func() { sprite.TurnTo__7(Mouse, 2) }},
		{"turn sprite animation", func() { sprite.TurnTo__8(target, 2, "missing") }},
		{"turn name animation", func() { sprite.TurnTo__9("target", 2, "missing") }},
		{"turn direction animation", func() { sprite.TurnTo__a(90, 2, "missing") }},
		{"turn mouse animation", func() { sprite.TurnTo__b(Mouse, 2, "missing") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(-10, 0)
			sprite.SetHeading(0)
			tt.run()
			if sprite.Heading() != 90 {
				t.Fatalf("heading = %v, want 90", sprite.Heading())
			}
		})
	}
	sprite.SetXYpos(0, -10)
	for name, got := range map[string]float64{"mouse": sprite.DistanceTo__2(Mouse), "random": sprite.DistanceTo__3(Random), "target": sprite.DistanceToWith(target)} {
		if got != 10 {
			t.Errorf("%s distance = %v, want 10", name, got)
		}
	}
	for name, got := range map[string]float64{"sprite": sprite.DirectionTo__0(target), "mouse": sprite.DirectionTo__2(Mouse), "random": sprite.DirectionTo__3(Random), "target": sprite.DirectionToWith(target)} {
		if got != 0 {
			t.Errorf("%s direction = %v, want 0", name, got)
		}
	}
	sprite.SetSize(2)
	sprite.ChangeSize(3)
	if sprite.Size() != 5 {
		t.Fatalf("size = %v, want 5", sprite.Size())
	}
}
