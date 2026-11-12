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
	"encoding/json"
	"fmt"
	"maps"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/core/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	tm "github.com/goplus/spx/v4/internal/core/tilemap"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/ui"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	spxapi "github.com/goplus/spx/v4/pkg/spx"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
}

// TestMain supplies the raw manager normally installed by gdengine.Link.
func TestMain(m *testing.M) {
	gdx.PlatformMgr = mainThreadTestPlatform{}
	os.Exit(m.Run())
}

func TestApplyRuntimeConfig(t *testing.T) {
	t.Setenv("SPX_PROJECT_DIR", filepath.Join(t.TempDir(), "my-project"))
	for _, tt := range []struct {
		name       string
		config     *Config
		project    coreproject.ProjectConfig
		wantTitle  string
		wantKey    string
		wantWidth  int
		wantHeight int
		wantFull   bool
	}{
		{name: "defaults reset previous config", wantTitle: "my-project (by spx)", wantKey: "F8"},
		{
			name: "explicit settings", config: &Config{Title: "Game", Width: 640, Height: 360, FullScreen: true, ScreenshotKey: "F9"},
			project: coreproject.ProjectConfig{Physics: true}, wantTitle: "Game", wantKey: "F9", wantWidth: 640, wantHeight: 360, wantFull: true,
		},
		{
			name: "project fullscreen and environment key", config: &Config{Width: 480, Height: 320},
			project: coreproject.ProjectConfig{FullScreen: true}, wantTitle: "my-project (by spx)", wantKey: "F8", wantWidth: 480, wantHeight: 320, wantFull: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SPX_SCREENSHOT_KEY", "F8")
			game := &Game{}
			game.displayState.WindowWidth, game.displayState.WindowHeight = 99, 88
			var original Config
			if tt.config != nil {
				original = *tt.config
			}
			title := game.applyRuntimeConfig(tt.config, &tt.project)
			if title != tt.wantTitle || game.displayState.WindowWidth != tt.wantWidth || game.displayState.WindowHeight != tt.wantHeight {
				t.Fatalf("runtime config = title %q, window %dx%d; want %q, %dx%d", title, game.displayState.WindowWidth, game.displayState.WindowHeight, tt.wantTitle, tt.wantWidth, tt.wantHeight)
			}
			if tt.project.FullScreen != tt.wantFull || game.physicsEnabled != tt.project.Physics {
				t.Fatalf("fullscreen/physics = %v/%v, want %v/%v", tt.project.FullScreen, game.physicsEnabled, tt.wantFull, tt.project.Physics)
			}
			if got := os.Getenv("SPX_SCREENSHOT_KEY"); got != tt.wantKey {
				t.Fatalf("screenshot key = %q, want %q", got, tt.wantKey)
			}
			if tt.config != nil && *tt.config != original {
				t.Fatalf("caller config mutated: got %+v, want %+v", *tt.config, original)
			}
		})
	}
}

func TestRuntimeConfigTitleFallsBackToWorkingDirectory(t *testing.T) {
	t.Setenv("SPX_PROJECT_DIR", "")
	t.Setenv("SPX_SCREENSHOT_KEY", "")
	dir := t.TempDir()
	t.Chdir(dir)
	got := new(Game).applyRuntimeConfig(nil, &coreproject.ProjectConfig{})
	if want := filepath.Base(dir) + " (by spx)"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestResolveTargetPosition(t *testing.T) {
	game := &Game{}
	game.initShapeMgr()
	game.inputMgr.init(game)
	game.inputMgr.mousePos = mathf.Vec2{X: 17, Y: -23}
	original := newTargetLookupSprite(game, "actor", false)
	original.components.transform = &transformComponent{sprite: &original.SpriteImpl, x: 12, y: -34}
	clone := newTargetLookupSprite(game, "actor", true)
	clone.components.transform = &transformComponent{sprite: &clone.SpriteImpl, x: 56, y: 78}
	game.shapeMgr.add(&clone.SpriteImpl)
	game.shapeMgr.add(&original.SpriteImpl)
	var nilSprite *targetLookupSprite
	for _, tt := range []struct {
		name   string
		target Target
		x, y   float64
		found  bool
	}{
		{"named original", "actor", 12, -34, true},
		{"direct clone", clone, 56, 78, true},
		{"mouse", Mouse, 17, -23, true},
		{"missing name", "absent", 0, 0, false},
		{"nil", nil, 0, 0, false},
		{"typed nil sprite", nilSprite, 0, 0, false},
		{"unsupported edge", Edge, 0, 0, false},
		{"unsupported position", Pos(12345), 0, 0, false},
		{"unsupported type", struct{}{}, 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			x, y, found := game.resolveTargetPosition(tt.target)
			if x != tt.x || y != tt.y || found != tt.found {
				t.Fatalf("position = (%v, %v, %v), want (%v, %v, %v)", x, y, found, tt.x, tt.y, tt.found)
			}
		})
	}
	original.transform().setPositionRaw(0, 0)
	if x, y, found := game.resolveTargetPosition("actor"); x != 0 || y != 0 || !found {
		t.Fatalf("target at origin = (%v, %v, %v), want (0, 0, true)", x, y, found)
	}
}

func TestResolveRandomTargetUsesWorldBoundsAndSeed(t *testing.T) {
	game := &Game{displayState: gameDisplayState{WorldWidth: 5, WorldHeight: 3, WindowWidth: 1, WindowHeight: 1}}
	t.Cleanup(ResetRandomSeed)
	sequence := func() [32][2]float64 {
		SetRandomSeed(42)
		var points [32][2]float64
		for i := range points {
			x, y, found := game.resolveTargetPosition(Random)
			if !found || x < -2 || x > 2 || y < -1 || y > 1 {
				t.Fatalf("random target = (%v, %v, %v), outside centered 5x3 world", x, y, found)
			}
			points[i] = [2]float64{x, y}
		}
		return points
	}
	first, second := sequence(), sequence()
	if first != second {
		t.Fatal("the same seed produced different random target sequences")
	}
	var outsideWindow bool
	for _, point := range first {
		outsideWindow = outsideWindow || point != [2]float64{}
	}
	if !outsideWindow {
		t.Fatal("random targets stayed inside the 1x1 window instead of using the world")
	}
}

func TestScheduling(t *testing.T) {
	for _, schedule := range []struct {
		name string
		call func() int
	}{
		{"SchedNow", SchedNow},
		{"Sched", Sched},
	} {
		for _, tt := range []struct {
			name    string
			offset  time.Duration
			demoted bool
		}{
			{"active", time.Minute, false},
			{"expired", -2 * mainExecTimeoutSec * time.Second, true},
		} {
			t.Run(schedule.name+"/Main/"+tt.name, func(t *testing.T) {
				currentScheduler := setupRuntimeScheduler(t)
				returned := false
				thread := currentScheduler.Create("main", func(thread coroutine.Thread) {
					started := time.Now().Add(tt.offset)
					end := thread.BeginMain(started)
					defer end()

					if got := schedule.call(); got != 0 {
						t.Errorf("%s() = %d, want 0", schedule.name, got)
					}
					want := started
					if tt.demoted {
						want = time.Time{}
					}
					if got := thread.MainStartedAt(); !got.Equal(want) {
						t.Errorf("MainStartedAt = %v, want %v", got, want)
					}
					returned = true
				})
				currentScheduler.Join(thread)
				if !returned {
					t.Fatal("Main scheduling stopped the coroutine before its assertions completed")
				}
			})
		}

		t.Run(schedule.name+"/ExternalCaller", func(t *testing.T) {
			currentScheduler := setupRuntimeScheduler(t)
			started := make(chan struct{})
			release := make(chan struct{})
			thread := currentScheduler.Create("main", func(thread coroutine.Thread) {
				end := thread.BeginMain(time.Now().Add(-2 * mainExecTimeoutSec * time.Second))
				defer end()
				close(started)
				<-release
			})
			defer func() {
				close(release)
				currentScheduler.Join(thread)
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("active coroutine did not start")
			}

			result := make(chan any, 1)
			go func() {
				defer func() { result <- recover() }()
				schedule.call()
			}()
			select {
			case recovered := <-result:
				if recovered != nil {
					t.Fatalf("external %s panicked: %v", schedule.name, recovered)
				}
			case <-time.After(time.Second):
				t.Fatalf("external %s did not return", schedule.name)
			}
			if thread.Stopped() || thread.MainStartedAt().IsZero() {
				t.Fatalf("external %s changed the active coroutine", schedule.name)
			}
		})

		t.Run(schedule.name+"/WithoutManager", func(t *testing.T) {
			original := scheduler
			scheduler = nil
			defer func() { scheduler = original }()
			if got := schedule.call(); got != 0 {
				t.Errorf("%s() = %d, want 0", schedule.name, got)
			}
		})
	}
}

func TestSetDebugFlagsForwardsPerfMode(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)
	var game Game
	for _, flags := range []dbgFlags{
		DbgFlagInstr | DbgFlagEvent | DbgFlagPerf,
		DbgFlagInstr,
		DbgFlagEvent,
		DbgFlagPerf,
		0,
	} {
		game.setDebugFlags(flags)
		currentScheduler.Update()
		if got, want := currentScheduler.UpdateStats().GCStatsEnabled, flags&DbgFlagPerf != 0; got != want {
			t.Errorf("flags %v: performance statistics enabled = %v, want %v", flags, got, want)
		}
		if got, want := game.debugState.DebugInstr, flags&DbgFlagInstr != 0; got != want {
			t.Errorf("flags %v: DebugInstr = %v, want %v", flags, got, want)
		}
		if got, want := game.debugState.DebugEvent, flags&DbgFlagEvent != 0; got != want {
			t.Errorf("flags %v: DebugEvent = %v, want %v", flags, got, want)
		}
	}
}

func TestSetDebugUpdatesDefaultAndRunningGame(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)
	originalGame, originalFlags := engine.GetGame(), defaultDebugFlags.Load()
	t.Cleanup(func() {
		engine.SetGame(originalGame)
		defaultDebugFlags.Store(originalFlags)
		spxlog.SetLevel(spxlog.LevelInfo)
	})
	engine.SetGame(nil)
	SetDebug(DbgFlagPerf | dbgFlags(1<<20))
	currentScheduler.Update()
	if defaultDebugFlags.Load() != uint32(DbgFlagPerf) || !currentScheduler.UpdateStats().GCStatsEnabled {
		t.Fatal("default debug flags did not filter unknown flags or enable profiling")
	}
	var game Game
	engine.SetGame(&game)
	SetDebug(DbgFlagInstr | DbgFlagEvent)
	if !game.debugState.DebugInstr || !game.debugState.DebugEvent {
		t.Fatalf("game debug=%+v", game.debugState)
	}
}

