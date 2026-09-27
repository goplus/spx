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
	"github.com/goplus/spx/v3/internal/ui"
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
