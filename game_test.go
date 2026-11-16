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
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"syscall"
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
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	pkgengine.IPlatformMgr
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
	pkgengine.INavigationMgr
	grid, cell, from, to mathf.Vec2
	jump, debug          bool
	setups               int
}

type gameBackdropSpriteMgr struct {
	pkgengine.ISpriteMgr
	repeat        mathf.Vec4
	position      mathf.Vec2
	positionCalls int
}

type gameQueryPhysicsMgr struct {
	pkgengine.IPhysicsMgr
	result        engine.Array
	from, to      mathf.Vec2
	radius        float64
	mask          int64
	ignore        []int64
	areas, bodies bool
}

type gameDebugDrawMgr struct {
	pkgengine.IDebugMgr
	lines  [][2]mathf.Vec2
	colors []mathf.Color
}

type touchFilterSpriteMgr struct {
	spyCloneSpriteMgr
	onLayer func(pkgengine.Object, int64)
	onMask  func(pkgengine.Object, int64)
}

type tilemapUnloadSceneMgr struct {
	pkgengine.ISceneMgr
	clearCalls int
}

type tilemapAPIMgr struct {
	pkgengine.ITilemapMgr
	positions []float32
	path      string
	pos       mathf.Vec2
	layer     int64
	erased    bool
	collision []float32
}

type decoratorSceneMgr struct {
	pkgengine.ISceneMgr
	path                                  string
	position, pivot, scale, colliderPivot mathf.Vec2
	rotation                              float64
	layer, collider                       int64
	params                                []float64
}

type decoratorResMgr struct{ pkgengine.IResMgr }

type cameraTestMgr struct {
	pkgengine.ICameraMgr
	position, zoom mathf.Vec2
	viewport       mathf.Rect2
	limits         map[int64]int64
	smoothing      bool
	positionWrites int
}

type bubbleLayoutCameraMgr struct{ pkgengine.ICameraMgr }

const (
	askScenePath = "res://engine/ui/UiAsk.tscn"
	sayScenePath = "res://engine/ui/UiSay.tscn"
)

type askTestBinding struct {
	parent pkgengine.Object
	path   string
}

type askTestUiMgr struct {
	pkgengine.IUiMgr
	nextID       pkgengine.Object
	ids          map[pkgengine.Object]struct{}
	created      map[string]int
	bound        map[askTestBinding]pkgengine.Object
	visible      map[pkgengine.Object]bool
	text         map[pkgengine.Object]string
	askRoot      pkgengine.Object
	onAskVisible func()
	onDestroy    func()
}

type bubbleRenderTestUiMgr struct {
	*askTestUiMgr
	positions map[pkgengine.Object]mathf.Vec2
	sizes     map[pkgengine.Object]mathf.Vec2
}

type askDelayedInputMgr struct{ pkgengine.IInputMgr }

type monitorEvalFixture struct {
	Game
	score float64
	unset any
	calls int
}

type monitorPanelSpy struct {
	visible bool
	events  []string
	value   ui.MonitorValue
	scale   float64
	pos     mathf.Vec2
}

type SliderMonitorGame struct {
	Game
	score float64
	Dot   sliderMonitorSprite
}

type sliderMonitorSprite struct {
	SpriteImpl
	*SliderMonitorGame
	score float64
}

type gameWidgetUiMgr struct {
	*askTestUiMgr
	rangeValue float64
}

type ListMonitorGame struct {
	Game
	items  List
	values []string
	Monkey listMonitorSprite
}

type listMonitorSprite struct {
	SpriteImpl
	*ListMonitorGame
	items List
}