func (mainThreadTestPlatform) IsMainThread() bool {
	return true
}

func setupImmediateEngineCalls(t testing.TB) {
	t.Helper()
	wrap.Init(func(call func()) { call() })
	t.Cleanup(func() { wrap.Init(engine.WaitMainThread) })
}

func TestDisplaySizesDefaultToBackdropAndPreserveConfiguredValues(t *testing.T) {
	for _, tt := range []struct {
		name          string
		display       gameDisplayState
		window, world [2]int
	}{
		{
			name: "backdrop defaults", window: [2]int{320, 240}, world: [2]int{320, 240},
		},
		{
			name:    "configured window",
			display: gameDisplayState{WindowWidth: 384, WindowHeight: 216},
			window:  [2]int{384, 216}, world: [2]int{320, 240},
		},
		{
			name:    "configured world",
			display: gameDisplayState{WorldWidth: 1024, WorldHeight: 768},
			window:  [2]int{320, 240}, world: [2]int{1024, 768},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{
				baseObj: baseObj{costumes: []*costume{newCostume(&coreproject.CostumeConfig{
					ImageWidth:       641.5,
					ImageHeight:      481.25,
					BitmapResolution: 2,
				})}},
				displayState: tt.display,
			}

			windowW, windowH := game.windowSize()
			if got := [2]int{windowW, windowH}; got != tt.window {
				t.Fatalf("windowSize() = %v, want %v", got, tt.window)
			}
			worldW, worldH := game.worldSize()
			if got := [2]int{worldW, worldH}; got != tt.world {
				t.Fatalf("worldSize() = %v, want %v", got, tt.world)
			}
		})
	}
}

func TestGameGoSetBackdropRandom(t *testing.T) {
	for _, test := range []struct {
		name     string
		selector any
		costumes []*costume
		initial  int
	}{
		{
			name: "action", selector: Random, initial: 1,
			costumes: []*costume{{name: "backdrop1"}, {name: "backdrop2"}, {name: "backdrop3"}},
		},
		{
			name: "float64", selector: float64(Random),
			costumes: []*costume{{name: "backdrop1"}, {name: "backdrop2"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &Game{baseObj: baseObj{costumes: test.costumes, costumeIndex: test.initial}}
			SetRandomSeed(1)
			t.Cleanup(ResetRandomSeed)

			if !game.goSetBackdrop(test.selector) {
				t.Fatal("goSetBackdrop(Random) = false, want true")
			}
			if got := game.costumeIndex; got < 0 || got >= len(game.costumes) || got == test.initial {
				t.Fatalf("costumeIndex = %d, want in [0, %d) and different from %d", got, len(game.costumes), test.initial)
			}
		})
	}
}

func TestGameSetRandomBackdropWithoutChoice(t *testing.T) {
	for _, test := range []struct {
		name     string
		costumes []*costume
		wantOK   bool
	}{
		{name: "no backdrops"},
		{name: "single backdrop", costumes: []*costume{{name: "backdrop1"}}, wantOK: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &Game{baseObj: baseObj{costumes: test.costumes}}
			if got := game.setRandomBackdrop(); got != test.wantOK {
				t.Fatalf("setRandomBackdrop() = %v, want %v", got, test.wantOK)
			}
			if got := game.costumeIndex; got != 0 {
				t.Fatalf("costumeIndex = %d, want unchanged index 0", got)
			}
		})
	}
}

func TestSetupWorldAndWindowClampsBackdropWindowToWorld(t *testing.T) {
	game := &Game{}
	proj := &coreproject.ProjectConfig{
		Map: coreproject.MapConfig{
			Width:  320,
			Height: 180,
			Mode:   "repeat",
		},
		Backdrops: []*coreproject.BackdropConfig{
			{
				CostumeConfig: coreproject.CostumeConfig{
					Path:        "bg.png",
					ImageWidth:  640,
					ImageHeight: 480,
				},
			},
		},
	}

	game.setupWorldAndWindow(proj)

	if game.displayState.WorldWidth != 320 || game.displayState.WorldHeight != 180 {
		t.Fatalf("world size = %dx%d, want 320x180", game.displayState.WorldWidth, game.displayState.WorldHeight)
	}
	if game.displayState.WindowWidth != 320 || game.displayState.WindowHeight != 180 {
		t.Fatalf("window size = %dx%d, want 320x180", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}
	if game.displayState.MinWorldX != -160 || game.displayState.MinWorldY != -90 {
		t.Fatalf("world min = (%d, %d), want (-160, -90)", game.displayState.MinWorldX, game.displayState.MinWorldY)
	}
	if game.displayState.MapMode != coreproject.MapModeRepeat {
		t.Fatalf("map mode = %d, want %d", game.displayState.MapMode, coreproject.MapModeRepeat)
	}
}

type targetPropertyActor struct {
	Score int
	Power float64
}

type targetLookupSprite struct {
	SpriteImpl
}

type targetPropertyGame struct {
	Game
	StageScore int
	Actor      targetPropertyActor
	Wrapped    Value
}

type stagePlanGame struct {
	Game
	Value    collisionLayerOrderSprite
	Pointer  *collisionLayerOrderSprite
	Dynamic  Sprite
	values   []collisionLayerOrderSprite
	pointers []*collisionLayerOrderSprite
}

type gameNavigationMgr struct {
	gdx.INavigationMgr
	grid, cell, from, to mathf.Vec2
	jump, debug          bool
	setups               int
}

type gameBackdropSpriteMgr struct {
	gdx.ISpriteMgr
	repeat        mathf.Vec4
	position      mathf.Vec2
	positionCalls int
}

func TestGameUsernameReturnsConfiguredUsername(t *testing.T) {
	previous := spxapi.Username()
	t.Cleanup(func() { spxapi.SetUsername(previous) })

	spxapi.SetUsername("scratch-user")

	var g Game
	if got := g.Username(); got != "scratch-user" {
		t.Fatalf("Game.Username() = %q, want %q", got, "scratch-user")
	}
}

func TestGameDayssince2000(t *testing.T) {
	var g Game
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

	lower := time.Since(base).Hours() / 24
	got := g.Dayssince2000()
	upper := time.Since(base).Hours() / 24

	if got < lower || got > upper {
		t.Fatalf("Dayssince2000 = %v, want in [%v, %v]", got, lower, upper)
	}
}

func TestGameWidgetLookup(t *testing.T) {
	var game Game
	first := &Monitor{name: "score"}
	game.shapeMgr.add(42)
	game.shapeMgr.add(&Monitor{name: "other"})
	game.shapeMgr.add(first)
	game.shapeMgr.add(&Monitor{name: "score"})
	sprite := &SpriteImpl{g: &game}
	for _, test := range []struct {
		name string
		root ShapeGetter
	}{
		{"game", &game},
		{"sprite", sprite},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := GetWidget(test.root, "score"); got != first {
				t.Fatalf("GetWidget = %p, want first matching widget %p", got, first)
			}
			if got := XGot_Game_XGox_GetWidget[Monitor](test.root, "score"); got != first {
				t.Fatalf("typed GetWidget = %p, want %p", got, first)
			}
		})
	}
	for _, test := range []struct {
		name string
		call func()
		want string
	}{
		{"missing", func() { GetWidget(&game, "missing") }, "GetWidget: widget not found - missing"},
		{"case sensitive", func() { GetWidget(&game, "Score") }, "GetWidget: widget not found - Score"},
		{"wrong type", func() { XGot_Game_XGox_GetWidget[struct{}](&game, "score") }, "GetWidget: type mismatch - score"},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != test.want {
					t.Fatalf("panic = %v, want %q", got, test.want)
				}
			}()
			test.call()
		})
	}
}

func TestGameGetTargetPropertyMissing(t *testing.T) {
	game := &targetPropertyGame{}
	game.gamer = game
	for _, target := range []string{"", "Actor", "Missing"} {
		if got := game.GetTargetProperty(target, "Missing"); got.Raw() != nil {
			t.Errorf("GetTargetProperty(%q, Missing) = %v, want empty value", target, got)
		}
	}
}

func TestGameGetTargetReturnsNamedSprite(t *testing.T) {
	var game Game
	clone := newTargetLookupSprite(&game, "Hero", true)
	target := newTargetLookupSprite(&game, "Hero", false)
	game.shapeMgr.add(&clone.SpriteImpl)
	game.shapeMgr.add(&target.SpriteImpl)

	if got := game.GetTarget("Hero"); got != &target.SpriteImpl {
		t.Fatalf("GetTarget(%q) = %v, want %v", "Hero", got, &target.SpriteImpl)
	}
}

func TestGameGetTargetReturnsNilWithoutMatch(t *testing.T) {
	var game Game
	game.shapeMgr.add(&newTargetLookupSprite(&game, "Hero", true).SpriteImpl)

	for _, target := range []string{"", "Missing", "hero"} {
		if got := game.GetTarget(target); got != nil {
			t.Errorf("GetTarget(%q) = %v, want nil", target, got)
		}
	}
}

func TestGameGetTargetReturnsNilAfterTargetIsRemoved(t *testing.T) {
	var game Game
	target := newTargetLookupSprite(&game, "Hero", false)
	clone := newTargetLookupSprite(&game, "Hero", true)
	game.shapeMgr.add(&target.SpriteImpl)
	game.shapeMgr.add(&clone.SpriteImpl)

	game.shapeMgr.removeShape(&target.SpriteImpl)

	if got := game.GetTarget("Hero"); got != nil {
		t.Fatalf("GetTarget(%q) after target removal = %v, want nil", "Hero", got)
	}
}

func TestGameGetTargetPropertyStageField(t *testing.T) {
	g := &targetPropertyGame{StageScore: 23}
	g.gamer = g

	got := g.GetTargetProperty("", "StageScore")
	if got.Int() != 23 {
		t.Fatalf("StageScore = %d, want 23", got.Int())
	}
}

func TestGameGetTargetPropertyPreservesValueWrapper(t *testing.T) {
	g := &targetPropertyGame{Wrapped: NewValue("7")}
	g.gamer = g
	got := g.GetTargetProperty("", "Wrapped")
	if wrapped, ok := got.Raw().(Value); !ok || wrapped.Raw() != "7" {
		t.Fatalf("Wrapped = %T(%v), want the original Value", got.Raw(), got.Raw())
	}
	if got.Int() != 0 || Compare(got, 10) != 1 {
		t.Fatal("property evaluation must retain the extra Value wrapper")
	}
}

