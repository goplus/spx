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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/enginewrap"
	"github.com/goplus/spx/v3/internal/ui"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type stagePlanGame struct {
	Game
	Value    collisionLayerOrderSprite
	Pointer  *collisionLayerOrderSprite
	Dynamic  Sprite
	values   []collisionLayerOrderSprite
	pointers []*collisionLayerOrderSprite
}

func prepareStageEntries(gamer reflect.Value, zorder []any) ([]preparedStageEntry, error) {
	entries := make([]preparedStageEntry, len(zorder))
	for i, raw := range zorder {
		entry, err := prepareStageEntry(gamer, raw)
		if err != nil {
			return nil, err
		}
		entries[i] = entry
	}
	return entries, nil
}

func TestPreparedStageUsesLiveFieldsAndOwnsParsedValues(t *testing.T) {
	for _, shadow := range []bool{false, true} {
		name := "cold"
		if shadow {
			name = "reload shadow"
		}
		t.Run(name, func(t *testing.T) {
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
			source := game
			if shadow {
				source = &stagePlanGame{Dynamic: &collisionLayerOrderSprite{}}
			}
			entries, err := prepareStageEntries(reflect.ValueOf(source).Elem(), raw)
			if err != nil {
				t.Fatal(err)
			}
			for i, value := range raw {
				if shape, ok := value.(coreproject.StageShape); ok {
					clear(shape)
				}
				raw[i] = nil
			}
			inits := game.loadAndInitSprites(reflect.ValueOf(game).Elem(), nil, nil, entries)
			if len(inits) != 8 {
				t.Fatalf("initialized %d sprites, want 8", len(inits))
			}
			if len(game.values) != 2 || len(game.pointers) != 1 {
				t.Fatalf("group sizes = %d, %d", len(game.values), len(game.pointers))
			}
			want := []Sprite{back, &game.Value, game.Pointer, game.Dynamic, &game.values[0], &game.values[1], game.pointers[0], front}
			shapes := game.getAllShapes()
			for i, sprite := range want {
				if inits[i] != sprite || shapes[i] != spriteOf(sprite) {
					t.Fatalf("sprite %d bound to wrong live object", i)
				}
				if spriteOf(sprite).runtimeState.Layer != firstSpriteLayer+i {
					t.Fatalf("sprite %d layer = %d", i, spriteOf(sprite).runtimeState.Layer)
				}
				if i > 0 && i < 7 && spriteOf(sprite).Xpos() != float64(i) {
					t.Fatalf("sprite %d x = %v", i, spriteOf(sprite).Xpos())
				}
			}
		})
	}
}

func TestPreparedDisplayShapesOwnAllConfiguration(t *testing.T) {
	monitor := coreproject.StageShape{
		"type": "monitor", "target": "", "val": "score", "name": "score", "label": "Score", "mode": "slider",
		"x": 2.0, "y": 3.0, "visible": true, "color": "#123456", "width": 120.0, "height": 90.0,
		"sliderMin": 4.0, "sliderMax": 8.0, "isDiscrete": false,
	}
	measure := coreproject.StageShape{"type": "measure", "size": 10.0, "x": 20.0, "y": 30.0, "scale": 2.0, "heading": 45.0}
	entries, err := prepareStageEntries(reflect.Value{}, []any{monitor, measure})
	if err != nil {
		t.Fatal(err)
	}
	clear(monitor)
	clear(measure)
	got := entries[0].monitor
	if got.config.Name != "score" || got.config.X != 2 || got.config.Y != 3 || !got.config.Visible {
		t.Fatalf("prepared monitor config = %+v", got.config)
	}
	color, _ := mathf.NewColorAny("#123456")
	if got.style.Appearance != ui.MonitorAppearanceSlider || got.style.Color != color || got.style.Dimensions != mathf.NewVec2(120, 90) || got.style.Slider != (ui.MonitorSlider{Min: 4, Max: 8, Step: 0.01}) {
		t.Fatalf("prepared monitor style = %+v", got.style)
	}
	m := entries[1].measure
	if m.Size != 10 || m.X != 20 || m.Y != 30 || m.Scale != 2 || m.Heading != 45 {
		t.Fatalf("prepared measure = %+v", m)
	}
}