// TestMain supplies the raw manager normally installed by gdengine.Link.
func TestMain(m *testing.M) {
	pkgengine.PlatformMgr = mainThreadTestPlatform{}
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
	game.scene.init()
	game.inputMgr.init(game)
	game.inputMgr.mousePos = mathf.Vec2{X: 17, Y: -23}
	original := newTargetLookupSprite(game, "actor", false)
	original.components.transform = &transformComponent{sprite: &original.SpriteImpl, x: 12, y: -34}
	clone := newTargetLookupSprite(game, "actor", true)
	clone.components.transform = &transformComponent{sprite: &clone.SpriteImpl, x: 56, y: 78}
	game.scene.add(&clone.SpriteImpl)
	game.scene.add(&original.SpriteImpl)
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
				visualObject: visualObject{costumes: []*costume{newCostume(&coreproject.CostumeConfig{
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
			game := &Game{visualObject: visualObject{costumes: test.costumes, costumeIndex: test.initial}}
			SetRandomSeed(1)
			t.Cleanup(ResetRandomSeed)

			if !game.selectBackdrop(test.selector) {
				t.Fatal("selectBackdrop(Random) = false, want true")
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
			game := &Game{visualObject: visualObject{costumes: test.costumes}}
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
	game.scene.add(42)
	game.scene.add(&Monitor{name: "other"})
	game.scene.add(first)
	game.scene.add(&Monitor{name: "score"})
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
	game.scene.add(&clone.SpriteImpl)
	game.scene.add(&target.SpriteImpl)

	if got := game.GetTarget("Hero"); got != &target.SpriteImpl {
		t.Fatalf("GetTarget(%q) = %v, want %v", "Hero", got, &target.SpriteImpl)
	}
}

func TestGameGetTargetReturnsNilWithoutMatch(t *testing.T) {
	var game Game
	game.scene.add(&newTargetLookupSprite(&game, "Hero", true).SpriteImpl)

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
	game.scene.add(&target.SpriteImpl)
	game.scene.add(&clone.SpriteImpl)

	game.scene.removeShape(&target.SpriteImpl)

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
	game.scene.init()
	game.Value = *newCollisionLayerOrderSprite(&game.Game, "Value", nil)
	game.Pointer = newCollisionLayerOrderSprite(&game.Game, "Pointer", nil)
	game.Dynamic = newCollisionLayerOrderSprite(&game.Game, "Dynamic", nil)
	back := newCollisionLayerOrderSprite(&game.Game, "Back", nil)
	front := newCollisionLayerOrderSprite(&game.Game, "Front", nil)
	prototype := newCollisionLayerOrderSprite(&game.Game, "collisionLayerOrderSprite", nil)
	prototype.visualObject.initWithSize(1, 1)
	prototype.physics().bodyCollider.Type = collision.ColliderNone
	prototype.physics().triggerCollider.Type = collision.ColliderNone
	game.spritePrototypes = map[string]Sprite{"Back": back, "Front": front, "collisionLayerOrderSprite": prototype}
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
	game.spriteTypes["Data"] = reflect.TypeFor[int]()
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
	if len(game.spritePrototypes) != 0 {
		t.Fatalf("cold load initialized unregistered sprites: %v", game.spritePrototypes)
	}
}

func TestColdStageKeepsLazyLoadFailureBeforeLaterParseFailure(t *testing.T) {
	var game Game
	game.scene.init()
	game.spritePrototypes = make(map[string]Sprite)
	game.spriteTypes = map[string]reflect.Type{"Sprite": reflect.TypeFor[collisionLayerOrderSprite]()}
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
	game.scene.init()
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
			game.scene.init()
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
		game.scene.init()
		sprite := newCollisionLayerOrderSprite(&game, "", nil)
		game.spritePrototypes = map[string]Sprite{"": sprite}
		sprites, err := game.loadStageEntries(reflect.Value{}, []any{""})
		if err != nil || len(sprites) != 1 || sprites[0] != sprite {
			t.Fatalf("empty named sprite: loaded=%v, error=%v", sprites, err)
		}
	})
}

func TestStageGroupValidatesAllItemsBeforeLoading(t *testing.T) {
	original := &collisionLayerOrderSprite{}
	game := &stagePlanGame{pointers: []*collisionLayerOrderSprite{original}}
	game.scene.init()
	game.spritePrototypes = make(map[string]Sprite)
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
	if len(game.pointers) != 1 || game.pointers[0] != original || len(game.getAllShapes()) != 0 || len(game.spritePrototypes) != 0 {
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
	if got, from := resolvePropertyTarget(game.propertyRootValue(), "Sprite"); from != 2 || got.Addr().Interface() != &game.Sprite {
		t.Fatalf("sprite scope=%v, %d", got, from)
	}
}

func TestGamePathFinderConfigurationAndQueries(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := pkgengine.NavigationMgr
	manager := &gameNavigationMgr{}
	pkgengine.NavigationMgr = manager
	t.Cleanup(func() { pkgengine.NavigationMgr = previous })
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
	game.visualObject.initWithSize(480, 360)
	manager := &gameBackdropSpriteMgr{ISpriteMgr: pkgengine.SpriteMgr}
	pkgengine.SpriteMgr = manager
	game.displayState.WorldWidth, game.displayState.WorldHeight = 960, 720
	game.costumes = []*costume{newCostumeWithSize(480, 360), newCostumeWithSize(480, 360)}
	game.costumes[0].name = "first"
	game.costumes[1].name = "second"
	game.runtimeState.proxy = &engine.Sprite{Sprite: pkgengine.Sprite{Id: 89801}}
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
	if game.graphicEffects[ColorEffect] != 15 {
		t.Fatalf("effect=%v", game.graphicEffects)
	}
	game.ClearGraphicEffects()
	for kind, value := range game.graphicEffects {
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
			game.spritePrototypes = map[string]Sprite{"layoutInvalidInitSprite": &layoutInvalidInitSprite{}}
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

func TestSpriteOverridesDistinguishMissingAndZeroValues(t *testing.T) {
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

	missing, err := parseSpriteOverrides(coreproject.StageShape{})
	if err != nil {
		t.Fatal(err)
	}
	unchanged := newSprite()
	applySpriteOverrides(unchanged, missing)
	if transform := unchanged.transform(); transform.x != 10 || transform.y != 20 ||
		transform.direction != 30 || transform.rotationStyle != LeftRight {
		t.Fatalf("missing properties changed transform: %+v", transform)
	}
	if !unchanged.spriteState.isVisible || unchanged.runtimeState.scale != 2 || unchanged.costumeIndex != 3 {
		t.Fatalf("missing properties changed sprite: %+v", unchanged)
	}

	zero, err := parseSpriteOverrides(coreproject.StageShape{
		"x": 0.0, "y": 0.0, "heading": 0.0,
		"rotationStyle": "none", "visible": false,
		"size": 0.0, "costumeIndex": 0.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	reset := newSprite()
	applySpriteOverrides(reset, zero)
	if transform := reset.transform(); transform.x != 0 || transform.y != 0 ||
		transform.direction != 0 || transform.rotationStyle != None {
		t.Fatalf("zero properties were not applied to transform: %+v", transform)
	}
	if reset.spriteState.isVisible || reset.runtimeState.scale != 0 || reset.costumeIndex != 0 {
		t.Fatalf("zero properties were not applied to sprite: %+v", reset)
	}
}

func TestParseSpriteOverridesRejectsWrongTypes(t *testing.T) {
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
			_, err := parseSpriteOverrides(coreproject.StageShape{test.name: test.value})
			if err == nil || !strings.Contains(err.Error(), "want "+test.want) {
				t.Fatalf("parseSpriteOverrides error = %v, want type %s", err, test.want)
			}
		})
	}
}

func TestGameIntersectionQueriesResolveLiveSprites(t *testing.T) {
	game, manager, sprite := setupGameQueryPhysics(t)
	// A missing engine object is ignored while the registered sprite is retained.
	manager.result = []engine.Object{-1, sprite.proxyID()}
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
	manager.result = []int64{1, sprite.proxyID(), engine.ConvertToInt64(12.5), engine.ConvertToInt64(-7.25), 0, 0}
	for _, test := range []struct {
		name   string
		call   func() (bool, Sprite, float64, float64)
		ignore []int64
	}{
		{"default", func() (bool, Sprite, float64, float64) { return game.Raycast__0(1, 2, 3, 4) }, nil},
		{"single", func() (bool, Sprite, float64, float64) { return game.Raycast__1(1, 2, 3, 4, sprite) }, []int64{sprite.proxyID()}},
		{"multiple with nil", func() (bool, Sprite, float64, float64) { return game.Raycast__2(1, 2, 3, 4, []Sprite{nil, sprite}) }, []int64{sprite.proxyID()}},
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
	previous := pkgengine.DebugMgr
	manager := &gameDebugDrawMgr{}
	pkgengine.DebugMgr = manager
	t.Cleanup(func() { pkgengine.DebugMgr = previous })
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

func TestManualModeClearsAutoTouchSetup(t *testing.T) {
	g := Game{physicsEnabled: true}
	g.touchFilters = map[string]*touchFilter{
		"SpriteA": {Layer: 1 << 0, Mask: 1 << 1},
	}
	g.touchEntries = []touchEntry{{
		filter: g.touchFilters["SpriteA"],
	}}

	g.setupTouchEntries(nil)

	if g.touchFilters != nil {
		t.Fatalf("touchFilters = %#v, want nil", g.touchFilters)
	}
	if g.touchEntries != nil {
		t.Fatalf("touchEntries = %#v, want nil", g.touchEntries)
	}
}

func TestTryRaycastResult(t *testing.T) {
	for _, test := range []struct {
		name    string
		data    engine.Array
		want    raycastResult
		wantErr string
	}{
		{
			name: "hit",
			data: []int64{
				1, 42,
				engine.ConvertToInt64(12.5), engine.ConvertToInt64(-7.25),
				engine.ConvertToInt64(0.6), engine.ConvertToInt64(0.8),
			},
			want: raycastResult{Hit: true, SpriteID: 42, PosX: 12.5, PosY: -7.25},
		},
		{
			name: "negative hit marker",
			data: []int64{-2, 43, 0, 0, 0, 0},
			want: raycastResult{Hit: true, SpriteID: 43},
		},
		{name: "no hit", data: make([]int64, 6)},
		{
			name: "no hit retains payload",
			data: []int64{0, 44, engine.ConvertToInt64(-5), engine.ConvertToInt64(9), 0, 0},
			want: raycastResult{SpriteID: 44, PosX: -5, PosY: 9},
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
	previous := pkgengine.DebugMgr
	manager := &gameDebugDrawMgr{}
	pkgengine.DebugMgr = manager
	t.Cleanup(func() { pkgengine.DebugMgr = previous })
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
	game.autoTouchLayers = true
	game.setupTouchEntries([]Sprite{nil})
	if got := game.touchEntries; len(got) != 0 {
		t.Fatalf("nil sprite collision=%v", got)
	}
	sprite.runtimeState.proxy.Target = (*SpriteImpl)(nil)
	if got := game.checkCollision([]engine.Object{sprite.proxyID()}); len(got) != 0 {
		t.Fatalf("nil target collision=%v", got)
	}
}

func TestApplyPhysicsSettingsForwardsCollisionMode(t *testing.T) {
	game := setupStageLoadTestGame(t, nil)
	mgr := pkgengine.PhysicsMgr.(*gameTestPhysicsMgr)
	spriteMgr := pkgengine.SpriteMgr.(*gameTestSpriteMgr)
	spriteMgr.pixelCollisionSamplingSteps = nil
	for _, byShape := range []bool{false, true, false} {
		game.applyPhysicsSettings(coreproject.SystemSettings{
			TouchByShape:            byShape,
			PixelCollisionPrecision: collision.PixelPrecisionHigh,
		})
		if game.touchByShape != byShape {
			t.Fatalf("runtime shape mode = %v, want %v", game.touchByShape, byShape)
		}
	}
	// The native API takes pixel mode, so only that boundary inverts the setting.
	if want := []bool{true, false, true}; !slices.Equal(mgr.collisionModes, want) {
		t.Fatalf("native pixel modes = %v, want %v", mgr.collisionModes, want)
	}
	if wantSteps := []int64{collision.PixelPrecisionHigh, collision.PixelPrecisionHigh, collision.PixelPrecisionHigh}; !slices.Equal(spriteMgr.pixelCollisionSamplingSteps, wantSteps) {
		t.Fatalf("pixel collision sampling steps = %v, want %v", spriteMgr.pixelCollisionSamplingSteps, wantSteps)
	}
}

func TestTouchFiltersReuseLayersAndCurrentTargets(t *testing.T) {
	for _, count := range []int{0, 1, 32, 33, 64, 65} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			setupRuntimeScheduler(t)
			game := &Game{autoTouchLayers: true, spriteTypes: make(map[string]reflect.Type)}
			for i := range count {
				game.spriteTypes[fmt.Sprintf("Sprite%d", i)] = reflect.TypeFor[collisionLayerOrderSprite]()
			}
			game.touchFilters = game.buildTouchFilters()
			groups := make(map[int64][]string)
			for name, info := range game.touchFilters {
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
					sprite.physics().touchTargets[target] = true
					sprites = append(sprites, sprite)
				}
			}
			if count > 0 {
				// A second instance of the same type contributes to the same shared info.
				sprite := newCollisionLayerOrderSprite(game, groups[1][0], nil)
				sprite.physics().touchTargets[groups[1<<(min(count, 32)-1)][0]] = true
				sprites = append(sprites, sprite)
			}
			game.setupTouchEntries(append(sprites, nil))
			if len(game.touchEntries) != len(sprites) {
				t.Fatalf("touch entries = %d, want %d", len(game.touchEntries), len(sprites))
			}
			for i, entry := range game.touchEntries {
				if entry.sprite != spriteOf(sprites[i]) || entry.filter != game.touchFilters[entry.sprite.name] {
					t.Fatal("touch entries lost sprite order or shared filters")
				}
			}
			checkMasks := func() {
				t.Helper()
				want := make(map[int64]int64)
				for _, sprite := range sprites {
					spr := spriteOf(sprite)
					layer := game.touchFilters[spr.name].Layer
					for target := range spr.physics().touchTargets {
						want[layer] |= game.touchFilters[target].Layer
					}
				}
				for name, info := range game.touchFilters {
					if info.Mask != want[info.Layer] {
						t.Fatalf("%s mask = %#x, want %#x", name, info.Mask, want[info.Layer])
					}
				}
			}
			game.refreshTouchFilters()
			checkMasks()
			if count > 0 {
				spr := spriteOf(sprites[0])
				spr.physics().addTouchTargets(spr.name)
				checkMasks()
				clear(spr.physics().touchTargets)
				game.refreshTouchFilters()
				checkMasks()
			}
		})
	}
}

func TestTouchFiltersCommitBeforeProxyWrites(t *testing.T) {
	setupRuntimeScheduler(t)
	setupImmediateEngineCalls(t)
	game := &Game{autoTouchLayers: true, touchFilters: map[string]*touchFilter{
		"A": {Layer: 1}, "B": {Layer: 2},
	}}
	a := newCollisionLayerOrderSprite(game, "A", nil)
	b := newCollisionLayerOrderSprite(game, "B", nil)
	a.physics().touchTargets["B"] = true
	b.physics().touchTargets["A"] = true
	var writes []int64
	var layerWrites []pkgengine.Object
	manager := &touchFilterSpriteMgr{onLayer: func(id pkgengine.Object, _ int64) {
		if game.touchFilters["A"].Mask != 2 || game.touchFilters["B"].Mask != 1 {
			t.Fatal("proxy write preceded complete mask commit")
		}
		layerWrites = append(layerWrites, id)
	}, onMask: func(_ pkgengine.Object, mask int64) {
		writes = append(writes, mask)
	}}
	previous := pkgengine.SpriteMgr
	pkgengine.SpriteMgr = manager
	t.Cleanup(func() { pkgengine.SpriteMgr = previous })
	for i, sprite := range []*collisionLayerOrderSprite{a, b} {
		sprite.physics().triggerCollider.Type = collision.ColliderNone
		sprite.runtimeState.proxy = &engine.Sprite{Sprite: pkgengine.Sprite{Id: int64(i + 1)}}
	}
	game.setupTouchEntries([]Sprite{a, b})
	game.refreshTouchFilters()
	if !slices.Equal(layerWrites, []pkgengine.Object{1, 2}) || !slices.Equal(writes, []int64{2, 1}) {
		t.Fatalf("proxy IDs = %v, masks = %v; want [1 2], [2 1]", layerWrites, writes)
	}
}

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
			previousParser := pkgengine.TilemapparserMgr
			parser := &gameTestTilemapparserMgr{}
			pkgengine.TilemapparserMgr = parser
			t.Cleanup(func() { pkgengine.TilemapparserMgr = previousParser })
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
			game.applyPreparedStage(gamer, project, game.lifecycle.generation(), &plan)
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
			previousScene, previousParser := pkgengine.SceneMgr, pkgengine.TilemapparserMgr
			scene := &tilemapUnloadSceneMgr{}
			parser := &gameTestTilemapparserMgr{}
			pkgengine.SceneMgr, pkgengine.TilemapparserMgr = scene, parser
			t.Cleanup(func() { pkgengine.SceneMgr, pkgengine.TilemapparserMgr = previousScene, previousParser })
			var game Game
			game.tilemapMgr.currentMap = "map1"
			game.tilemapMgr.tilemapDir = "tilemaps/map1"
			game.tilemapMgr.usesNativeParser = tt.newLoader
			game.UnloadTilemap()
			game.UnloadTilemap()
			if game.TilemapName() != "" || game.tilemapMgr.tilemapDir != "" || game.tilemapMgr.usesNativeParser || game.tilemapMgr.hasData() {
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
	previous := pkgengine.TilemapMgr
	manager := &tilemapAPIMgr{}
	pkgengine.TilemapMgr = manager
	t.Cleanup(func() { pkgengine.TilemapMgr = previous })
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
	previousScene, previousRes := pkgengine.SceneMgr, pkgengine.ResMgr
	scene := &decoratorSceneMgr{}
	pkgengine.SceneMgr, pkgengine.ResMgr = scene, &decoratorResMgr{}
	t.Cleanup(func() { pkgengine.SceneMgr, pkgengine.ResMgr = previousScene, previousRes })
	var game Game
	game.tilemapMgr = tilemapManager{g: &game, usesNativeParser: true, tilemapDir: "tilemaps/map", decoratorData: &DecoratorJSON{Decorators: []tm.DecoratorNode{{Path: "tree.png", Position: tm.Vec2{X: 3, Y: 4}, Pivot: tm.Vec2{X: 12, Y: 8}, Scale: tm.Vec2{X: 2, Y: 3}, Ratation: 120, ZIndex: 7, ColliderType: "rect", ColliderPivot: tm.Vec2{X: 1, Y: 2}, ColliderParams: []float64{4, 5}}}}}
	game.tilemapMgr.applyMapContents()
	if scene.path != engine.ToAssetPath("tilemaps/map/tree.png") || scene.position != mathf.NewVec2(3, 4) || scene.pivot != mathf.NewVec2(2, 3) || scene.rotation != 30 || scene.scale != mathf.NewVec2(2, 3) || scene.layer != 7 || scene.collider != collision.ColliderRect || scene.colliderPivot != mathf.NewVec2(13, 10) || !reflect.DeepEqual(scene.params, []float64{4, 5}) {
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
		game.tilemapMgr.mapData = data
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

func TestBubbleEntryPointsCreateAndRenewBubbles(t *testing.T) {
	installAskTestUiMgr(t)
	_, sprite, component := newBubbleTestSprite()

	textToken := sprite.sayOrThink(123, ui.StyleSay)
	if textToken.bubble == nil || component.textObj == nil {
		t.Fatal("Say/Think did not create a text bubble")
	}
	component.expireText(textToken)
	renewedText := component.upsertText("renewed", ui.StyleThink)
	if renewedText.generation != textToken.generation+1 || component.textObj.timedRemovalPending {
		t.Fatal("text bubble renewal did not invalidate its pending removal")
	}
	component.expireText(renewedText)
	content := component.textObj.content
	sameText := component.upsertText("renewed", ui.StyleThink)
	component.expireText(renewedText)
	component.textObj.flushPendingRemoval(clock.Frame() + 1)
	if component.textObj == nil || sameText.bubble != renewedText.bubble || sameText.generation != renewedText.generation+1 || component.textObj.timedRemovalPending || component.textObj.content != content {
		t.Fatal("identical text did not renew the current bubble and cancel older removal")
	}
	component.textObj.generation = ^uint64(0)
	if wrapped := component.upsertText("renewed", ui.StyleThink); wrapped.generation != 1 {
		t.Fatalf("wrapped text generation = %d, want 1", wrapped.generation)
	}
	if empty := sprite.sayOrThink("", ui.StyleSay); empty.bubble != nil || component.textObj != nil {
		t.Fatal("empty Say/Think did not stop the text bubble")
	}
	sprite.stopTextBubble()

	quoteToken := sprite.bubble().upsertQuote("message", "description")
	if quoteToken.bubble == nil || component.quoteObj == nil {
		t.Fatal("Quote did not create a quote bubble")
	}
	component.expireQuote(quoteToken)
	renewedQuote := component.upsertQuote("renewed", "description")
	if renewedQuote.generation != quoteToken.generation+1 || component.quoteObj.timedRemovalPending {
		t.Fatal("quote renewal did not invalidate its pending removal")
	}
	component.expireQuote(renewedQuote)
	sameQuote := component.upsertQuote("renewed", "description")
	component.expireQuote(renewedQuote)
	component.quoteObj.flushPendingRemoval(clock.Frame() + 1)
	if component.quoteObj == nil || sameQuote.bubble != renewedQuote.bubble || sameQuote.generation != renewedQuote.generation+1 || component.quoteObj.timedRemovalPending {
		t.Fatal("identical quote did not renew the current bubble and cancel older removal")
	}
	component.quoteObj.generation = ^uint64(0)
	if wrapped := component.upsertQuote("renewed", "description"); wrapped.generation != 1 {
		t.Fatalf("wrapped quote generation = %d, want 1", wrapped.generation)
	}
	sprite.stopQuoteBubble()
	if component.quoteObj != nil {
		t.Fatal("explicit quote stop did not remove the quote bubble")
	}
	sprite.stopQuoteBubble()

	// Exercise the generated public entry points with untimed calls.
	sprite.Say__0("say")
	sprite.Say__1("say", 0)
	sprite.Think__0("think")
	sprite.Think__1("think", 0)
	sprite.Quote__0("quote")
	sprite.Quote__1("quote", 0)
	sprite.Quote__2("quote", "description")
	sprite.Quote__3("quote", "description", 0)
	sprite.QuoteMsg("", 0)
	component.stopAll()
}

func TestBubbleRemovalUnlocksBeforeDestroy(t *testing.T) {
	for _, kind := range []string{"text", "quote"} {
		for _, removal := range []string{"stop", "expire"} {
			t.Run(kind+"/"+removal, func(t *testing.T) {
				mgr := installAskTestUiMgr(t)
				game, _, component := newBubbleTestSprite()
				var bubble bubbleShape
				var stop func()
				if kind == "text" {
					token := component.upsertText("message", ui.StyleSay)
					bubble, stop = component.textObj, component.stopText
					component.expireText(token)
				} else {
					token := component.upsertQuote("message", "description")
					bubble, stop = component.quoteObj, component.stopQuote
					component.expireQuote(token)
				}
				destroyed := false
				mgr.onDestroy = func() {
					destroyed = true
					if !component.mu.TryLock() {
						t.Fatal("panel destruction ran with the component locked")
					}
					component.mu.Unlock()
					if component.textObj != nil || component.quoteObj != nil {
						t.Fatal("panel destruction ran before the current slot was cleared")
					}
				}
				if removal == "stop" {
					stop()
				} else {
					bubble.flushPendingRemoval(clock.Frame() + 1)
				}
				if !destroyed || game.scene.findShapeIndex(bubble) >= 0 {
					t.Fatal("bubble removal did not destroy its panel and remove its shape")
				}
			})
		}
	}
}

func TestTimedBubbleEntryPointsScheduleRemoval(t *testing.T) {
	installAskTestUiMgr(t)
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	_, sprite, component := newBubbleTestSprite()

	done := false
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
		sprite.Say__1("say", 0.01)
		sprite.Think__1("think", 0.01)
		sprite.Quote__3("quote", "description", 0.01)
		done = true
	})
	currentScheduler.JoinFirstSlice(thread)
	currentScheduler.Update()
	if done {
		t.Fatal("timed bubble calls completed in their issuing frame")
	}
	for range 3 {
		clock.Update(1.0/30, 30)
		currentScheduler.Update()
	}
	if !done {
		t.Fatal("timed bubble calls did not complete after their deadlines")
	}
	if component.textObj == nil || !component.textObj.timedRemovalPending {
		t.Fatal("timed Think did not schedule text bubble removal")
	}
	if component.quoteObj == nil || !component.quoteObj.timedRemovalPending {
		t.Fatal("timed Quote did not schedule quote bubble removal")
	}
	component.stopAll()
}

func TestBubbleObservesCameraChangesAfterDirtyFlagIsCleared(t *testing.T) {
	sprite := &SpriteImpl{}
	sprite.spriteState.isVisible = true
	camera := &cameraImpl{}
	bubble := bubbleBase{sprite: sprite, camera: camera, isDirty: true}

	if !bubble.needsUpdate() {
		t.Fatal("new bubble should need an initial update")
	}
	bubble.markClean()
	if bubble.needsUpdate() {
		t.Fatal("clean bubble unexpectedly needs an update")
	}

	camera.setDirtyFlag(true)
	camera.setDirtyFlag(false)
	if !bubble.needsUpdate() {
		t.Fatal("camera change was lost when its dirty flag was cleared")
	}

	bubble.markClean()
	if bubble.needsUpdate() {
		t.Fatal("bubble still needs an update after observing the camera change")
	}
}

func TestBubbleObservesSpriteChangesAfterProxySync(t *testing.T) {
	installTouchingSyncSpriteMgr(t, newTouchingSyncSpriteMgr())
	sprite := newTouchingTestSprite("sprite", 0, 0, 1)
	camera := &cameraImpl{}
	bubble := bubbleBase{sprite: sprite, camera: camera, isDirty: true}
	bubble.markClean()

	sprite.markProxyDirty()
	buffer := engine.NewSpriteSyncBuffer(1)
	sprite.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 1 {
		t.Fatalf("proxy sync batched %d transforms, want 1", got)
	}
	if !bubble.needsUpdate() {
		t.Fatal("sprite change was lost after its proxy was synchronized")
	}
	bubble.markClean()
	if bubble.needsUpdate() {
		t.Fatal("bubble still needs an update after observing the sprite change")
	}
}

func TestBubbleVisualsAreDeferredUntilFrameEnd(t *testing.T) {
	_, game := setupRuntimeEventGame(t)
	sprite := &SpriteImpl{}
	sprite.spriteState.isVisible = true
	quote := &quoteBubble{
		bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true},
	}
	text := &textBubble{
		bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true},
	}
	component := &bubbleComponent{sprite: sprite, textObj: text}
	sprite.components.bubble = component
	items := []Shape{sprite, quote, text}
	game.scene.items = items
	withEventRegistrationBarrier(game, game.dispatchShapeUpdates)
	if !quote.isDirty || !text.isDirty {
		t.Fatal("update phase unexpectedly committed bubble visuals")
	}

	game.scene.flushBubbleVisuals(items)
	if quote.isDirty || text.isDirty {
		t.Fatal("frame-end phase did not commit bubble visuals")
	}
	if len(game.scene.bubbles) != 2 {
		t.Fatalf("collected %d bubbles, want 2", len(game.scene.bubbles))
	}
}

func TestTimedTextRemovalWaitsForNextFrame(t *testing.T) {
	game, sprite, component := newBubbleTestSprite()
	bubble := &textBubble{
		bubbleBase: bubbleBase{
			sprite:              sprite,
			timedRemovalPending: true,
			timedRemovalFrame:   10,
			generation:          1,
		},
	}
	component.textObj = bubble
	game.scene.add(bubble)

	for _, frame := range []int64{9, 10} {
		bubble.flushPendingRemoval(frame)
		if component.textObj != bubble || game.scene.findShapeIndex(bubble) < 0 {
			t.Fatalf("timed text bubble was removed before its grace frame ended: %d", frame)
		}
	}
	bubble.timedRemovalPending = false
	bubble.flushPendingRemoval(11)
	if component.textObj != bubble {
		t.Fatal("cancelled text expiration removed the bubble")
	}
	bubble.timedRemovalPending = true

	bubble.flushPendingRemoval(11)
	if component.textObj != nil || game.scene.findShapeIndex(bubble) >= 0 {
		t.Fatal("timed bubble was not removed on the following frame")
	}

	// A stale shape must not remove the replacement now occupying its slot.
	replacement := *bubble
	replacement.timedRemovalPending = true
	component.textObj = &replacement
	game.scene.add(&replacement)
	bubble.timedRemovalPending = true
	bubble.flushPendingRemoval(12)
	if component.textObj != &replacement || game.scene.findShapeIndex(&replacement) < 0 {
		t.Fatal("stale text bubble removed its replacement")
	}
	replacement.flushPendingRemoval(12)
	if component.textObj != nil || replacement.timedRemovalPending {
		t.Fatal("current text bubble did not commit its expiration")
	}
}

func TestTextBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	bubble := &textBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{textObj: bubble}

	for _, token := range []bubbleToken{{}, {bubble: &bubble.bubbleBase, generation: 1}} {
		component.expireText(token)
		if bubble.timedRemovalPending {
			t.Fatalf("invalid token %+v scheduled bubble removal", token)
		}
	}

	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
	if !bubble.timedRemovalPending {
		t.Fatal("current token did not schedule timed bubble removal")
	}

	bubble.timedRemovalPending = false
	component.textObj = &textBubble{bubbleBase: bubbleBase{generation: 2}}
	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
	if bubble.timedRemovalPending || component.textObj.timedRemovalPending {
		t.Fatal("token scheduled removal of a different bubble with the same generation")
	}

	component.textObj = nil
	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
}

func TestTimedQuoteRemovalWaitsForNextFrame(t *testing.T) {
	game, sprite, component := newBubbleTestSprite()
	quote := &quoteBubble{
		bubbleBase: bubbleBase{
			sprite:              sprite,
			generation:          1,
			timedRemovalPending: true,
			timedRemovalFrame:   10,
		},
	}
	component.quoteObj = quote
	game.scene.add(quote)

	for _, frame := range []int64{9, 10} {
		quote.flushPendingRemoval(frame)
		if component.quoteObj != quote || game.scene.findShapeIndex(quote) < 0 {
			t.Fatalf("timed quote bubble was removed before its grace frame ended: %d", frame)
		}
	}
	quote.timedRemovalPending = false
	quote.flushPendingRemoval(11)
	if component.quoteObj != quote {
		t.Fatal("cancelled quote expiration removed the bubble")
	}
	quote.timedRemovalPending = true

	quote.flushPendingRemoval(11)
	if component.quoteObj != nil || game.scene.findShapeIndex(quote) >= 0 {
		t.Fatal("timed quote was not removed on the following frame")
	}

	// A stale shape must not remove the replacement now occupying its slot.
	replacement := *quote
	replacement.timedRemovalPending = true
	component.quoteObj = &replacement
	game.scene.add(&replacement)
	quote.timedRemovalPending = true
	quote.flushPendingRemoval(12)
	if component.quoteObj != &replacement || game.scene.findShapeIndex(&replacement) < 0 {
		t.Fatal("stale quote bubble removed its replacement")
	}
	replacement.flushPendingRemoval(12)
	if component.quoteObj != nil || replacement.timedRemovalPending {
		t.Fatal("current quote bubble did not commit its expiration")
	}
}

func TestQuoteBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	quote := &quoteBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{quoteObj: quote}

	for _, token := range []bubbleToken{{}, {bubble: &quote.bubbleBase, generation: 1}} {
		component.expireQuote(token)
		if quote.timedRemovalPending {
			t.Fatalf("invalid token %+v scheduled bubble removal", token)
		}
	}

	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
	if !quote.timedRemovalPending {
		t.Fatal("current token did not schedule timed quote removal")
	}

	quote.timedRemovalPending = false
	component.quoteObj = &quoteBubble{bubbleBase: bubbleBase{generation: 2}}
	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
	if quote.timedRemovalPending || component.quoteObj.timedRemovalPending {
		t.Fatal("token scheduled removal of a different quote with the same generation")
	}

	component.quoteObj = nil
	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
}

func TestLayoutTextBubblesPreservesEmptyActiveSlice(t *testing.T) {
	for _, active := range [][]*textBubble{nil, {}} {
		shapes := bubbleLayout{activeTextBubbles: active, textBubbles: active}
		shapes.collectBubbles(nil)
		shapes.layoutTextBubbles()
		if len(shapes.activeTextBubbles) != 0 || (shapes.activeTextBubbles == nil) != (active == nil) {
			t.Fatalf("empty topology changed active slice: before=%#v after=%#v", active, shapes.activeTextBubbles)
		}
		if len(shapes.textBubbles) != 0 || (shapes.textBubbles == nil) != (active == nil) {
			t.Fatalf("empty topology changed collected slice: before=%#v after=%#v", active, shapes.textBubbles)
		}
	}
}

func TestLayoutTextBubblesTracksPointerIdentity(t *testing.T) {
	setupImmediateEngineCalls(t)
	original := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = original })
	newBubble := func(id uint64) *textBubble {
		sprite := newRenderOffsetTestSprite()
		sprite.spriteState.isVisible = true
		sprite.g.displayState.WindowWidth = 480
		sprite.g.displayState.WindowHeight = 360
		return &textBubble{
			bubbleBase: bubbleBase{sprite: sprite},
			layoutID:   id,
			panel:      &ui.UiSay{},
			content:    ui.NewSayBubbleContent("same message", ui.StyleSay),
		}
	}
	first, second := newBubble(1), newBubble(2)
	shapes := sceneObjects{}
	layout := func(items []Shape) {
		shapes.collectBubbles(items)
		shapes.layoutTextBubbles()
	}
	layout([]Shape{second, first})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{first, second}) || !first.hasLayout || !second.hasLayout {
		t.Fatal("new bubbles were not laid out in stable ID order")
	}
	storage := &shapes.activeTextBubbles[0]
	firstLayout, secondLayout := first.layout, second.layout
	layout([]Shape{first, second})
	if &shapes.activeTextBubbles[0] != storage || first.layout != firstLayout || second.layout != secondLayout {
		t.Fatal("unchanged bubble identities replaced storage or resolved cached layouts")
	}

	replacement := *first
	layout([]Shape{second, &replacement})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement, second}) {
		t.Fatal("equal-valued replacement bubble was not detected by pointer identity")
	}
	second.sprite.spriteState.isVisible = false
	layout([]Shape{&replacement, second})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement}) {
		t.Fatal("hidden bubble remained active")
	}
	if shapes.activeTextBubbles[:2][1] != nil || shapes.textBubbles[:2][1] != nil {
		t.Fatal("removed bubble reference was retained in reusable storage")
	}
	replacement.panel = nil
	layout([]Shape{&replacement, second})
	if len(shapes.activeTextBubbles) != 0 || shapes.activeTextBubbles[:1][0] != nil {
		t.Fatal("panel removal did not clear the final active bubble")
	}
	for _, bubble := range shapes.textBubbles[:cap(shapes.textBubbles)] {
		if bubble != nil {
			t.Fatal("filtered bubble remained in collected storage")
		}
	}

	for _, reason := range []string{"hidden", "panel-less"} {
		for removed, position := range []string{"first", "middle", "last"} {
			t.Run(reason+"/"+position, func(t *testing.T) {
				first, second, third := newBubble(1), newBubble(2), newBubble(3)
				bubbles := []*textBubble{first, second, third}
				if reason == "hidden" {
					bubbles[removed].sprite.spriteState.isVisible = false
				} else {
					bubbles[removed].panel = nil
				}
				var shapes sceneObjects
				shapes.collectBubbles([]Shape{first, second, third})
				storage := shapes.textBubbles[:cap(shapes.textBubbles)]
				shapes.layoutTextBubbles()
				want := slices.Delete(slices.Clone(bubbles), removed, removed+1)
				if !slices.Equal(shapes.textBubbles, want) || &shapes.textBubbles[0] != &storage[0] || storage[2] != nil {
					t.Fatal("filtering lost order, replaced storage, or retained removed bubble")
				}
				shapes.collectBubbles(nil)
				shapes.layoutTextBubbles()
				shapes.init()
				for _, bubble := range storage {
					if bubble != nil {
						t.Fatal("empty frame and reset retained bubble")
					}
				}
			})
		}
	}
}