func TestGameGetTargetPropertyTargetField(t *testing.T) {
	g := &targetPropertyGame{Actor: targetPropertyActor{Score: 9}}
	g.gamer = g

	got := g.GetTargetProperty("Actor", "Score")
	if got.Int() != 9 {
		t.Fatalf("Actor.Score = %d, want 9", got.Int())
	}
}

func TestGameGetTargetPropertyTargetMethod(t *testing.T) {
	g := &targetPropertyGame{Actor: targetPropertyActor{Power: 3.5}}
	g.gamer = g

	got := g.GetTargetProperty("Actor", "Health")
	if got.Float() != 3.5 {
		t.Fatalf("Actor.Health = %v, want 3.5", got.Float())
	}
}

func TestStageUsesDirectFieldsAndPreservesSpriteOrder(t *testing.T) {
	setupCloneSpriteMgr(t)
	game := &stagePlanGame{}
	game.initShapeMgr()
	game.Value = *newCollisionLayerOrderSprite(&game.Game, "Value", nil)
	game.Pointer = newCollisionLayerOrderSprite(&game.Game, "Pointer", nil)
	game.Dynamic = newCollisionLayerOrderSprite(&game.Game, "Dynamic", nil)
	back := newCollisionLayerOrderSprite(&game.Game, "Back", nil)
	front := newCollisionLayerOrderSprite(&game.Game, "Front", nil)
	prototype := newCollisionLayerOrderSprite(&game.Game, "collisionLayerOrderSprite", nil)
	prototype.baseObj.initWithSize(1, 1)
	prototype.physics().collisionInfo.Type = physicsColliderNone
	prototype.physics().triggerInfo.Type = physicsColliderNone
	game.sprs = map[string]Sprite{"Back": back, "Front": front, "collisionLayerOrderSprite": prototype}
	raw := []any{
		"Back",
		coreproject.StageShape{"type": "sprite", "target": "Value", "x": 1.0},
		coreproject.StageShape{"type": "sprite", "target": "Pointer", "x": 2.0},
		coreproject.StageShape{"type": "sprite", "target": "Dynamic", "x": 3.0},
		coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{coreproject.StageShape{"x": 4.0}, coreproject.StageShape{"x": 5.0}}},
		coreproject.StageShape{"type": "sprites", "target": "pointers", "items": []any{coreproject.StageShape{"x": 6.0}}},
		coreproject.StageShape{"type": "monitor", "target": "", "val": "missing", "name": "missing", "label": "Missing", "mode": 1.0, "x": 0.0, "y": 0.0, "visible": true},
		"Front",
	}
	sprites, err := game.loadStageEntries(reflect.ValueOf(game).Elem(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(sprites) != 8 {
		t.Fatalf("loaded %d sprites, want 8", len(sprites))
	}
	if len(game.values) != 2 || len(game.pointers) != 1 {
		t.Fatalf("group sizes = %d, %d", len(game.values), len(game.pointers))
	}
	want := []Sprite{back, &game.Value, game.Pointer, game.Dynamic, &game.values[0], &game.values[1], game.pointers[0], front}
	shapes := game.getAllShapes()
	for i, sprite := range want {
		if sprites[i] != sprite || shapes[i] != spriteOf(sprite) {
			t.Fatalf("sprite %d bound to wrong live object", i)
		}
		if spriteOf(sprite).runtimeState.layer != firstSpriteLayer+i {
			t.Fatalf("sprite %d layer = %d", i, spriteOf(sprite).runtimeState.layer)
		}
		if i > 0 && i < 7 && spriteOf(sprite).Xpos() != float64(i) {
			t.Fatalf("sprite %d x = %v", i, spriteOf(sprite).Xpos())
		}
	}
}

func TestPreparedDisplayShapesOwnAllConfiguration(t *testing.T) {
	for _, kind := range []string{"monitor", "stageMonitor"} {
		t.Run(kind, func(t *testing.T) {
			shape := coreproject.StageShape{
				"type": kind, "target": "", "val": "score", "name": "score", "label": "Score", "mode": "slider",
				"x": 2.0, "y": 3.0, "visible": true, "color": "#123456", "width": 120.0, "height": 90.0,
				"sliderMin": 4.0, "sliderMax": 8.0, "isDiscrete": false,
			}
			got, err := prepareMonitor(shape)
			if err != nil {
				t.Fatal(err)
			}
			clear(shape)
			if got.config.Name != "score" || got.config.X != 2 || got.config.Y != 3 || !got.config.Visible {
				t.Fatalf("prepared monitor config = %+v", got.config)
			}
			color, _ := mathf.NewColorAny("#123456")
			if got.style.Appearance != ui.MonitorAppearanceSlider || got.style.Color != color || got.style.Dimensions != mathf.NewVec2(120, 90) || got.style.Slider != (ui.MonitorSlider{Min: 4, Max: 8, Step: 0.01}) {
				t.Fatalf("prepared monitor style = %+v", got.style)
			}
		})
	}
	t.Run("measure", func(t *testing.T) {
		shape := coreproject.StageShape{"type": "measure", "size": 10.0, "x": 20.0, "y": 30.0, "scale": 2.0, "heading": 45.0}
		got, err := coreproject.ParseMeasureShape(shape)
		if err != nil {
			t.Fatal(err)
		}
		clear(shape)
		if got.Size != 10 || got.X != 20 || got.Y != 30 || got.Scale != 2 || got.Heading != 45 {
			t.Fatalf("prepared measure = %+v", got)
		}
	})
}

func TestStageRejectsPromotedFieldTargets(t *testing.T) {
	type embedded struct{ Target *collisionLayerOrderSprite }
	type holder struct{ embedded }
	game := holder{embedded{Target: &collisionLayerOrderSprite{}}}
	var stage Game
	_, err := stage.loadStageEntries(reflect.ValueOf(&game).Elem(), []any{
		coreproject.StageShape{"type": "sprite", "target": "Target"},
	})
	if err == nil {
		t.Fatal("promoted field unexpectedly accepted as direct stage target")
	}
}

func TestColdLoadKeepsUnregisteredSpriteFieldsUnloaded(t *testing.T) {
	game := &struct {
		Game
		DirectLoadSprite *DirectLoadSprite
		Data             any
	}{}
	files := gameTestFS{}
	setupGameTestRuntime(t, files, game, nil, false)
	game.typs["Data"] = reflect.TypeFor[int]()
	gamer := reflect.ValueOf(game).Elem()
	plan, err := game.prepareProject(files, &coreproject.ProjectConfig{}, gamer)
	if err != nil {
		t.Fatal(err)
	}
	if game.DirectLoadSprite != nil || game.Data != nil {
		t.Fatal("preparation allocated live fields")
	}
	game.applySpriteFields(gamer, &plan)
	if game.DirectLoadSprite == nil {
		t.Fatal("cold load did not allocate the sprite field")
	}
	if data, ok := game.Data.(*int); !ok || data == nil {
		t.Fatalf("registered non-sprite field = %v, want allocated *int", game.Data)
	}
	if len(game.sprs) != 0 {
		t.Fatalf("cold load initialized unregistered sprites: %v", game.sprs)
	}
}

func TestColdStageKeepsLazyLoadFailureBeforeLaterParseFailure(t *testing.T) {
	var game Game
	game.initShapeMgr()
	game.sprs = make(map[string]Sprite)
	game.typs = map[string]reflect.Type{"Sprite": reflect.TypeFor[collisionLayerOrderSprite]()}
	game.fs = gameTestFS{}
	_, err := game.loadStageEntries(reflect.Value{}, []any{
		"Sprite", coreproject.StageShape{"type": "measure", "size": "invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), `zorder[0]: load sprite "Sprite": file not found:`) {
		t.Fatalf("cold load failure = %v, want indexed first sprite config error", err)
	}
}

func TestColdStageWrapsEntryFailureWithLayer(t *testing.T) {
	game := &stagePlanGame{}
	game.initShapeMgr()
	_, err := game.loadStageEntries(reflect.ValueOf(game).Elem(), []any{
		coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{}}, coreproject.StageShape{},
	})
	if err == nil || err.Error() != "zorder[1]: invalid stage shape type" {
		t.Fatalf("cold load failure = %v, want indexed stage shape error", err)
	}
}

func TestStageLoadsEmptyValues(t *testing.T) {
	for _, target := range []string{"values", "pointers"} {
		t.Run(target, func(t *testing.T) {
			game := &stagePlanGame{}
			game.initShapeMgr()
			gamer := reflect.ValueOf(game).Elem()
			sprites, err := game.loadStageEntries(gamer, []any{
				coreproject.StageShape{"type": "sprites", "target": target, "items": []any{}},
			})
			if err != nil {
				t.Fatal(err)
			}
			field := gamer.FieldByName(target)
			if len(sprites) != 0 || len(game.getAllShapes()) != 0 || field.IsNil() || field.Len() != 0 {
				t.Fatal("empty group did not initialize an empty slice without loading a prototype")
			}
		})
	}
	t.Run("empty sprite name", func(t *testing.T) {
		var game Game
		game.initShapeMgr()
		sprite := newCollisionLayerOrderSprite(&game, "", nil)
		game.sprs = map[string]Sprite{"": sprite}
		sprites, err := game.loadStageEntries(reflect.Value{}, []any{""})
		if err != nil || len(sprites) != 1 || sprites[0] != sprite {
			t.Fatalf("empty named sprite: loaded=%v, error=%v", sprites, err)
		}
	})
}

func TestStageGroupValidatesAllItemsBeforeLoading(t *testing.T) {
	original := &collisionLayerOrderSprite{}
	game := &stagePlanGame{pointers: []*collisionLayerOrderSprite{original}}
	game.initShapeMgr()
	game.sprs = make(map[string]Sprite)
	game.fs = gameTestFS{}
	_, err := game.loadStageEntries(reflect.ValueOf(game).Elem(), []any{
		coreproject.StageShape{"type": "sprites", "target": "pointers", "items": []any{
			coreproject.StageShape{"x": 1.0}, coreproject.StageShape{"x": "invalid"},
		}},
	})
	want := `zorder[0]: stage sprites target "pointers" item[1]: stage shape field "x" has type string, want float64`
	if err == nil || err.Error() != want {
		t.Fatalf("group error = %v, want item validation before prototype loading: %s", err, want)
	}
	if len(game.pointers) != 1 || game.pointers[0] != original || len(game.getAllShapes()) != 0 || len(game.sprs) != 0 {
		t.Fatal("invalid group changed the target slice, shapes, or loaded prototypes")
	}
}

func TestStageEntryValidationErrors(t *testing.T) {
	game := &struct {
		Sprite   *collisionLayerOrderSprite
		Number   int
		BadItems []int
		Sprites  []*collisionLayerOrderSprite
	}{}
	tests := []struct {
		name string
		raw  any
		want string
	}{
		{"nil", nil, "invalid zorder entry type <nil>"},
		{"number", 42, "invalid zorder entry type int"},
		{"boolean", false, "invalid zorder entry type bool"},
		{"array", []any{}, "invalid zorder entry type []interface {}"},
		{"nil shape", coreproject.StageShape(nil), "invalid stage shape type"},
		{"missing type", coreproject.StageShape{}, "invalid stage shape type"},
		{"null type", coreproject.StageShape{"type": nil}, "invalid stage shape type"},
		{"non-string type", coreproject.StageShape{"type": 1.0}, "invalid stage shape type"},
		{"unknown shape", coreproject.StageShape{"type": "unknown"}, "unknown shape - unknown"},
		{"case-sensitive alias", coreproject.StageShape{"type": "StageMonitor"}, "unknown shape - StageMonitor"},
		{"empty type", coreproject.StageShape{"type": ""}, "unknown shape - "},
		{"monitor validation", coreproject.StageShape{"type": "monitor"}, `stage shape field "target" is required`},
		{"alias validation", coreproject.StageShape{"type": "stageMonitor"}, `stage shape field "target" is required`},
		{"measure validation", coreproject.StageShape{"type": "measure", "size": "large"}, `stage shape field "size" has type string, want float64`},
		{"measure color", coreproject.StageShape{"type": "measure", "size": 10.0, "x": 0.0, "y": 0.0, "color": []any{1.0}}, `stage shape field "color": unsupported color format`},
		{"monitor mode", coreproject.StageShape{"type": "monitor", "target": "", "val": "score", "name": "score", "label": "Score", "mode": nil, "x": 0.0, "y": 0.0, "visible": true}, `stage shape field "mode" has type <nil>, want float64`},
		{"missing target", coreproject.StageShape{"type": "sprite"}, "stage shape target must be a non-empty string"},
		{"empty target", coreproject.StageShape{"type": "sprite", "target": ""}, "stage shape target must be a non-empty string"},
		{"non-string target", coreproject.StageShape{"type": "sprites", "target": 1.0}, "stage shape target must be a non-empty string"},
		{"unknown sprite", coreproject.StageShape{"type": "sprite", "target": "Missing"}, `stage sprite target "Missing" is not defined`},
		{"non-sprite field", coreproject.StageShape{"type": "sprite", "target": "Number"}, `stage sprite target "Number" is not a sprite field`},
		{"nil sprite", coreproject.StageShape{"type": "sprite", "target": "Sprite"}, `stage sprite target "Sprite" has no sprite value`},
		{"sprite property", coreproject.StageShape{"type": "sprite", "target": "Sprite", "x": "left"}, `stage shape field "x" has type string, want float64`},
		{"missing items", coreproject.StageShape{"type": "sprites", "target": "Sprites"}, "stage shape items must be an array"},
		{"null items", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": nil}, "stage shape items must be an array"},
		{"non-array items", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": false}, "stage shape items must be an array"},
		{"unknown sprites", coreproject.StageShape{"type": "sprites", "target": "Missing", "items": []any{}}, `stage sprites target "Missing" is not defined`},
		{"non-slice field", coreproject.StageShape{"type": "sprites", "target": "Sprite", "items": []any{}}, `stage sprites target "Sprite" is not a slice`},
		{"non-sprite items", coreproject.StageShape{"type": "sprites", "target": "BadItems", "items": []any{}}, `stage sprites target "BadItems" has invalid item type int`},
		{"invalid item", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": []any{coreproject.StageShape{}, false}}, `stage sprites target "Sprites" item[1] has invalid type bool`},
		{"item property", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": []any{coreproject.StageShape{}, coreproject.StageShape{"x": "left"}}}, `stage sprites target "Sprites" item[1]: stage shape field "x" has type string, want float64`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stage Game
			_, err := stage.loadStageEntries(reflect.ValueOf(game).Elem(), []any{test.raw})
			want := "zorder[0]: " + test.want
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})
	}
}

func TestStageInstantiatesDisplayWidgets(t *testing.T) {
	setupGameWidgetUi(t)
	game := &monitorEvalFixture{score: 3}
	root := reflect.ValueOf(game).Elem()
	shapes := []any{
		coreproject.StageShape{"type": "monitor", "name": "score", "val": "score", "target": "", "label": "Score", "mode": 1.0, "x": 0.0, "y": 0.0, "visible": true},
		coreproject.StageShape{"type": "measure", "size": 12.0, "x": 0.0, "y": 0.0},
	}
	sprites, err := game.loadStageEntries(root, shapes)
	if err != nil || len(sprites) != 0 || len(game.getAllShapes()) != 2 {
		t.Fatalf("display widgets=%v, sprites=%v, error=%v", game.getAllShapes(), sprites, err)
	}
	if got := GetWidget(&game.Game, "score"); got.GetName() != "score" {
		t.Fatalf("widget=%v", got)
	}
}

func TestGamePropertyResolutionScopes(t *testing.T) {
	var plain Game
	if got := plain.propertyRootValue(); got.Addr().Interface() != &plain {
		t.Fatal("unbound game root changed")
	}
	if got := plain.GetTargetProperty("", ""); got.Raw() != nil {
		t.Fatalf("empty property = %v, want zero Value", got)
	}
	if plain.setStageMonitor("", "missing", true) {
		t.Fatal("missing monitor resolved")
	}
	game := &struct {
		Game
		Sprite spriteLoadTestSprite
	}{}
	game.gamer = game
	if got, from := game.resolvePropertyTarget("Sprite"); from != 2 || got.Addr().Interface() != &game.Sprite {
		t.Fatalf("sprite scope=%v, %d", got, from)
	}
}

func TestGamePathFinderConfigurationAndQueries(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := gdx.NavigationMgr
	manager := &gameNavigationMgr{}
	gdx.NavigationMgr = manager
	t.Cleanup(func() { gdx.NavigationMgr = previous })
	game := &Game{displayState: gameDisplayState{WorldWidth: 480, WorldHeight: 360}, pathfindingState: gamePathfindingState{PathCellSizeX: 24, PathCellSizeY: 12}}
	game.SetupPathFinder__0()
	if manager.grid != mathf.NewVec2(20, 30) || manager.cell != mathf.NewVec2(24, 12) || !manager.jump || manager.debug {
		t.Fatalf("default navigation=%+v", manager)
	}
	game.SetupPathFinder__1(10, 12, 3, 4, false, true)
	if manager.grid != mathf.NewVec2(10, 12) || manager.cell != mathf.NewVec2(3, 4) || manager.jump || !manager.debug {
		t.Fatalf("explicit navigation=%+v", manager)
	}
	for _, call := range []func() []float64{
		func() []float64 { return game.FindPath__0(1, 2, 3, 4) },
		func() []float64 { return game.FindPath__1(1, 2, 3, 4, true) },
		func() []float64 { return game.FindPath__2(1, 2, 3, 4, false, false) },
	} {
		if got := call(); !slices.Equal(got, []float64{1, 2, 3, 4}) {
			t.Fatalf("path=%v", got)
		}
	}
	if manager.setups != 3 || manager.from != mathf.NewVec2(1, 2) || manager.to != mathf.NewVec2(3, 4) || manager.jump {
		t.Fatalf("navigation query=%+v", manager)
	}
}

func TestGameBackdropOverloads(t *testing.T) {
	t.Cleanup(func() { wrap.Init(engine.WaitMainThread) })
	game := setupStageLoadTestGame(t, gameTestFS{})
	game.baseObj.initWithSize(480, 360)
	manager := &gameBackdropSpriteMgr{ISpriteMgr: gdx.SpriteMgr}
	gdx.SpriteMgr = manager
	game.displayState.WorldWidth, game.displayState.WorldHeight = 960, 720
	game.costumes = []*costume{newCostumeWithSize(480, 360), newCostumeWithSize(480, 360)}
	game.costumes[0].name = "first"
	game.costumes[1].name = "second"
	game.runtimeState.syncSprite = &engine.Sprite{Sprite: gdx.Sprite{Id: 89801}}
	for _, test := range []struct {
		name string
		call func()
		want int
	}{
		{"name", func() { game.SetBackdrop__0("second") }, 1},
		{"float", func() { game.SetBackdrop__1(0) }, 0},
		{"int", func() { game.SetBackdrop__2(1) }, 1},
		{"action", func() { game.SetBackdrop__3(Next) }, 0},
		{"wait name", func() { game.SetBackdropAndWait__0("second") }, 1},
		{"wait float", func() { game.SetBackdropAndWait__1(0) }, 0},
		{"wait int", func() { game.SetBackdropAndWait__2(1) }, 1},
		{"wait action", func() { game.SetBackdropAndWait__3(Next) }, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.want == 1 {
				game.displayState.MapMode = coreproject.MapModeRepeat
			} else {
				game.displayState.MapMode = coreproject.MapModeActualSize
			}
			test.call()
			if test.want == 1 && manager.repeat != (mathf.Vec4{X: 2, Y: 2}) {
				t.Fatalf("repeat scale=%v", manager.repeat)
			}
			if test.want == 0 && (manager.positionCalls == 0 || manager.position != (mathf.Vec2{})) {
				t.Fatalf("actual size position=%v (%d calls)", manager.position, manager.positionCalls)
			}
			if game.BackdropIndex() != test.want || game.BackdropName() != game.costumes[test.want].name {
				t.Fatalf("backdrop index/name=%d/%s", game.BackdropIndex(), game.BackdropName())
			}
		})
	}
}

func TestGameGraphicEffectsAndClock(t *testing.T) {
	setupRuntimeScheduler(t)
	clock.Start()
	t.Cleanup(func() { clock.Start() })
	var game Game
	game.SetGraphicEffect(ColorEffect, 10)
	game.ChangeGraphicEffect(ColorEffect, 5)
	if game.greffUniforms[ColorEffect] != 15 {
		t.Fatalf("effect=%v", game.greffUniforms)
	}
	game.ClearGraphicEffects()
	for kind, value := range game.greffUniforms {
		if value != 0 {
			t.Fatalf("cleared %v=%v", kind, value)
		}
	}
	game.ResetTimer()
	if game.Timer() != 0 {
		t.Fatalf("reset timer=%v", game.Timer())
	}
	before := time.Now()
	got := game.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("Now=%v outside %v..%v", got, before, after)
	}
}

func TestGameFrameWaitAndRepeatUntil(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	t.Cleanup(func() { clock.Start() })
	var game Game
	count := 0
	finished := false
	thread := currentScheduler.Create(nil, func(coroutine.Thread) {
		game.WaitNextFrame()
		RepeatUntil(func() bool { return count == 3 }, func() { count++ })
		finished = true
	})
	currentScheduler.JoinFirstSlice(thread)
	if finished {
		t.Fatal("WaitNextFrame completed before a frame update")
	}
	for range 5 {
		clock.Update(1.0/30, 30)
		currentScheduler.Update()
	}
	if !finished || count != 3 {
		t.Fatalf("loop completed=%v, count=%d", finished, count)
	}
}

func TestStagePreparationChecksInitFromOnlyForNonemptyGroups(t *testing.T) {
	for _, test := range []struct {
		name  string
		entry any
		clone bool
	}{
		{"named", "layoutInvalidInitSprite", false},
		{"group", coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{coreproject.StageShape{}}}, true},
		{"empty group", coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &struct {
				Game
				Items []*layoutInvalidInitSprite
			}{}
			game.sprs = map[string]Sprite{"layoutInvalidInitSprite": &layoutInvalidInitSprite{}}
			_, err := game.prepareProject(gameTestFS{}, &coreproject.ProjectConfig{Zorder: []any{test.entry}}, reflect.ValueOf(game).Elem())
			if test.clone {
				if err == nil || !strings.Contains(err.Error(), "field value InitFrom") {
					t.Fatalf("clone preflight error = %v, want InitFrom signature error", err)
				}
			} else if err != nil {
				t.Fatalf("non-cloning stage entry rejected an unused InitFrom: %v", err)
			}
		})
	}
}

