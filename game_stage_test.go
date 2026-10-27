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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	"github.com/goplus/spx/v4/internal/ui"
	spxapi "github.com/goplus/spx/v4/pkg/spx"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

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
		if got := game.GetTargetProperty(target, "Missing"); got.data != nil {
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
		if spriteOf(sprite).runtimeState.Layer != firstSpriteLayer+i {
			t.Fatalf("sprite %d layer = %d", i, spriteOf(sprite).runtimeState.Layer)
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
	if _, ok := plain.resolveTargetProperty("", ""); ok {
		t.Fatal("empty property resolved")
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
	t.Cleanup(func() { enginewrap.Init(engine.WaitMainThread) })
	game := setupStageLoadTestGame(t, gameTestFS{})
	game.baseObj.initWithSize(480, 360)
	manager := &gameBackdropSpriteMgr{ISpriteMgr: pkgengine.SpriteMgr}
	pkgengine.SpriteMgr = manager
	game.displayState.WorldWidth, game.displayState.WorldHeight = 960, 720
	game.costumes = []*costume{newCostumeWithSize(480, 360), newCostumeWithSize(480, 360)}
	game.costumes[0].name = "first"
	game.costumes[1].name = "second"
	game.runtimeState.SyncSprite = &engine.Sprite{Sprite: pkgengine.Sprite{Id: 89801}}
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
	itime.Start(nil)
	t.Cleanup(func() { itime.Start(nil) })
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
	co := setupRuntimeScheduler(t)
	itime.Start(nil)
	t.Cleanup(func() { itime.Start(nil) })
	var game Game
	count := 0
	finished := false
	thread := co.Create(nil, func(coroutine.Thread) {
		game.WaitNextFrame()
		RepeatUntil(func() bool { return count == 3 }, func() { count++ })
		finished = true
	})
	co.JoinFirstSlice(thread)
	if finished {
		t.Fatal("WaitNextFrame completed before a frame update")
	}
	for range 5 {
		itime.Update(1.0/30, 30)
		co.Update()
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
		sprite.spriteState.IsVisible = true
		sprite.spriteState.Cloned = true
		sprite.runtimeState.Scale = 2
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
	if !unchanged.spriteState.IsVisible || unchanged.runtimeState.Scale != 2 || unchanged.costumeIndex != 3 {
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
	if reset.spriteState.IsVisible || reset.runtimeState.Scale != 0 || reset.costumeIndex != 0 {
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

func newTargetLookupSprite(g *Game, name string, cloned bool) *targetLookupSprite {
	sprite := &targetLookupSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.spriteState.Cloned = cloned
	return sprite
}