func TestAskQuestionPlacement(t *testing.T) {
	tests := []struct {
		name          string
		useStage      bool
		spriteVisible bool
		wantPanel     bool
		wantBubble    bool
	}{
		{name: "stage", useStage: true, wantPanel: true},
		{name: "visible sprite", spriteVisible: true, wantBubble: true},
		{name: "hidden sprite", wantPanel: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr := installAskTestUiMgr(t)
			game := &Game{}
			game.scene.init()
			mgr.onAskVisible = func() {
				game.dialogState.AskPanel.OnCheck("answer")
			}

			const question = "How many?"
			if test.useStage {
				game.Ask(question)
			} else {
				sprite := &SpriteImpl{g: game}
				sprite.spriteState.isVisible = test.spriteVisible
				sprite.Ask(question)
			}

			askBody := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody"}]
			if got := mgr.visible[askBody]; got != test.wantPanel {
				t.Errorf("question panel visible = %v, want %v", got, test.wantPanel)
			}
			askLabel := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody/LabelAsk"}]
			if test.wantPanel && mgr.text[askLabel] != question {
				t.Errorf("question panel text = %q, want %q", mgr.text[askLabel], question)
			}
			if got := mgr.created[sayScenePath] != 0; got != test.wantBubble {
				t.Errorf("question bubble created = %v, want %v", got, test.wantBubble)
			}
			if got := game.Answer(); got != "answer" {
				t.Errorf("answer = %q, want %q", got, "answer")
			}
		})
	}
}