func TestSpritePropertiesDistinguishMissingAndZeroValues(t *testing.T) {
	newSprite := func() *SpriteImpl {
		sprite := &SpriteImpl{}
		sprite.components.transform = &transformComponent{
			x:             10,
			y:             20,
			direction:     30,
			rotationStyle: LeftRight,
		}
		sprite.spriteState.isVisible = true
		sprite.spriteState.cloned = true
		sprite.runtimeState.scale = 2
		sprite.costumeIndex = 3
		return sprite
	}

	missing, err := parseSpriteProperties(coreproject.StageShape{})
	if err != nil {
		t.Fatal(err)
	}
	unchanged := newSprite()
	applySpriteProperties(unchanged, missing)
	if transform := unchanged.transform(); transform.x != 10 || transform.y != 20 ||
		transform.direction != 30 || transform.rotationStyle != LeftRight {
		t.Fatalf("missing properties changed transform: %+v", transform)
	}
	if !unchanged.spriteState.isVisible || unchanged.runtimeState.scale != 2 || unchanged.costumeIndex != 3 {
		t.Fatalf("missing properties changed sprite: %+v", unchanged)
	}

	zero, err := parseSpriteProperties(coreproject.StageShape{
		"x": 0.0, "y": 0.0, "heading": 0.0,
		"rotationStyle": "none", "visible": false,
		"size": 0.0, "costumeIndex": 0.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	reset := newSprite()
	applySpriteProperties(reset, zero)
	if transform := reset.transform(); transform.x != 0 || transform.y != 0 ||
		transform.direction != 0 || transform.rotationStyle != None {
		t.Fatalf("zero properties were not applied to transform: %+v", transform)
	}
	if reset.spriteState.isVisible || reset.runtimeState.scale != 0 || reset.costumeIndex != 0 {
		t.Fatalf("zero properties were not applied to sprite: %+v", reset)
	}
}

func TestParseSpritePropertiesRejectsWrongTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"x", "0", "float64"},
		{"y", true, "float64"},
		{"heading", nil, "float64"},
		{"rotationStyle", 0.0, "string"},
		{"visible", 0.0, "bool"},
		{"size", "small", "float64"},
		{"costumeIndex", 0, "float64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseSpriteProperties(coreproject.StageShape{test.name: test.value})
			if err == nil || !strings.Contains(err.Error(), "want "+test.want) {
				t.Fatalf("parseSpriteProperties error = %v, want type %s", err, test.want)
			}
		})
	}
}