func TestPreparedStagePreservesDirectFieldLookup(t *testing.T) {
	type embedded struct{ Target *collisionLayerOrderSprite }
	type holder struct{ embedded }
	game := holder{embedded{Target: &collisionLayerOrderSprite{}}}
	_, err := prepareStageEntries(reflect.ValueOf(&game).Elem(), []any{coreproject.StageShape{"type": "sprite", "target": "Target"}})
	if err == nil {
		t.Fatal("promoted field unexpectedly accepted as direct stage target")
	}
}

func TestColdLoadKeepsUnregisteredSpriteFieldsUnloaded(t *testing.T) {
	game := &reloadDirectCommitGame{}
	files := reloadConfigFS{}
	setupReloadCommitRuntime(t, files, game, nil, false)
	loadGameSprites(&game.Game, reflect.ValueOf(game).Elem(), files, &coreproject.ProjectConfig{})
	if game.DirectCommitSprite == nil {
		t.Fatal("cold load did not allocate the sprite field")
	}
	if len(game.sprs) != 0 {
		t.Fatalf("cold load initialized unregistered sprites: %v", game.sprs)
	}
}

func TestReloadStageDoesNotReadProjectZOrderDuringCommit(t *testing.T) {
	game := setupReloadCommitGame(t, reloadConfigFS{"sprites/reloadCommitSprite/index.json": preparedPropertiesSpriteConfig})
	gamer := reflect.ValueOf(game).Elem()
	plan, err := prepareReload(&game.Game, gamer, strings.NewReader(`{"zorder":[{"type":"sprites","target":"Sprites","items":[{"x":7},{"x":9}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	clear(plan.project.Zorder[0].(coreproject.StageShape))
	plan.project.Zorder = []any{false}
	inits := game.loadAndInitSprites(gamer, &plan.project, plan.spriteLoader(&game.Game), plan.stage)
	if len(inits) != 2 || len(game.Sprites) != 2 || game.Sprites[0].Xpos() != 7 || game.Sprites[1].Xpos() != 9 {
		t.Fatalf("reload did not commit prepared stage entries: %v", game.Sprites)
	}
}

func TestColdStageKeepsLazyLoadFailureBeforeLaterParseFailure(t *testing.T) {
	var game Game
	game.initShapeMgr()
	game.sprs = make(map[string]Sprite)
	game.typs = map[string]reflect.Type{"Sprite": reflect.TypeFor[collisionLayerOrderSprite]()}
	want := errors.New("first sprite config failed")
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("cold load failure = %v, want first sprite config error", got)
		}
	}()
	game.loadAndInitSprites(reflect.Value{}, &coreproject.ProjectConfig{Zorder: []any{
		"Sprite", coreproject.StageShape{"type": "measure", "size": "invalid"},
	}}, func(Sprite, string, reflect.Value) error { return want }, nil)
	t.Fatal("cold stage load unexpectedly succeeded")
}

func TestPrepareStageEntryKinds(t *testing.T) {
	game := &stagePlanGame{Dynamic: &collisionLayerOrderSprite{}}
	monitorShape := func(kind string) coreproject.StageShape {
		return coreproject.StageShape{
			"type": kind, "target": "", "val": "score", "name": "score", "label": "Score",
			"mode": 1.0, "x": 2.0, "y": 3.0, "visible": true,
		}
	}
	tests := []struct {
		name string
		raw  any
		kind stageEntryKind
	}{
		{"named sprite", "Sprite", stageNamedSprite},
		{"empty sprite name", "", stageNamedSprite},
		{"monitor", monitorShape("monitor"), stageMonitor},
		{"stage monitor alias", monitorShape("stageMonitor"), stageMonitor},
		{"measure", coreproject.StageShape{"type": "measure", "size": 10.0, "x": 20.0, "y": 30.0}, stageMeasure},
		{"value sprite", coreproject.StageShape{"type": "sprite", "target": "Value", "x": 4.0}, stageSprite},
		{"pointer sprite", coreproject.StageShape{"type": "sprite", "target": "Pointer"}, stageSprite},
		{"interface sprite", coreproject.StageShape{"type": "sprite", "target": "Dynamic"}, stageSprite},
		{"value sprites", coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{coreproject.StageShape{"x": 4.0}}}, stageSprites},
		{"pointer sprites", coreproject.StageShape{"type": "sprites", "target": "pointers", "items": []any{coreproject.StageShape{}}}, stageSprites},
		{"empty sprites", coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{}}, stageSprites},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, err := prepareStageEntry(reflect.ValueOf(game).Elem(), test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if entry.kind != test.kind {
				t.Fatalf("kind = %v, want %v", entry.kind, test.kind)
			}
			switch entry.kind {
			case stageNamedSprite:
				if entry.name != test.raw.(string) {
					t.Fatalf("name = %q, want %q", entry.name, test.raw)
				}
			case stageMonitor:
				if entry.monitor.config.Name != "score" || entry.monitor.config.X != 2 || entry.monitor.config.Y != 3 {
					t.Fatalf("monitor = %+v", entry.monitor)
				}
			case stageMeasure:
				if entry.measure.Size != 10 || entry.measure.X != 20 || entry.measure.Y != 30 || entry.measure.Scale != 1 {
					t.Fatalf("measure = %+v", entry.measure)
				}
			case stageSprite, stageSprites:
				shape := test.raw.(coreproject.StageShape)
				if entry.name != shape["target"] || reflect.TypeOf(game).Elem().Field(entry.fieldIndex).Name != entry.name {
					t.Fatalf("sprite target = %q, field index = %d", entry.name, entry.fieldIndex)
				}
				count := 1
				if entry.kind == stageSprites {
					count = len(shape["items"].([]any))
					if entry.spriteType != reflect.TypeFor[collisionLayerOrderSprite]() {
						t.Fatalf("sprite type = %v", entry.spriteType)
					}
				}
				if len(entry.properties) != count {
					t.Fatalf("properties count = %d, want %d", len(entry.properties), count)
				}
			}
		})
	}
}

func TestPrepareStageEntryErrors(t *testing.T) {
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
		{"missing target", coreproject.StageShape{"type": "sprite"}, "stage shape target must be a non-empty string"},
		{"empty target", coreproject.StageShape{"type": "sprite", "target": ""}, "stage shape target must be a non-empty string"},
		{"non-string target", coreproject.StageShape{"type": "sprites", "target": 1.0}, "stage shape target must be a non-empty string"},
		{"unknown sprite", coreproject.StageShape{"type": "sprite", "target": "Missing"}, `stage sprite target "Missing" is not defined`},
		{"non-sprite field", coreproject.StageShape{"type": "sprite", "target": "Number"}, `stage sprite target "Number" is not a sprite field`},
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
			_, err := prepareStageEntry(reflect.ValueOf(game).Elem(), test.raw)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

type stageEntryPanicExtMgr struct{ pkgengine.IExtMgr }

func (*stageEntryPanicExtMgr) OnRuntimePanic(message string) { panic(message) }

func TestColdStageWrapsEntryFailureWithLayer(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	originalExtMgr, originalPlatformMgr := pkgengine.ExtMgr, pkgengine.PlatformMgr
	pkgengine.ExtMgr, pkgengine.PlatformMgr = &stageEntryPanicExtMgr{}, &reloadCommitPlatformMgr{}
	t.Cleanup(func() {
		pkgengine.ExtMgr, pkgengine.PlatformMgr = originalExtMgr, originalPlatformMgr
	})
	game := &stagePlanGame{}
	game.initShapeMgr()
	defer func() {
		got := recover()
		if got != "zorder[1]: invalid stage shape type" {
			t.Fatalf("cold load failure = %v, want indexed stage shape error", got)
		}
	}()
	game.loadAndInitSprites(reflect.ValueOf(game).Elem(), &coreproject.ProjectConfig{Zorder: []any{
		coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{}}, coreproject.StageShape{},
	}}, nil, nil)
	t.Fatal("cold stage load unexpectedly succeeded")
}