func TestBubbleRefreshUsesCurrentTextLayoutAndQuoteBounds(t *testing.T) {
	mgr := &bubbleRenderTestUiMgr{
		askTestUiMgr: installAskTestUiMgr(t),
		positions:    make(map[pkgengine.Object]mathf.Vec2), sizes: make(map[pkgengine.Object]mathf.Vec2),
	}
	pkgengine.UiMgr = mgr
	originalCamera, originalScale := pkgengine.CameraMgr, engine.WindowScale()
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	engine.SetWindowScale(1)
	t.Cleanup(func() { pkgengine.CameraMgr = originalCamera; engine.SetWindowScale(originalScale) })
	sprite := newRenderOffsetTestSprite()
	sprite.g.displayState.WindowWidth, sprite.g.displayState.WindowHeight = 480, 360
	sprite.spriteState.isVisible = true
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true}, panel: ui.NewUiSay(), content: ui.NewSayBubbleContent("first", ui.StyleSay)}
	manager := sceneObjects{}
	sprite.spriteState.isVisible = false
	manager.flushBubbleVisuals([]Shape{bubble})
	if bubble.hasLayout || !bubble.isDirty || slices.Contains(bubbleRenderedTexts(mgr.text), "first") {
		t.Fatal("hidden bubble initialized or rendered its layout")
	}
	sprite.spriteState.isVisible = true
	manager.flushBubbleVisuals([]Shape{bubble})
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "first") {
		t.Fatalf("text bubble was not rendered: dirty=%v text=%v", bubble.isDirty, mgr.text)
	}
	layout := ui.NewSayBubbleLayoutContext(mathf.NewVec2(480, 360)).NewLayout(1, mathf.Vec2{}, mathf.NewVec2(20, 20), ui.NewSayBubbleContent("planned", ui.StyleThink))
	bubble.setLayout(layout)
	bubble.onUpdate()
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "planned") {
		t.Fatal("precomputed layout did not replace the visible text")
	}
	sprite.spriteState.isVisible = false
	bubble.markDirty()
	bubble.onUpdate()
	if !bubble.isDirty {
		t.Fatal("hidden bubble consumed its pending visual update")
	}
	sprite.spriteState.isVisible = true
	panel := bubble.panel
	bubble.panel = nil
	bubble.markDirty()
	manager.flushBubbleVisuals([]Shape{bubble})
	bubble.panel = panel
	quote := &quoteBubble{bubbleBase: bubbleBase{sprite: sprite, camera: bubble.camera, isDirty: true}, panel: ui.NewUiQuote(), message: "quotation", description: "speaker"}
	quote.onUpdate()
	container := mgr.bound[askTestBinding{parent: quote.panel.GetId(), path: "C"}]
	_, size := quote.getBounds()
	if mgr.sizes[container] != size.Addf(20) || !slices.Contains(bubbleRenderedTexts(mgr.text), "quotation") || !slices.Contains(bubbleRenderedTexts(mgr.text), "speaker") || quote.isDirty {
		t.Fatalf("quote layout = size %v, text %v, dirty %v", mgr.sizes[container], mgr.text, quote.isDirty)
	}
}