func (*targetLookupSprite) Main() {}

func (a *targetPropertyActor) Health() float64 {
	return a.Power
}

func (m *gameNavigationMgr) SetupPathFinderWithSize(grid, cell mathf.Vec2, jump, debug bool) {
	m.grid, m.cell, m.jump, m.debug = grid, cell, jump, debug
	m.setups++
}

func (m *gameNavigationMgr) FindPath(from, to mathf.Vec2, jump bool) gdx.Array {
	m.from, m.to, m.jump = from, to, jump
	return []float32{1, 2, 3, 4}
}

func (m *gameBackdropSpriteMgr) SetMaterialParamsVec(_ gdx.Object, effect string, x, y, z, w float64) {
	if effect == "repeat_scale" {
		m.repeat = mathf.Vec4{X: x, Y: y, Z: z, W: w}
	}
}

func (m *gameBackdropSpriteMgr) SetPosition(_ gdx.Object, pos mathf.Vec2) {
	m.position = pos
	m.positionCalls++
}

// Stage-focused tests execute the same prepare/apply phases as project loading,
// while retaining the already-live sprite fixtures they supply.
func (p *Game) loadStageEntries(gamer reflect.Value, entries []any) ([]Sprite, error) {
	plan := projectPlan{sprites: make(map[string]spritePlan)}
	var err error
	plan.stage, err = p.prepareStageEntries(p.fs, gamer, entries, &plan)
	if err != nil {
		return nil, err
	}
	return p.applyStageEntries(gamer, &plan), nil
}

func newTargetLookupSprite(g *Game, name string, cloned bool) *targetLookupSprite {
	sprite := &targetLookupSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.spriteState.cloned = cloned
	return sprite
}

type gameQueryPhysicsMgr struct {
	gdx.IPhysicsMgr
	result        engine.Array
	from, to      mathf.Vec2
	radius        float64
	mask          int64
	ignore        []int64
	areas, bodies bool
}

type gameDebugDrawMgr struct {
	gdx.IDebugMgr
	lines  [][2]mathf.Vec2
	colors []mathf.Color
}

func TestGameIntersectionQueriesResolveLiveSprites(t *testing.T) {
	game, manager, sprite := setupGameQueryPhysics(t)
	// A missing engine object is ignored while the registered sprite is retained.
	manager.result = []engine.Object{-1, sprite.getSpriteId()}
	if got := game.IntersectRect(10, 20, 30, 40); !slices.Equal(got, []Sprite{sprite}) {
		t.Fatalf("IntersectRect = %v, want target sprite", got)
	}
	if manager.from != mathf.NewVec2(10, 20) || manager.to != mathf.NewVec2(30, 40) || manager.mask != -1 {
		t.Fatalf("rectangle query = %+v", manager)
	}
	if got := game.IntersectCircle(-10, -20, 15); !slices.Equal(got, []Sprite{sprite}) {
		t.Fatalf("IntersectCircle = %v, want target sprite", got)
	}
	if manager.from != mathf.NewVec2(-10, -20) || manager.radius != 15 || manager.mask != -1 {
		t.Fatalf("circle query = %+v", manager)
	}
	manager.result = []engine.Object{}
	if got := game.IntersectCircle(0, 0, 1); len(got) != 0 {
		t.Fatalf("empty query = %v", got)
	}
}

func TestGameRaycastResolvesTargetsAndIgnores(t *testing.T) {
	game, manager, sprite := setupGameQueryPhysics(t)
	manager.result = []int64{1, sprite.getSpriteId(), engine.ConvertToInt64(12.5), engine.ConvertToInt64(-7.25), 0, 0}
	for _, test := range []struct {
		name   string
		call   func() (bool, Sprite, float64, float64)
		ignore []int64
	}{
		{"default", func() (bool, Sprite, float64, float64) { return game.Raycast__0(1, 2, 3, 4) }, nil},
		{"single", func() (bool, Sprite, float64, float64) { return game.Raycast__1(1, 2, 3, 4, sprite) }, []int64{sprite.getSpriteId()}},
		{"multiple with nil", func() (bool, Sprite, float64, float64) { return game.Raycast__2(1, 2, 3, 4, []Sprite{nil, sprite}) }, []int64{sprite.getSpriteId()}},
	} {
		t.Run(test.name, func(t *testing.T) {
			hit, target, x, y := test.call()
			if !hit || target != sprite || x != 12.5 || y != -7.25 {
				t.Fatalf("raycast = %v, %v, %v, %v", hit, target, x, y)
			}
			if manager.from != mathf.NewVec2(1, 2) || manager.to != mathf.NewVec2(3, 4) || manager.mask != -1 || !manager.areas || !manager.bodies || !slices.Equal(manager.ignore, test.ignore) {
				t.Fatalf("raycast arguments = %+v", manager)
			}
		})
	}
	for _, test := range []struct {
		name   string
		result engine.Array
		hit    bool
	}{
		{"miss", make([]int64, 6), false},
		{"missing sprite", []int64{1, -1, 0, 0, 0, 0}, true},
		{"malformed result", []int64{1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager.result = test.result
			hit, target, x, y := game.Raycast__0(1, 2, 3, 4)
			if hit != test.hit || target != nil || x != 0 || y != 0 {
				t.Fatalf("raycast = %v, %v, %v, %v", hit, target, x, y)
			}
		})
	}
}

func TestGameDebugDrawLinesValidatesCompleteSegments(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := gdx.DebugMgr
	manager := &gameDebugDrawMgr{}
	gdx.DebugMgr = manager
	t.Cleanup(func() { gdx.DebugMgr = previous })
	var game Game
	color := NewColor("#12345678")
	for _, points := range [][]float64{nil, {1, 2}, {1, 2, 3}, {1, 2, 3, 4, 5}} {
		game.DebugDrawLines(points, color)
	}
	if len(manager.lines) != 0 {
		t.Fatal("incomplete point pairs drew segments")
	}
	game.DebugDrawLines([]float64{1, 2, 3, 4, 5, 6}, color)
	want := [][2]mathf.Vec2{{mathf.NewVec2(1, 2), mathf.NewVec2(3, 4)}, {mathf.NewVec2(3, 4), mathf.NewVec2(5, 6)}}
	if !slices.Equal(manager.lines, want) {
		t.Fatalf("segments = %v, want %v", manager.lines, want)
	}
	for _, got := range manager.colors {
		if got != toMathfColor(color) {
			t.Fatalf("segment color = %v, want %v", got, toMathfColor(color))
		}
	}
}

func TestGameResetCollisionLayerStateClearsCollisionLayerState(t *testing.T) {
	var g Game
	g.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {layerIndex: 0, Layer: 1 << 0, Mask: 1 << 1},
	}
	g.sprCollisionData = []spriteCollisionData{{
		info: g.sprCollisionInfos["SpriteA"],
	}}

	g.resetCollisionLayerState()

	if g.sprCollisionInfos != nil {
		t.Fatalf("sprCollisionInfos = %#v, want nil", g.sprCollisionInfos)
	}
	if g.sprCollisionData != nil {
		t.Fatalf("sprCollisionData = %#v, want nil", g.sprCollisionData)
	}
}

func TestTryRaycastResult(t *testing.T) {
	for _, test := range []struct {
		name    string
		data    engine.Array
		want    rayCastResult
		wantErr string
	}{
		{
			name: "hit",
			data: []int64{
				1, 42,
				engine.ConvertToInt64(12.5), engine.ConvertToInt64(-7.25),
				engine.ConvertToInt64(0.6), engine.ConvertToInt64(0.8),
			},
			want: rayCastResult{Hited: true, SpriteId: 42, PosX: 12.5, PosY: -7.25},
		},
		{
			name: "negative hit marker",
			data: []int64{-2, 43, 0, 0, 0, 0},
			want: rayCastResult{Hited: true, SpriteId: 43},
		},
		{name: "no hit", data: make([]int64, 6)},
		{
			name: "no hit retains payload",
			data: []int64{0, 44, engine.ConvertToInt64(-5), engine.ConvertToInt64(9), 0, 0},
			want: rayCastResult{SpriteId: 44, PosX: -5, PosY: 9},
		},
		{name: "nil", wantErr: "array type error: expected []int64 but got <nil>"},
		{name: "wrong type", data: make([]float64, 6), wantErr: "array type error: expected []int64 but got []float64"},
		{name: "empty", data: []int64{}, wantErr: "array len error: expected 6 but got 0"},
		{name: "missing normal", data: make([]int64, 4), wantErr: "array len error: expected 6 but got 4"},
		{name: "short", data: make([]int64, 5), wantErr: "array len error: expected 6 but got 5"},
		{name: "long", data: make([]int64, 7), wantErr: "array len error: expected 6 but got 7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := tryRaycastResult(test.data)
			if test.wantErr != "" {
				if got != nil || err == nil || err.Error() != test.wantErr {
					t.Fatalf("result = %+v, error = %v; want nil, %q", got, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || *got != test.want {
				t.Fatalf("raycast result = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestGameDebugDrawingPrimitives(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := gdx.DebugMgr
	manager := &gameDebugDrawMgr{}
	gdx.DebugMgr = manager
	t.Cleanup(func() { gdx.DebugMgr = previous })
	var game Game
	color := NewColor("#123456")
	game.DebugDrawRect(1, 2, 3, 4, color)
	game.DebugDrawCircle(5, 6, 7, color)
	game.DebugDrawLine(8, 9, 10, 11, color)
	want := [][2]mathf.Vec2{{mathf.NewVec2(1, 2), mathf.NewVec2(3, 4)}, {mathf.NewVec2(5, 6), mathf.NewVec2(7, 7)}, {mathf.NewVec2(8, 9), mathf.NewVec2(10, 11)}}
	if !slices.Equal(manager.lines, want) {
		t.Fatalf("draw commands=%v", manager.lines)
	}
	for _, got := range manager.colors {
		if got != mathf.NewColorRGBAi(0x12, 0x34, 0x56, 0xff) {
			t.Fatalf("draw color=%v", got)
		}
	}
}

func TestGameCollisionSkipsInvalidSprites(t *testing.T) {
	game, _, sprite := setupGameQueryPhysics(t)
	if got := game.buildSpriteCollisionData([]Sprite{nil}); len(got) != 0 {
		t.Fatalf("nil sprite collision=%v", got)
	}
	sprite.runtimeState.syncSprite.Target = (*SpriteImpl)(nil)
	if got := game.checkCollision([]engine.Object{sprite.getSpriteId()}); len(got) != 0 {
		t.Fatalf("nil target collision=%v", got)
	}
}

func TestApplyPhysicsSettingsForwardsCollisionMode(t *testing.T) {
	game := setupStageLoadTestGame(t, nil)
	mgr := gdx.PhysicsMgr.(*gameTestPhysicsMgr)
	spriteMgr := gdx.SpriteMgr.(*gameTestSpriteMgr)
	spriteMgr.pixelCollisionSamplingSteps = nil
	want := []bool{true, false, true}
	for _, byPixel := range want {
		game.applyPhysicsSettings(coreproject.SystemSettings{
			CollisionByPixel:        byPixel,
			PixelCollisionPrecision: collision.PixelPrecisionHigh,
		})
	}
	if !slices.Equal(mgr.collisionModes, want) {
		t.Fatalf("collision modes = %v, want %v", mgr.collisionModes, want)
	}
	if wantSteps := []int64{collision.PixelPrecisionHigh, collision.PixelPrecisionHigh, collision.PixelPrecisionHigh}; !slices.Equal(spriteMgr.pixelCollisionSamplingSteps, wantSteps) {
		t.Fatalf("pixel collision sampling steps = %v, want %v", spriteMgr.pixelCollisionSamplingSteps, wantSteps)
	}
}

func (m *gameQueryPhysicsMgr) CheckCollisionRect(pos, size mathf.Vec2, mask int64) engine.Array {
	m.from, m.to, m.mask = pos, size, mask
	return m.result
}

func (m *gameQueryPhysicsMgr) CheckCollisionCircle(pos mathf.Vec2, radius float64, mask int64) engine.Array {
	m.from, m.radius, m.mask = pos, radius, mask
	return m.result
}

func (m *gameQueryPhysicsMgr) RaycastWithDetails(from, to mathf.Vec2, ignore engine.Array, mask int64, areas, bodies bool) engine.Array {
	m.from, m.to, m.mask = from, to, mask
	m.ignore = ignore.([]int64)
	m.areas, m.bodies = areas, bodies
	return m.result
}

func (m *gameDebugDrawMgr) DebugDrawLine(from, to mathf.Vec2, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{from, to})
	m.colors = append(m.colors, color)
}

func (m *gameDebugDrawMgr) DebugDrawRect(pos, size mathf.Vec2, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{pos, size})
	m.colors = append(m.colors, color)
}

func (m *gameDebugDrawMgr) DebugDrawCircle(pos mathf.Vec2, radius float64, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{pos, mathf.NewVec2(radius, radius)})
	m.colors = append(m.colors, color)
}

func setupGameQueryPhysics(t *testing.T) (*Game, *gameQueryPhysicsMgr, *targetLookupSprite) {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := gdx.PhysicsMgr
	manager := &gameQueryPhysicsMgr{}
	gdx.PhysicsMgr = manager
	t.Cleanup(func() { gdx.PhysicsMgr = previous })
	game := &Game{}
	sprite := newTargetLookupSprite(game, "target", false)
	const id = 89001
	proxy := &engine.Sprite{Sprite: gdx.Sprite{Id: id}, Target: &sprite.SpriteImpl}
	sprite.runtimeState.syncSprite = proxy
	registry := gdx.Sprites()
	old, existed := registry[id]
	registry[id] = proxy
	t.Cleanup(func() {
		if existed {
			registry[id] = old
		} else {
			delete(registry, id)
		}
	})
	return game, manager, sprite
}

func TestCollisionLayersReuseSlotsAndCurrentTargets(t *testing.T) {
	for _, count := range []int{0, 1, 32, 33, 64, 65} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			setupRuntimeScheduler(t)
			game := &Game{isAutoSetCollisionLayer: true, typs: make(map[string]reflect.Type)}
			for i := range count {
				game.typs[fmt.Sprintf("Sprite%d", i)] = reflect.TypeFor[collisionLayerOrderSprite]()
			}
			game.sprCollisionInfos = game.buildSpriteCollisionInfos()
			groups := make(map[int64][]string)
			for name, info := range game.sprCollisionInfos {
				if info.Layer <= 0 || info.Layer > 1<<31 || bits.OnesCount64(uint64(info.Layer)) != 1 {
					t.Fatalf("%s layer = %d, want one of the 32 engine bits", name, info.Layer)
				}
				groups[info.Layer] = append(groups[info.Layer], name)
			}
			for slot := range min(count, 32) {
				want := count / 32
				if slot < count%32 {
					want++
				}
				if got := len(groups[1<<slot]); got != want {
					t.Fatalf("slot %d has %d types, want %d", slot, got, want)
				}
			}
			var sprites []Sprite
			for slot := range min(count, 32) {
				for instance, name := range groups[1<<slot] {
					sprite := newCollisionLayerOrderSprite(game, name, nil)
					target := groups[1<<((slot+instance+1)%min(count, 32))][0]
					sprite.physics().collisionTargets[target] = true
					sprites = append(sprites, sprite)
				}
			}
			if count > 0 {
				// A second instance of the same type contributes to the same shared info.
				sprite := newCollisionLayerOrderSprite(game, groups[1][0], nil)
				sprite.physics().collisionTargets[groups[1<<(min(count, 32)-1)][0]] = true
				sprites = append(sprites, sprite)
			}
			game.setupCollisionData(append(sprites, nil))
			if len(game.sprCollisionData) != len(sprites) {
				t.Fatalf("collision records = %d, want %d", len(game.sprCollisionData), len(sprites))
			}
			for i, data := range game.sprCollisionData {
				if data.sprite != spriteOf(sprites[i]) || data.info != game.sprCollisionInfos[data.sprite.name] {
					t.Fatal("collision records lost sprite order or shared info")
				}
			}
			checkMasks := func() {
				t.Helper()
				want := make(map[int64]int64)
				for _, sprite := range sprites {
					spr := spriteOf(sprite)
					layer := game.sprCollisionInfos[spr.name].Layer
					for target := range spr.physics().collisionTargets {
						want[layer] |= game.sprCollisionInfos[target].Layer
					}
				}
				for name, info := range game.sprCollisionInfos {
					if info.Mask != want[info.Layer] {
						t.Fatalf("%s mask = %#x, want %#x", name, info.Mask, want[info.Layer])
					}
				}
			}
			game.refreshCollisionLayers()
			checkMasks()
			if count > 0 {
				spr := spriteOf(sprites[0])
				spr.physics().addCollisionTarget(spr.name)
				checkMasks()
				clear(spr.physics().collisionTargets)
				game.refreshCollisionLayers()
				checkMasks()
			}
		})
	}
}

func TestCollisionLayersCommitAllMasksBeforeProxyWrites(t *testing.T) {
	setupRuntimeScheduler(t)
	setupImmediateEngineCalls(t)
	game := &Game{isAutoSetCollisionLayer: true, sprCollisionInfos: map[string]*spriteCollisionInfo{
		"A": {layerIndex: 0, Layer: 1}, "B": {layerIndex: 1, Layer: 2},
	}}
	a := newCollisionLayerOrderSprite(game, "A", nil)
	b := newCollisionLayerOrderSprite(game, "B", nil)
	a.physics().collisionTargets["B"] = true
	b.physics().collisionTargets["A"] = true
	var writes []int64
	var layerWrites []gdx.Object
	manager := &collisionLayerSpriteMgr{onLayer: func(id gdx.Object, _ int64) {
		if game.sprCollisionInfos["A"].Mask != 2 || game.sprCollisionInfos["B"].Mask != 1 {
			t.Fatal("proxy write preceded complete mask commit")
		}
		layerWrites = append(layerWrites, id)
	}, onMask: func(_ gdx.Object, mask int64) {
		writes = append(writes, mask)
	}}
	previous := gdx.SpriteMgr
	gdx.SpriteMgr = manager
	t.Cleanup(func() { gdx.SpriteMgr = previous })
	for i, sprite := range []*collisionLayerOrderSprite{a, b} {
		sprite.physics().triggerInfo.Type = physicsColliderNone
		sprite.runtimeState.syncSprite = &engine.Sprite{Sprite: gdx.Sprite{Id: int64(i + 1)}}
	}
	game.setupCollisionData([]Sprite{a, b})
	game.refreshCollisionLayers()
	if !slices.Equal(layerWrites, []gdx.Object{1, 2}) || !slices.Equal(writes, []int64{2, 1}) {
		t.Fatalf("proxy IDs = %v, masks = %v; want [1 2], [2 1]", layerWrites, writes)
	}
}

type collisionLayerSpriteMgr struct {
	spyCloneSpriteMgr
	onLayer func(gdx.Object, int64)
	onMask  func(gdx.Object, int64)
}

func (m *collisionLayerSpriteMgr) SetTriggerMask(id gdx.Object, mask int64) {
	m.onMask(id, mask)
}

func (m *collisionLayerSpriteMgr) SetCollisionLayer(id gdx.Object, layer int64) {
	m.onLayer(id, layer)
}

type tilemapUnloadSceneMgr struct {
	gdx.ISceneMgr
	clearCalls int
}

type tilemapAPIMgr struct {
	gdx.ITilemapMgr
	positions []float32
	path      string
	pos       mathf.Vec2
	layer     int64
	erased    bool
	collision []float32
}

type decoratorSceneMgr struct {
	gdx.ISceneMgr
	path                                  string
	position, pivot, scale, colliderPivot mathf.Vec2
	rotation                              float64
	layer, collider                       int64
	params                                []float64
}

type decoratorResMgr struct{ gdx.IResMgr }

func TestLoadStageWithNativeTilemapSkipsBackdrops(t *testing.T) {
	for _, test := range []struct{ name, decorators string }{
		{"missing decorators", ""},
		{"empty decorators", `{"decorators":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := gameTestFS{"tilemaps/map/tilemap.json": `{}`}
			if test.decorators != "" {
				files["tilemaps/map/decorator.json"] = test.decorators
			}
			game := setupStageLoadTestGame(t, files)
			previousParser := gdx.TilemapparserMgr
			parser := &gameTestTilemapparserMgr{}
			gdx.TilemapparserMgr = parser
			t.Cleanup(func() { gdx.TilemapparserMgr = previousParser })
			project := &coreproject.ProjectConfig{
				TilemapPath: "tilemaps/map",
				Map:         coreproject.MapConfig{Width: 640, Height: 480},
				Backdrops: []*coreproject.BackdropConfig{{CostumeConfig: coreproject.CostumeConfig{
					Name: "unused", Path: "unused.png", ImageWidth: 10, ImageHeight: 10,
				}}},
			}
			gamer := reflect.ValueOf(game).Elem()
			plan, err := game.prepareProject(files, project, gamer)
			if err != nil {
				t.Fatal(err)
			}
			game.tilemapMgr.applyLoadedMap(plan.tilemap)
			game.applyPreparedStage(gamer, project, game.bootstrapGeneration(), &plan)
			if game.TilemapName() != "map" || parser.loadedPath != engine.ToAssetPath("tilemaps/map/tilemap.json") {
				t.Fatalf("native tilemap name/path = %q/%q", game.TilemapName(), parser.loadedPath)
			}
			if game.getCostumePath() != "" || game.displayState.WorldWidth != 640 || game.displayState.WorldHeight != 480 {
				t.Fatalf("tilemap stage used backdrop: path=%q, world=%dx%d",
					game.getCostumePath(), game.displayState.WorldWidth, game.displayState.WorldHeight)
			}
			if parser.destroyCalls != 0 {
				t.Fatalf("initial tilemap was destroyed %d times", parser.destroyCalls)
			}
		})
	}
}

func TestUnloadTilemapClearsStateOnce(t *testing.T) {
	setupImmediateEngineCalls(t)
	for _, tt := range []struct {
		name        string
		newLoader   bool
		wantDestroy int
	}{
		{"legacy", false, 0},
		{"native", true, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			previousScene, previousParser := gdx.SceneMgr, gdx.TilemapparserMgr
			scene := &tilemapUnloadSceneMgr{}
			parser := &gameTestTilemapparserMgr{}
			gdx.SceneMgr, gdx.TilemapparserMgr = scene, parser
			t.Cleanup(func() { gdx.SceneMgr, gdx.TilemapparserMgr = previousScene, previousParser })
			var game Game
			game.tilemapMgr.currentMap = "map1"
			game.tilemapMgr.tilemapDir = "tilemaps/map1"
			game.tilemapMgr.useNewLoader = tt.newLoader
			game.UnloadTilemap()
			game.UnloadTilemap()
			if game.TilemapName() != "" || game.tilemapMgr.tilemapDir != "" || game.tilemapMgr.useNewLoader || game.tilemapMgr.hasData() {
				t.Fatalf("unloaded map retained state: %+v", game.tilemapMgr)
			}
			if scene.clearCalls != 1 || parser.destroyCalls != tt.wantDestroy {
				t.Fatalf("cleanup calls: scene=%d, native=%d; want 1, %d", scene.clearCalls, parser.destroyCalls, tt.wantDestroy)
			}
		})
	}
}

func TestGameTilePlacementAndQueries(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := gdx.TilemapMgr
	manager := &tilemapAPIMgr{}
	gdx.TilemapMgr = manager
	t.Cleanup(func() { gdx.TilemapMgr = previous })
	var game Game
	game.PlaceTiles__0([]float64{1.5, 2.5, 3.5, 4.5}, "tile.png")
	if !reflect.DeepEqual(manager.positions, []float32{1.5, 2.5, 3.5, 4.5}) || manager.path != engine.ToAssetPath("tile.png") {
		t.Fatalf("placement=%+v", manager)
	}
	game.PlaceTile(4, 5, "tile.png")
	if got := game.GetTile__0(4, 5); got != engine.ToAssetPath("tile.png") || manager.pos != mathf.NewVec2(4, 5) {
		t.Fatalf("tile=%q, position=%v", got, manager.pos)
	}
	if got := game.GetTile__1(4, 5, 2); got != manager.path || manager.layer != 2 {
		t.Fatalf("layered tile=%q, layer=%d", got, manager.layer)
	}
	game.EraseTile__0(4, 5)
	if game.GetTile__0(4, 5) != "" {
		t.Fatal("erased tile still present")
	}
	game.PlaceTile(4, 5, "tile.png")
	game.EraseTile__1(4, 5, 2)
	if game.GetTile__1(4, 5, 2) != "" {
		t.Fatal("erased layer tile still present")
	}
	game.setTileInfo("tile.png", []float64{0, 0, 1, 0, 1, 1})
	if !reflect.DeepEqual(manager.collision, []float32{0, 0, 1, 0, 1, 1}) {
		t.Fatalf("collision points=%v", manager.collision)
	}
}

func TestTilemapDecoratorCoordinates(t *testing.T) {
	setupImmediateEngineCalls(t)
	previousScene, previousRes := gdx.SceneMgr, gdx.ResMgr
	scene := &decoratorSceneMgr{}
	gdx.SceneMgr, gdx.ResMgr = scene, &decoratorResMgr{}
	t.Cleanup(func() { gdx.SceneMgr, gdx.ResMgr = previousScene, previousRes })
	var game Game
	game.tilemapMgr = gameTilemapMgr{g: &game, useNewLoader: true, tilemapDir: "tilemaps/map", decoratorDatas: &DecoratorJSON{Decorators: []tm.DecoratorNode{{Path: "tree.png", Position: tm.Vec2{X: 3, Y: 4}, Pivot: tm.Vec2{X: 12, Y: 8}, Scale: tm.Vec2{X: 2, Y: 3}, Ratation: 120, ZIndex: 7, ColliderType: "rect", ColliderPivot: tm.Vec2{X: 1, Y: 2}, ColliderParams: []float64{4, 5}}}}}
	game.tilemapMgr.parseTilemap()
	if scene.path != engine.ToAssetPath("tilemaps/map/tree.png") || scene.position != mathf.NewVec2(3, 4) || scene.pivot != mathf.NewVec2(2, 3) || scene.rotation != 30 || scene.scale != mathf.NewVec2(2, 3) || scene.layer != 7 || scene.collider != physicsColliderRect || scene.colliderPivot != mathf.NewVec2(13, 10) || !reflect.DeepEqual(scene.params, []float64{4, 5}) {
		t.Fatalf("decorator=%+v", scene)
	}
}

func TestTilemapLoadAndEmptyBounds(t *testing.T) {
	var game Game
	files := gameTestFS{"map.json": `{"tilemap":{"layers":[]}}`}
	game.tilemapMgr.g, game.tilemapMgr.fs = &game, files
	game.tilemapMgr.loadMap("map.json")
	if game.TilemapName() != "map" {
		t.Fatalf("map name=%q", game.TilemapName())
	}
	game.tilemapMgr.loadMap("")
	if game.TilemapName() != "map" {
		t.Fatal("empty path replaced map")
	}
	game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
	for _, raw := range []string{`{}`, `{"tilemap":{"layers":[{"tile_data":[]}]}}`, `null`} {
		var data *tm.TscnMapData
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatal(err)
		}
		game.tilemapMgr.datas = data
		game.tilemapMgr.calcWorldSize()
		if game.displayState.WorldWidth != 480 || game.displayState.WorldHeight != 360 {
			t.Fatal("empty bounds changed world")
		}
	}
	defer func() {
		if got := recover(); got == nil || !strings.Contains(fmt.Sprint(got), "Failed to load tilemap") {
			t.Fatalf("missing map panic=%v", got)
		}
	}()
	game.tilemapMgr.loadMap("missing.json")
}

func (m *tilemapUnloadSceneMgr) ClearPureSprites() { m.clearCalls++ }

func (m *tilemapAPIMgr) PlaceTiles(positions gdx.Array, path string) {
	m.positions = positions.([]float32)
	m.path = path
}

func (m *tilemapAPIMgr) PlaceTile(pos mathf.Vec2, path string) {
	m.pos, m.path, m.erased = pos, path, false
}

func (m *tilemapAPIMgr) EraseTile(pos mathf.Vec2) { m.pos, m.erased = pos, true }

func (m *tilemapAPIMgr) EraseTileWithLayer(pos mathf.Vec2, layer int64) {
	m.pos, m.layer, m.erased = pos, layer, true
}

func (m *tilemapAPIMgr) GetTile(pos mathf.Vec2) string {
	m.pos = pos
	if m.erased {
		return ""
	}
	return m.path
}

func (m *tilemapAPIMgr) GetTileWithLayer(pos mathf.Vec2, layer int64) string {
	m.layer = layer
	return m.GetTile(pos)
}

func (m *tilemapAPIMgr) SetTileWithCollisionInfo(path string, points gdx.Array) {
	m.path, m.collision = path, points.([]float32)
}

func (m *decoratorSceneMgr) CreateStaticSprite(path string, pos mathf.Vec2, rotation float64, scale mathf.Vec2, layer int64, pivot mathf.Vec2, collider int64, colliderPivot mathf.Vec2, params gdx.Array) gdx.Object {
	m.path, m.position, m.rotation, m.scale, m.layer, m.pivot, m.collider, m.colliderPivot, m.params = path, pos, rotation, scale, layer, pivot, collider, colliderPivot, params.([]float64)
	return 991
}

func (*decoratorResMgr) GetImageSize(string) mathf.Vec2 { return mathf.NewVec2(20, 10) }

type cameraTestMgr struct {
	gdx.ICameraMgr
	position, zoom mathf.Vec2
	viewport       mathf.Rect2
	limits         map[int64]int64
	smoothing      bool
	positionWrites int
}

func TestCameraCoordinatesAndWindowScale(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	game := &Game{}
	game.displayState.WindowScale = 2.5
	camera := &cameraImpl{g: game, followTarget: Mouse}

	camera.SetZoom(1.5)
	if mgr.zoom != mathf.NewVec2(3.75, 3.75) || camera.Zoom() != 1.5 {
		t.Fatalf("zoom = %v, logical zoom = %v; want (3.75, 3.75) and 1.5", mgr.zoom, camera.Zoom())
	}
	camera.SetXYpos(12, -8)
	if camera.Xpos() != 12 || camera.Ypos() != -8 {
		t.Fatalf("camera position = (%v, %v), want (12, -8)", camera.Xpos(), camera.Ypos())
	}
	camera.ChangeXYpos(-2, 3)
	if mgr.position != mathf.NewVec2(10, -5) || camera.followTarget != nil {
		t.Fatalf("manual movement = %v, follow = %v; want (10, -5) and no follow target", mgr.position, camera.followTarget)
	}
	if !camera.isDirty || camera.dirtyVersion != 3 {
		t.Fatalf("camera invalidation = (%v, %d), want (true, 3)", camera.isDirty, camera.dirtyVersion)
	}
	mgr.viewport = mathf.Rect2{Position: mathf.NewVec2(-90, 25), Size: mathf.NewVec2(320, 180)}
	x, y, width, height := camera.ViewportRect()
	if x != -90 || y != 25 || width != 320 || height != 180 {
		t.Fatalf("viewport = (%v, %v, %v, %v), want (-90, 25, 320, 180)", x, y, width, height)
	}
}

func TestCameraLimitsRespectOffsetWorldCoordinates(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	game := &Game{}
	game.displayState.WindowScale = 2
	game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
	game.displayState.MinWorldX, game.displayState.MinWorldY = -120, -70
	camera := &cameraImpl{}
	camera.init(game)
	want := map[int64]int64{
		int64(sideLeft): -120, int64(sideRight): 360,
		int64(sideTop): -290, int64(sideBottom): 70,
	}
	if !maps.Equal(mgr.limits, want) || !mgr.smoothing {
		t.Fatalf("camera limits = %v, smoothing = %v; want %v and true", mgr.limits, mgr.smoothing, want)
	}
	if camera.Zoom() != 1 {
		t.Fatalf("initial logical zoom = %v, want 1", camera.Zoom())
	}
}

func TestCameraFollowUpdatesOnlyWhenTargetChanges(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	sprite := newTestTransformSprite(12, -8)
	camera := &cameraImpl{g: sprite.g}
	camera.follow(sprite)
	camera.onUpdate()
	if mgr.position != mathf.NewVec2(12, -8) || mgr.positionWrites != 1 {
		t.Fatalf("first follow update = %v, writes = %d", mgr.position, mgr.positionWrites)
	}
	camera.setDirtyFlag(false)
	camera.onUpdate()
	if mgr.positionWrites != 1 {
		t.Fatal("unchanged sprite caused another camera update")
	}
	sprite.SetXYpos(21, 34)
	camera.onUpdate()
	if mgr.position != mathf.NewVec2(21, 34) || mgr.positionWrites != 2 {
		t.Fatalf("changed follow update = %v, writes = %d", mgr.position, mgr.positionWrites)
	}
	if camera.observedFollowVersion != sprite.spriteState.visualVersion {
		t.Fatal("camera did not acknowledge the target's visual version")
	}
	camera.follow(nil)
	camera.onUpdate()
	if mgr.positionWrites != 2 {
		t.Fatal("detaching the follow target moved the camera")
	}
}

func TestCameraFollowResolvesTargetsWithoutReplacingOnMissingName(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "target")
	camera := &cameraImpl{g: game}
	camera.Follow__0(sprite)
	if camera.followTarget != &sprite.SpriteImpl {
		t.Fatal("follow did not resolve a generated sprite to its implementation")
	}
	camera.Follow__1("target")
	version := camera.dirtyVersion
	for _, target := range []any{SpriteName("missing"), Edge} {
		camera.follow(target)
		if camera.followTarget != &sprite.SpriteImpl || camera.dirtyVersion != version {
			t.Fatalf("unsupported target %v changed the active follow target", target)
		}
	}
	camera.follow(Mouse)
	game.inputMgr.mousePos = mathf.NewVec2(45, 67)
	if changed, pos := camera.getFollowPos(); !changed || pos != mathf.NewVec2(45, 67) {
		t.Fatalf("mouse follow = (%v, %v), want (true, (45, 67))", changed, pos)
	}
}

func TestCameraFollowRejectsUnsupportedType(t *testing.T) {
	camera := &cameraImpl{followTarget: Mouse}
	defer func() {
		if recover() == nil {
			t.Fatal("unsupported follow target did not panic")
		}
		if camera.followTarget != Mouse || camera.dirtyVersion != 0 {
			t.Fatal("invalid target changed camera state before failing")
		}
	}()
	camera.follow(42)
}

func TestCameraWithoutFollowTargetHasNoPositionUpdate(t *testing.T) {
	camera := &cameraImpl{}
	if changed, position := camera.getFollowPos(); changed || position != (mathf.Vec2{}) {
		t.Fatalf("detached camera follow position = (%v, %v), want false and origin", changed, position)
	}
}

func TestCameraFollowObservesPhysicsPositionChanges(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)
	camera := &cameraImpl{
		followTarget:          sprite,
		observedFollowVersion: sprite.spriteState.visualVersion,
	}

	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("unchanged follow target unexpectedly invalidated the camera")
	}

	sprite.mergeEnginePosition(3, 4)
	changed, pos := camera.getFollowPos()
	if !changed {
		t.Fatal("physics position change did not invalidate the camera")
	}
	if pos.X != 3 || pos.Y != 4 {
		t.Fatalf("camera follow position = (%v, %v), want (3, 4)", pos.X, pos.Y)
	}

	camera.observedFollowVersion = sprite.spriteState.visualVersion
	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("observed follow target still invalidated the camera")
	}
}

func TestCameraFollowObservesMousePositionChanges(t *testing.T) {
	game := new(Game)
	camera := &cameraImpl{g: game, followTarget: Mouse}

	for _, want := range []mathf.Vec2{mathf.NewVec2(1, 2), mathf.NewVec2(3, 4)} {
		game.inputMgr.mousePos = want
		changed, pos := camera.getFollowPos()
		if !changed {
			t.Fatal("mouse follow did not update without a camera dirty flag")
		}
		if pos != want {
			t.Fatalf("camera follow position = %v, want cached mouse position %v", pos, want)
		}
	}
}

func (m *cameraTestMgr) GetCameraPosition() mathf.Vec2 { return m.position }

func (m *cameraTestMgr) SetCameraPosition(pos mathf.Vec2) {
	m.position = pos
	m.positionWrites++
}

func (m *cameraTestMgr) GetCameraZoom() mathf.Vec2 { return m.zoom }

func (m *cameraTestMgr) SetCameraZoom(zoom mathf.Vec2) { m.zoom = zoom }

func (m *cameraTestMgr) GetGlobalCameraRect() mathf.Rect2 { return m.viewport }

func (m *cameraTestMgr) SetCameraLimit(side, value int64) { m.limits[side] = value }

func (m *cameraTestMgr) SetCameraSmoothing(enabled bool) { m.smoothing = enabled }

func setupCameraTestMgr(t *testing.T) *cameraTestMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := gdx.CameraMgr
	mgr := &cameraTestMgr{limits: make(map[int64]int64)}
	gdx.CameraMgr = mgr
	t.Cleanup(func() { gdx.CameraMgr = previous })
	return mgr
}