func TestAskWaitsForAnswerAcrossFrames(t *testing.T) {
	mgr := installAskTestUiMgr(t)
	originalInput := pkgengine.InputMgr
	pkgengine.InputMgr = askDelayedInputMgr{}
	t.Cleanup(func() { pkgengine.InputMgr = originalInput })
	t.Cleanup(clock.ClearTimers)
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	game := &Game{}
	game.scene.init()
	answered := false
	thread := currentScheduler.Create(game, func(coroutine.Thread) { game.Ask("later?"); answered = true })
	currentScheduler.JoinFirstSlice(thread)
	if answered || game.dialogState.AskPanel == nil || !mgr.visible[mgr.askRoot] {
		t.Fatal("Ask did not suspend while displaying its question")
	}
	game.dialogState.AskPanel.OnCheck("later answer")
	clock.Update(1.0/30, 30)
	currentScheduler.Update()
	if !answered || game.Answer() != "later answer" {
		t.Fatalf("Ask resumed=%v, answer=%q", answered, game.Answer())
	}
	currentScheduler.Join(thread)
}

func TestCloneBubbleIsLazyAndIndependent(t *testing.T) {
	game := setupCloneLimitGame(t)
	original := newCloneLimitSprite(game, "original")
	originalBubble := original.bubble()
	originalText := &textBubble{bubbleBase: original.newBubbleBase()}
	originalBubble.textObj = originalText
	game.scene.add(originalText)

	clone := createRuntimeClone(&original.SpriteImpl)
	if clone.components.bubble != nil {
		t.Fatal("clone inherited an allocated bubble component")
	}
	if originalBubble.textObj != originalText {
		t.Fatal("cloning changed the original bubble")
	}

	cloneBubble := clone.bubble()
	if cloneBubble == originalBubble || cloneBubble.sprite != clone {
		t.Fatal("lazy bubble component is not owned by the clone")
	}
	cloneText := &textBubble{bubbleBase: clone.newBubbleBase()}
	cloneBubble.textObj = cloneText
	game.scene.add(cloneText)

	clone.Destroy()
	if cloneBubble.textObj != nil || cloneBubble.quoteObj != nil {
		t.Fatal("destroy left clone bubble state active")
	}
	for _, shape := range game.getAllShapes() {
		if shape == cloneText {
			t.Fatal("destroy left the clone bubble in the shape list")
		}
	}
	if originalBubble.textObj != originalText {
		t.Fatal("destroying the clone changed the original bubble")
	}
}

func TestSpriteDialogueDebugModeKeepsPresentationAndAnswer(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	mgr := installAskTestUiMgr(t)
	sprite := newCloneLimitSprite(game, "speaker")
	sprite.Show()
	sprite.SayWith("hello", 0)
	sprite.ThinkWith("thought", 0)
	sprite.QuoteMsgEx("quote", "source", 0)
	if sprite.bubble().textObj.msg != "thought" || sprite.bubble().quoteObj.message != "quote" {
		t.Fatal("debug logging changed dialogue presentation")
	}
	mgr.onAskVisible = func() { game.dialogState.AskPanel.OnCheck("answer") }
	sprite.Ask("question")
	if game.Answer() != "answer" || sprite.bubble().textObj != nil {
		t.Fatal("debug Ask did not store the answer and retire its bubble")
	}
	sprite.bubble().stopAll()
}

func TestBubbleLayoutIDWrapsWithoutUsingZero(t *testing.T) {
	manager := sceneObjects{bubbleLayout: bubbleLayout{nextTextBubbleLayoutID: ^uint64(0)}}
	bubble := &textBubble{}
	manager.add(bubble)
	if bubble.layoutID != 1 {
		t.Fatalf("wrapped layout ID = %d, want 1", bubble.layoutID)
	}
}

func TestSceneObjectsRelayoutsChangedBubbleInput(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	mgr.zoom = mathf.NewVec2(1, 1)
	previousScale := engine.WindowScale()
	engine.SetWindowScale(1)
	t.Cleanup(func() { engine.SetWindowScale(previousScale) })
	sprite := newRenderOffsetTestSprite()
	sprite.spriteState.isVisible = true
	sprite.g.displayState.WindowWidth, sprite.g.displayState.WindowHeight = 480, 360
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite}, layoutID: 1, panel: &ui.UiSay{}, content: ui.NewSayBubbleContent("hello", ui.StyleSay)}
	manager := bubbleLayout{textBubbles: []*textBubble{bubble}}
	manager.layoutTextBubbles()
	previous := bubble.layout
	sprite.SetXYpos(80, 60)
	manager.layoutTextBubbles()
	if bubble.layout.SameInput(previous) {
		t.Fatal("moving bubble owner did not refresh cached layout input")
	}
}

func TestMonitorEvalPreservesFormatting(t *testing.T) {
	g := monitorEvalFixture{score: 1.23456789, unset: nil}
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"score", "getVar:score"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || eval().Text != "1.23456789" {
			t.Fatalf("field binding %q lost precision", name)
		}
		g.score = 2.3456789
		if eval().Text != "2.3456789" {
			t.Fatal("field binding stopped tracking changes")
		}
		g.score = 1.23456789
	}
	for _, name := range []string{"value", "getVar:value"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || g.calls != 0 {
			t.Fatal("binding must not invoke a getter")
		}
		if eval().Text != "1.23" || g.calls != 1 {
			t.Fatal("auto-property formatting or invocation changed")
		}
		g.calls = 0
	}
	// An explicitly named method is displayed as its function pointer by the
	// existing string evaluator; only the lowercase auto-property is invoked.
	eval := monitorEvalForTest(root, "", "Value", ui.MonitorAppearanceDefault)
	if eval == nil || !regexp.MustCompile(`^0x[0-9a-f]+$`).MatchString(eval().Text) || g.calls != 0 {
		t.Fatal("explicit method behavior changed")
	}
	if eval := monitorEvalForTest(root, "", "unset", ui.MonitorAppearanceDefault); eval == nil || eval().Text != "<nil>" {
		t.Fatal("nil field formatting changed")
	}
	for _, binding := range [][2]string{{"", ""}, {"", "getVar:"}, {"", "missing"}, {"missing", "score"}} {
		if monitorEvalForTest(root, binding[0], binding[1], ui.MonitorAppearanceDefault) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

func TestMonitorRefreshLifecycle(t *testing.T) {
	for name, value := range map[string]ui.MonitorValue{
		"scalar": {Text: "42"},
		"list":   {Items: []string{"one", "two"}},
	} {
		t.Run(name, func(t *testing.T) {
			panel := &monitorPanelSpy{}
			m := &Monitor{
				panel: panel, visible: true, isDirty: true, size: 1,
				eval: func() ui.MonitorValue {
					panel.events = append(panel.events, "eval")
					return value
				},
			}
			refresh := func(delta float64, want ...string) {
				t.Helper()
				panel.events = nil
				m.onUpdate(delta)
				if !slices.Equal(panel.events, want) {
					t.Fatalf("refresh(%v) calls = %v, want %v", delta, panel.events, want)
				}
			}
			visible := []string{"eval", "scale", "position", "render", "show"}
			refresh(0, visible...)
			if !reflect.DeepEqual(panel.value, value) {
				t.Fatal("value was lost during refresh")
			}
			refresh(0.1)
			m.SetXYpos(12, -34)
			m.SetSize(2)
			refresh(0, visible...)
			if panel.pos != mathf.NewVec2(12, -34) || panel.scale != 2 || m.updateTimer != 0.1 {
				t.Fatal("dirty refresh changed the timer or lost widget state")
			}
			refresh(0.1, visible...)
			if m.updateTimer != 0 {
				t.Fatal("periodic refresh did not reset the timer")
			}
			m.Hide()
			refresh(0, "hide")
			refresh(0.1)
			refresh(0.1, "hide")
			m.Show()
			refresh(0, visible...)
			m.Show()
			refresh(0)
			refresh(1, visible...)
			refresh(0)
			m.updateTimer = math.NaN()
			refresh(0)
			m.ChangeXpos(1)
			refresh(0, visible...)
		})
	}
}

func TestMonitorGetterVisibilityTakesEffectNextRefresh(t *testing.T) {
	panel := &monitorPanelSpy{}
	m := &Monitor{panel: panel, visible: true, isDirty: true}
	m.eval = func() ui.MonitorValue {
		m.Hide()
		return ui.MonitorValue{Text: "hidden"}
	}
	m.onUpdate(0)
	if !slices.Equal(panel.events, []string{"scale", "position", "render", "show"}) || m.visible || m.isDirty {
		t.Fatal("getter changed the in-progress refresh's visibility")
	}
	panel.events = nil
	m.onUpdate(monitorUpdateIntervalS)
	if !slices.Equal(panel.events, []string{"hide"}) {
		t.Fatal("getter visibility did not take effect on the next refresh")
	}
}

func TestMonitorAppearance(t *testing.T) {
	tests := []struct {
		name  string
		mode  int
		style any
		want  ui.MonitorAppearance
	}{
		{name: "default", mode: 1, want: ui.MonitorAppearanceDefault},
		{name: "default large", mode: 2, want: ui.MonitorAppearanceDefaultLarge},
		{name: "default unsupported mode", mode: 99, want: ui.MonitorAppearanceDefaultLarge},
		{name: "explicit default", mode: 1, style: "default", want: ui.MonitorAppearanceDefault},
		{name: "scratch", mode: 1, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "scratch large", mode: 2, style: "scratch", want: ui.MonitorAppearanceCompatibleLarge},
		{name: "scratch unsupported mode", mode: 99, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "unknown style", mode: 1, style: "custom", want: ui.MonitorAppearanceDefault},
		{name: "invalid style", mode: 2, style: 1.0, want: ui.MonitorAppearanceDefaultLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shape := coreproject.StageShape{"mode": float64(test.mode)}
			if test.style != nil {
				shape["style"] = test.style
			}
			if got := monitorAppearance(testMonitorShape(t, shape)); got != test.want {
				t.Fatalf("monitorAppearance(%v) = %v, want %v", shape, got, test.want)
			}
		})
	}
}

func TestStageMonitorFixture(t *testing.T) {
	data, err := os.ReadFile("test/StageMonitor/assets/index.json")
	if err != nil {
		t.Fatalf("read StageMonitor fixture: %v", err)
	}
	var project struct {
		ZOrder []coreproject.StageShape `json:"zorder"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("decode StageMonitor fixture: %v", err)
	}

	type monitorFixture struct {
		appearance ui.MonitorAppearance
		target     string
		value      string
		x, y       float64
	}
	want := map[string]monitorFixture{
		"default-implicit": {ui.MonitorAppearanceDefault, "Monkey", "getVar:clicked", -235, 175},
		"default-large":    {ui.MonitorAppearanceDefaultLarge, "", "getVar:downs", -115, 175},
		"scratch-default":  {ui.MonitorAppearanceCompatible, "Monkey", "getVar:clicked", -235, 140},
		"scratch-large":    {ui.MonitorAppearanceCompatibleLarge, "", "getVar:downs", -115, 140},
	}
	for _, shape := range project.ZOrder {
		if shape["type"] != "monitor" {
			continue
		}
		name, _ := shape["name"].(string)
		expected, ok := want[name]
		if !ok {
			t.Errorf("unexpected monitor %q", name)
			continue
		}
		got := monitorFixture{
			appearance: monitorAppearance(testMonitorShape(t, shape)),
			target:     shape["target"].(string),
			value:      shape["val"].(string),
			x:          shape["x"].(float64),
			y:          shape["y"].(float64),
		}
		if got != expected {
			t.Errorf("monitor %q = %+v, want %+v", name, got, expected)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing monitor %q", name)
	}
}

func TestTutorialMonitorFixtures(t *testing.T) {
	fixtures := []struct {
		demo   string
		config coreproject.MonitorShape
	}{
		{"02-Dragon", coreproject.MonitorShape{
			Target: "Dragon", Val: "getVar:score", Name: "monitor-1", Label: "score",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
		{"03-Clone", coreproject.MonitorShape{
			Target: "", Val: "getVar:gid", Name: "monitor-1", Label: "gid",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
	}
	for _, fixture := range fixtures {
		for _, filename := range []string{"index.json", "index_pack.json"} {
			t.Run(fixture.demo+"/"+filename, func(t *testing.T) {
				data, err := os.ReadFile("tutorial/" + fixture.demo + "/assets/" + filename)
				if err != nil {
					t.Fatal(err)
				}
				var project struct {
					ZOrder []any `json:"zorder"`
				}
				if err := json.Unmarshal(data, &project); err != nil {
					t.Fatal(err)
				}
				monitors := 0
				for _, entry := range project.ZOrder {
					shape, ok := entry.(map[string]any)
					if !ok || shape["type"] != "monitor" {
						continue
					}
					monitors++
					got, err := prepareMonitor(shape)
					if err != nil {
						t.Fatal(err)
					}
					if got.config != fixture.config {
						t.Errorf("monitor config = %+v, want %+v", got.config, fixture.config)
					}
					if got.style.Appearance != ui.MonitorAppearanceCompatible {
						t.Errorf("monitor appearance = %v, want Scratch", got.style.Appearance)
					}
					if got.style.Label != fixture.config.Label || got.style.Color != mathf.NewColorRGBAi(0xee, 0x7d, 0x16, 0xff) {
						t.Errorf("monitor label or color changed: %+v", got.style)
					}
				}
				if monitors != 1 {
					t.Errorf("monitor count = %d, want 1", monitors)
				}
			})
		}
	}
}

func TestMonitorSliderConfig(t *testing.T) {
	for _, mode := range []any{"slider", float64(3)} {
		if got := monitorAppearance(testMonitorShape(t, coreproject.StageShape{"mode": mode})); got != ui.MonitorAppearanceSlider {
			t.Fatalf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		config coreproject.StageShape
		want   ui.MonitorSlider
	}{
		{coreproject.StageShape{}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 10.0, "sliderMax": -10.0, "isDiscrete": false}, ui.MonitorSlider{Min: -10, Max: 10, Step: 0.01}},
		{coreproject.StageShape{"sliderMin": math.NaN(), "sliderMax": math.Inf(1)}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 5.0, "sliderMax": 5.0}, ui.MonitorSlider{Min: 5, Max: 5, Step: 1}},
		{coreproject.StageShape{"sliderMin": -math.MaxFloat64, "sliderMax": math.MaxFloat64}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMax": math.MaxFloat64, "isDiscrete": false}, ui.MonitorSlider{Min: 0, Max: 100, Step: 0.01}},
		{coreproject.StageShape{"sliderMin": "--5", "sliderMax": " 10 ", "isDiscrete": "false"}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": "-2.5", "sliderMax": "2.5", "isDiscrete": false}, ui.MonitorSlider{Min: -2.5, Max: 2.5, Step: 0.01}},
	} {
		if got := parseMonitorSlider(tt.config); got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
		}
	}
}

func TestMonitorColorDefaultsAndOverride(t *testing.T) {
	for _, test := range []struct {
		appearance ui.MonitorAppearance
		want       mathf.Color
	}{
		{ui.MonitorAppearanceDefault, mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)},
		{ui.MonitorAppearanceCompatible, mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)},
		{ui.MonitorAppearanceList, mathf.NewColorRGBAi(0xff, 0x66, 0x1a, 0xff)},
	} {
		for _, invalid := range []any{nil, "invalid color"} {
			if got := parseMonitorColor(coreproject.StageShape{"color": invalid}, test.appearance); got != test.want {
				t.Errorf("appearance %v: color = %v, want %v", test.appearance, got, test.want)
			}
		}
		want := mathf.NewColorRGBAi(0x12, 0x34, 0x56, 0xff)
		if got := parseMonitorColor(coreproject.StageShape{"color": "#123456"}, test.appearance); got != want {
			t.Errorf("appearance %v: explicit color = %v, want %v", test.appearance, got, want)
		}
	}
}

func TestMonitorSliderWritesBeforeRefresh(t *testing.T) {
	game := monitorEvalFixture{score: 25}
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil {
		t.Fatal(err)
	}
	panel := &monitorPanelSpy{}
	pending := false
	monitor := Monitor{
		visible: true, panel: panel,
		eval: binding.read,
		updateInput: func() bool {
			if !pending {
				return false
			}
			pending = false
			return binding.write(37)
		},
	}
	pending = true
	monitor.onUpdate(0.001)
	if game.score != 37 || panel.value.Text != "37" {
		t.Fatal("input did not update variable and label in the same frame")
	}
	monitor.visible = false
	pending = true
	monitor.onUpdate(0.001)
	if !pending {
		t.Fatal("hidden slider consumed input")
	}
}

func TestMonitorSliderTargetScope(t *testing.T) {
	game := SliderMonitorGame{score: 1}
	game.Dot.SliderMonitorGame = &game
	game.Dot.score = 2
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "Dot", "getVar:score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(7.5) || game.Dot.score != 7.5 || game.score != 1 {
		t.Fatal("sprite-local slider wrote the wrong scope")
	}
	eval := binding.read
	if eval == nil || eval().Text != "7.5" {
		t.Fatal("slider reader and writer resolve different variables")
	}
	binding, err = bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(9) || game.score != 9 || game.Dot.score != 7.5 {
		t.Fatal("stage slider wrote the wrong scope")
	}
}

func TestParseMonitorModes(t *testing.T) {
	shape := coreproject.StageShape{"target": "", "val": "score", "name": "score", "label": "score", "x": 0.0, "y": 0.0, "visible": true}
	for _, mode := range []any{1.0, 2.0, 3.0, 4.0, "slider", "list"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err != nil {
			t.Errorf("mode %v: %v", mode, err)
		}
	}
	for _, mode := range []any{nil, true, "unknown"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err == nil {
			t.Errorf("accepted invalid mode %v", mode)
		}
	}
}

func TestMonitorBindingCapabilities(t *testing.T) {
	game := monitorEvalFixture{score: 1}
	root := reflect.ValueOf(&game).Elem()
	for _, appearance := range []ui.MonitorAppearance{ui.MonitorAppearanceDefault, ui.MonitorAppearanceCompatibleLarge} {
		binding, err := bindMonitor(root, "", "score", appearance)
		if err != nil || binding.read == nil || binding.write != nil {
			t.Fatal("display-only monitor gained a writer")
		}
	}
	for _, tt := range []struct {
		target, name string
		want         error
	}{
		{"", "value", syscall.EINVAL},
		{"", "Value", syscall.EINVAL},
		{"", "missing", syscall.ENOENT},
		{"missing", "score", syscall.ENOENT},
		{"", "getVar:", syscall.ENOENT},
	} {
		binding, err := bindMonitor(root, tt.target, tt.name, ui.MonitorAppearanceSlider)
		if err != tt.want || binding.read != nil || binding.write != nil {
			t.Errorf("binding %q/%q: error %v, want %v without partial binding", tt.target, tt.name, err, tt.want)
		}
	}
	if game.calls != 0 {
		t.Fatal("binding invoked a reporter")
	}
}

func TestMonitorConstructionAndWidgetTransforms(t *testing.T) {
	manager := setupGameWidgetUi(t)
	game := &monitorEvalFixture{score: 5}
	prepared, err := prepareMonitor(coreproject.StageShape{"name": "score", "val": "score", "label": "Score", "target": "", "mode": "slider", "x": 1.0, "y": 2.0, "visible": true})
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if monitor.GetName() != "score" || monitor.updateInput == nil {
		t.Fatalf("monitor=%+v", monitor)
	}
	monitor.SetXpos(10)
	monitor.SetYpos(20)
	monitor.ChangeYpos(-3)
	monitor.ChangeXYpos(4, 5)
	monitor.SetSize(2)
	monitor.ChangeSize(0.5)
	if monitor.Xpos() != 14 || monitor.Ypos() != 22 || monitor.Size() != 2.5 || !monitor.isDirty {
		t.Fatalf("widget transform=%+v", monitor)
	}
	monitor.onUpdate(0)
	if monitor.updateInput() {
		t.Fatal("unchanged slider reports input")
	}
	manager.rangeValue = 42
	if !monitor.updateInput() || game.score != 42 {
		t.Fatalf("slider input=%g", game.score)
	}
	prepared.style.Appearance = ui.MonitorAppearanceDefault
	plain, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil || plain.updateInput != nil {
		t.Fatalf("plain monitor=%+v, err=%v", plain, err)
	}
}

func TestListMonitorItems(t *testing.T) {
	items := []any{"苹果", 1.23456789, true, nil, NewValue("[b]text[/b]\nnext")}
	list := NewList(items...)
	var retained List
	retained.Init(items...)
	for _, value := range []any{list, &list, retained, &retained, items} {
		want := []string{"苹果", "1.23456789", "true", "", "[b]text[/b]\nnext"}
		if got := listMonitorItems(value); !slices.Equal(got, want) {
			t.Errorf("listMonitorItems(%T) = %q, want %q", value, got, want)
		}
	}
	for _, tt := range []struct {
		value   any
		wantNil bool
	}{
		{nil, true},
		{(*List)(nil), true},
		{[]string(nil), false},
		{NewList(), false},
		{List{}, false},
		{42, true},
	} {
		if got := listMonitorItems(tt.value); len(got) != 0 || (got == nil) != tt.wantNil {
			t.Errorf("listMonitorItems(%T) = %#v, want empty with nil=%v", tt.value, got, tt.wantNil)
		}
	}
	array := [2]int{1, 2}
	for _, value := range []any{array, &array, []int{1, 2}} {
		if got := listMonitorItems(value); !slices.Equal(got, []string{"1", "2"}) {
			t.Errorf("listMonitorItems(%T) = %q", value, got)
		}
	}
}

func TestListMonitorEvalTracksChanges(t *testing.T) {
	g := ListMonitorGame{items: NewList("first")}
	g.Monkey.ListMonitorGame = &g
	g.Monkey.items = NewList("sprite")
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"items", "getVar:items", "current", "Current"} {
		t.Run(name, func(t *testing.T) {
			g.items = NewList("first")
			eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceList)
			if eval == nil {
				t.Fatal("list binding failed")
			}
			before := eval().Items
			g.items.Append("second")
			g.items.Set(0, "changed")
			if got := eval().Items; !slices.Equal(got, []string{"changed", "second"}) {
				t.Fatalf("after append/set: %q", got)
			}
			if !slices.Equal(before, []string{"first"}) {
				t.Fatalf("snapshot changed: %q", before)
			}
			g.items.Delete(0)
			if got := eval().Items; !slices.Equal(got, []string{"second"}) {
				t.Fatalf("after delete: %q", got)
			}
			g.items = NewList("replacement")
			if got := eval().Items; !slices.Equal(got, []string{"replacement"}) {
				t.Fatalf("after replacement: %q", got)
			}
			g.items.Clear()
			if len(eval().Items) != 0 {
				t.Fatal("clear not reflected")
			}
		})
	}
	spriteEval := monitorEvalForTest(root, "Monkey", "getVar:items", ui.MonitorAppearanceList)
	if spriteEval == nil || !slices.Equal(spriteEval().Items, []string{"sprite"}) {
		t.Fatal("sprite-local list binding failed")
	}
	globalEval := monitorEvalForTest(root, "Monkey", "values", ui.MonitorAppearanceList)
	g.values = []string{"shared"}
	if globalEval == nil || !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("promoted global slice binding failed")
	}
	g.values = append(g.values, "grown")
	if !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("slice growth not reflected")
	}
	for _, binding := range [][2]string{{"", "getVar:"}, {"", "missing"}, {"missing", "items"}} {
		if monitorEvalForTest(root, binding[0], binding[1], ui.MonitorAppearanceList) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

func TestListMonitorConfiguration(t *testing.T) {
	for _, mode := range []any{"list", float64(4)} {
		if got := monitorAppearance(testMonitorShape(t, coreproject.StageShape{"mode": mode})); got != ui.MonitorAppearanceList {
			t.Errorf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		width, height any
		want          mathf.Vec2
	}{
		{nil, nil, mathf.NewVec2(100, 200)},
		{0.0, 0.0, mathf.NewVec2(100, 200)},
		{180.0, 240.0, mathf.NewVec2(180, 240)},
		{1.0, 1.0, mathf.NewVec2(100, 60)},
		{math.NaN(), math.Inf(1), mathf.NewVec2(100, 200)},
	} {
		if got := parseListMonitorDimensions(coreproject.StageShape{"width": tt.width, "height": tt.height}); got != tt.want {
			t.Errorf("dimensions(%v, %v) = %v, want %v", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestListMonitorVisibility(t *testing.T) {
	g := &Game{}
	global := &Monitor{target: "", val: "getVar:items"}
	local := &Monitor{target: "Monkey", val: "items"}
	g.scene.add(global)
	g.scene.add(local)
	g.ShowVar("items")
	if !global.Visible() || local.Visible() || !global.isDirty {
		t.Fatal("ShowVar did not select the global list")
	}
	g.HideVar("items")
	if global.Visible() {
		t.Fatal("HideVar did not hide the list")
	}
	if !g.setStageMonitor("Monkey", "getVar:items", true) || !local.Visible() || global.Visible() {
		t.Fatal("sprite list visibility mismatch")
	}
}

func TestSpriteVariableMonitorsPreferLocalThenStage(t *testing.T) {
	game := &Game{}
	sprite := &SpriteImpl{g: game, name: "actor"}
	local := &Monitor{target: "actor", val: "score"}
	global := &Monitor{target: "", val: "score"}
	fallback := &Monitor{target: "", val: "lives"}
	game.scene.items = []Shape{local, global, fallback}
	sprite.ShowVar("score")
	if !local.visible || global.visible {
		t.Fatal("local show changed the shadowed stage monitor")
	}
	sprite.HideVar("score")
	if local.visible {
		t.Fatal("local monitor remained visible")
	}
	sprite.ShowVar("lives")
	if !fallback.visible {
		t.Fatal("stage fallback was not shown")
	}
	sprite.HideVar("lives")
	if fallback.visible {
		t.Fatal("stage fallback remained visible")
	}
}

func (mainThreadTestPlatform) IsMainThread() bool {
	return true
}

func (*targetLookupSprite) Main() {}

func (a *targetPropertyActor) Health() float64 {
	return a.Power
}

func (m *gameNavigationMgr) SetupPathFinderWithSize(grid, cell mathf.Vec2, jump, debug bool) {
	m.grid, m.cell, m.jump, m.debug = grid, cell, jump, debug
	m.setups++
}

func (m *gameNavigationMgr) FindPath(from, to mathf.Vec2, jump bool) pkgengine.Array {
	m.from, m.to, m.jump = from, to, jump
	return []float32{1, 2, 3, 4}
}

func (m *gameBackdropSpriteMgr) SetMaterialParamsVec(_ pkgengine.Object, effect string, x, y, z, w float64) {
	if effect == "repeat_scale" {
		m.repeat = mathf.Vec4{X: x, Y: y, Z: z, W: w}
	}
}

func (m *gameBackdropSpriteMgr) SetPosition(_ pkgengine.Object, pos mathf.Vec2) {
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

func (m *touchFilterSpriteMgr) SetTriggerMask(id pkgengine.Object, mask int64) {
	m.onMask(id, mask)
}

func (m *touchFilterSpriteMgr) SetTriggerLayer(id pkgengine.Object, layer int64) {
	m.onLayer(id, layer)
}

func (m *tilemapUnloadSceneMgr) ClearPureSprites() { m.clearCalls++ }

func (m *tilemapAPIMgr) PlaceTiles(positions pkgengine.Array, path string) {
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

func (m *tilemapAPIMgr) SetTileWithCollisionInfo(path string, points pkgengine.Array) {
	m.path, m.collision = path, points.([]float32)
}

func (m *decoratorSceneMgr) CreateStaticSprite(path string, pos mathf.Vec2, rotation float64, scale mathf.Vec2, layer int64, pivot mathf.Vec2, collider int64, colliderPivot mathf.Vec2, params pkgengine.Array) pkgengine.Object {
	m.path, m.position, m.rotation, m.scale, m.layer, m.pivot, m.collider, m.colliderPivot, m.params = path, pos, rotation, scale, layer, pivot, collider, colliderPivot, params.([]float64)
	return 991
}

func (*decoratorResMgr) GetImageSize(string) mathf.Vec2 { return mathf.NewVec2(20, 10) }

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

func setupImmediateEngineCalls(t testing.TB) {
	t.Helper()
	wrap.Init(func(call func()) { call() })
	t.Cleanup(func() { wrap.Init(engine.WaitMainThread) })
}

func newTargetLookupSprite(g *Game, name string, cloned bool) *targetLookupSprite {
	sprite := &targetLookupSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.spriteState.cloned = cloned
	return sprite
}

func setupGameQueryPhysics(t *testing.T) (*Game, *gameQueryPhysicsMgr, *targetLookupSprite) {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := pkgengine.PhysicsMgr
	manager := &gameQueryPhysicsMgr{}
	pkgengine.PhysicsMgr = manager
	t.Cleanup(func() { pkgengine.PhysicsMgr = previous })
	game := &Game{}
	sprite := newTargetLookupSprite(game, "target", false)
	const id = 89001
	proxy := &engine.Sprite{Sprite: pkgengine.Sprite{Id: id}, Target: &sprite.SpriteImpl}
	sprite.runtimeState.proxy = proxy
	registry := pkgengine.Sprites()
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

func setupCameraTestMgr(t *testing.T) *cameraTestMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := pkgengine.CameraMgr
	mgr := &cameraTestMgr{limits: make(map[int64]int64)}
	pkgengine.CameraMgr = mgr
	t.Cleanup(func() { pkgengine.CameraMgr = previous })
	return mgr
}

func (bubbleLayoutCameraMgr) GetCameraPosition() mathf.Vec2 { return mathf.Vec2{} }

func (bubbleLayoutCameraMgr) GetCameraZoom() mathf.Vec2 { return mathf.NewVec2(1, 1) }

func (bubbleLayoutCameraMgr) GetViewportRect() mathf.Rect2 {
	return mathf.NewRect2(0, 0, 480, 360)
}

func (m *askTestUiMgr) CreateNode(path string) pkgengine.Object {
	id := m.newID()
	m.created[path]++
	if path == askScenePath {
		m.askRoot = id
	}
	return id
}

func (m *askTestUiMgr) BindNode(parent pkgengine.Object, path string) pkgengine.Object {
	id := m.newID()
	m.bound[askTestBinding{parent: parent, path: path}] = id
	return id
}

func (m *askTestUiMgr) DestroyNode(pkgengine.Object) bool {
	if m.onDestroy != nil {
		m.onDestroy()
	}
	return true
}

func (m *askTestUiMgr) SetVisible(id pkgengine.Object, visible bool) {
	m.visible[id] = visible
	if id == m.askRoot && visible && m.onAskVisible != nil {
		onAskVisible := m.onAskVisible
		m.onAskVisible = nil
		onAskVisible()
	}
}

func (m *askTestUiMgr) SetText(id pkgengine.Object, text string) {
	m.text[id] = text
}

func (m *askTestUiMgr) GetText(id pkgengine.Object) string {
	return m.text[id]
}

func (m *askTestUiMgr) newID() pkgengine.Object {
	m.nextID++
	m.ids[m.nextID] = struct{}{}
	return m.nextID
}

func (*bubbleRenderTestUiMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (m *bubbleRenderTestUiMgr) SetPosition(id pkgengine.Object, position mathf.Vec2) {
	m.positions[id] = position
}

func (m *bubbleRenderTestUiMgr) SetGlobalPosition(id pkgengine.Object, position mathf.Vec2) {
	m.positions[id] = position
}

func (m *bubbleRenderTestUiMgr) SetSize(id pkgengine.Object, size mathf.Vec2) {
	m.sizes[id] = size
}

func (askDelayedInputMgr) GetKey(int64) bool { return false }

func newBubbleTestSprite() (*Game, *SpriteImpl, *bubbleComponent) {
	game := &Game{}
	game.scene.init()
	sprite := &SpriteImpl{g: game}
	sprite.spriteState.isVisible = true
	component := &bubbleComponent{sprite: sprite}
	sprite.components.bubble = component
	return game, sprite, component
}

func newAskTestUiMgr(base pkgengine.IUiMgr) *askTestUiMgr {
	return &askTestUiMgr{
		IUiMgr:  base,
		nextID:  100,
		ids:     make(map[pkgengine.Object]struct{}),
		created: make(map[string]int),
		bound:   make(map[askTestBinding]pkgengine.Object),
		visible: make(map[pkgengine.Object]bool),
		text:    make(map[pkgengine.Object]string),
	}
}

func installAskTestUiMgr(t *testing.T) *askTestUiMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	original := pkgengine.UiMgr
	mgr := newAskTestUiMgr(original)
	pkgengine.UiMgr = mgr
	t.Cleanup(func() {
		for id := range mgr.ids {
			pkgengine.DeleteUINode(id)
		}
		pkgengine.UiMgr = original
	})
	return mgr
}

func bubbleRenderedTexts(values map[pkgengine.Object]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func (g *monitorEvalFixture) Value() float64 {
	g.calls++
	return g.score
}

func (p *monitorPanelSpy) Render(value ui.MonitorValue) {
	p.events = append(p.events, "render")
	p.value = value
}

func (p *monitorPanelSpy) SetVisible(visible bool) {
	p.visible = visible
	if visible {
		p.events = append(p.events, "show")
	} else {
		p.events = append(p.events, "hide")
	}
}

func (p *monitorPanelSpy) UpdateScale(scale float64) {
	p.events = append(p.events, "scale")
	p.scale = scale
}

func (p *monitorPanelSpy) UpdatePos(pos mathf.Vec2) {
	p.events = append(p.events, "position")
	p.pos = pos
}

func (*gameWidgetUiMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) GetCanvasScale(pkgengine.Object) mathf.Vec2 { return mathf.NewVec2(1, 1) }

func (*gameWidgetUiMgr) SetGlobalPosition(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) SetColor(pkgengine.Object, mathf.Color) {}

func (*gameWidgetUiMgr) SetSize(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) SetRotation(pkgengine.Object, float64) {}

func (m *gameWidgetUiMgr) SetRange(_ pkgengine.Object, _, _, _, value float64) { m.rangeValue = value }

func (m *gameWidgetUiMgr) GetRangeValue(pkgengine.Object) float64 { return m.rangeValue }

func (g *ListMonitorGame) Current() *List { return &g.items }

func monitorEvalForTest(g reflect.Value, target, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	binding, _ := bindMonitor(g, target, val, appearance)
	return binding.read
}

func testMonitorShape(t *testing.T, fields coreproject.StageShape) coreproject.MonitorShape {
	t.Helper()
	shape := coreproject.StageShape{
		"target": "", "val": "score", "name": "score", "label": "score",
		"mode": 1.0, "x": 0.0, "y": 0.0, "visible": true,
	}
	for key, value := range fields {
		shape[key] = value
	}
	parsed, err := coreproject.ParseMonitorShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func setupGameWidgetUi(t *testing.T) *gameWidgetUiMgr {
	t.Helper()
	base := installAskTestUiMgr(t)
	previousCamera := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = previousCamera })
	manager := &gameWidgetUiMgr{askTestUiMgr: base}
	pkgengine.UiMgr = manager
	return manager
}
