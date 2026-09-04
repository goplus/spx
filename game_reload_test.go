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
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

type reloadConfigFS map[string]string

func (f reloadConfigFS) Open(name string) (io.ReadCloser, error) {
	content, ok := f[name]
	if !ok {
		return nil, errors.New("file not found: " + name)
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (reloadConfigFS) Close() error { return nil }

type reloadPreflightGame struct {
	Game
	Sprite *reloadPreflightSprite
}

type reloadPreflightSprite struct {
	SpriteImpl
	*reloadPreflightGame
}

func (*reloadPreflightSprite) Main() {}

func setupReloadPreflightGame(t *testing.T, files reloadConfigFS) (*reloadPreflightGame, *reloadPreflightSprite, *coroutine.Coroutines) {
	t.Helper()

	originalScheduler := gco
	originalGame := engine.GetGame()
	co := coroutine.New(nil)
	co.OnInited()
	gco = co
	engine.SetCoroutines(co)

	game := &reloadPreflightGame{}
	sprite := &reloadPreflightSprite{reloadPreflightGame: game}
	game.Sprite = sprite
	game.Game.gamer = game
	game.Game.fs = files
	game.Game.events = make(chan event, eventBufferSize)
	game.Game.sprs = map[string]Sprite{"Sprite": sprite}
	game.Game.typs = map[string]reflect.Type{
		"reloadPreflightSprite": reflect.TypeFor[reloadPreflightSprite](),
	}
	sprite.name = "live-sentinel"
	engine.SetGame(&game.Game)

	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("reload preflight test coroutines did not stop")
		}
		gco = originalScheduler
		engine.SetCoroutines(originalScheduler)
		engine.SetGame(originalGame)
	})
	return game, sprite, co
}

func startReloadPreflightSentinelThread(t *testing.T, co *coroutine.Coroutines, owner any) (coroutine.Thread, func()) {
	t.Helper()

	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})
	thread := co.Create(owner, func(coroutine.Thread) int {
		close(started)
		<-release
		close(completed)
		return 0
	})
	var once sync.Once
	finish := func() {
		once.Do(func() { close(release) })
		co.Join(thread)
		select {
		case <-completed:
		default:
			t.Error("sentinel coroutine did not complete after release")
		}
	}
	t.Cleanup(finish)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("sentinel coroutine did not start")
	}
	return thread, finish
}

func assertReloadPreflightPreservedLiveState(
	t *testing.T,
	game *reloadPreflightGame,
	sprite *reloadPreflightSprite,
	thread coroutine.Thread,
	events chan event,
) {
	t.Helper()

	if thread.Stopped() {
		t.Fatal("reload preflight stopped a live coroutine")
	}
	if game.Game.events != events {
		t.Fatal("reload preflight replaced the live event channel")
	}
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("reload preflight closed the live event channel")
		}
		t.Fatal("live event channel unexpectedly contained an event")
	default:
	}
	marker := new(struct{})
	events <- marker
	if got := <-events; got != marker {
		t.Fatalf("live event channel returned %T, want sentinel marker", got)
	}
	if game.Game.sprs["Sprite"] != sprite {
		t.Fatal("reload preflight cleared or replaced the live sprite map")
	}
	if game.Sprite != sprite || sprite.name != "live-sentinel" || sprite.reloadPreflightGame != game {
		t.Fatal("reload preflight mutated the live sprite field")
	}
}

func TestPrepareReloadSuccessDoesNotMutateLiveGame(t *testing.T) {
	files := reloadConfigFS{
		"sprites/Sprite/index.json": `{
			"costumeSet":{"path":"sprite.png","nx":2},
			"fAnimations":{"walk":{"frameFrom":"0","frameTo":"1"}}
		}`,
	}
	game, sprite, co := setupReloadPreflightGame(t, files)
	events := game.Game.events
	thread, finishThread := startReloadPreflightSentinelThread(t, co, game)

	plan, err := prepareReload(&game.Game, reflect.ValueOf(game).Elem(), strings.NewReader(`{"zorder":["Sprite"]}`))
	if err != nil {
		t.Fatalf("prepareReload error = %v", err)
	}
	loaded, ok := plan.spriteConfigs["Sprite"]
	if !ok {
		t.Fatal("reload plan does not contain Sprite config")
	}
	if got, want := loaded.Config.CostumeSet.Path, "sprites/Sprite/sprite.png"; got != want {
		t.Fatalf("normalized sprite path = %q, want %q", got, want)
	}
	assertReloadPreflightPreservedLiveState(t, game, sprite, thread, events)
	finishThread()
}

func TestReloadPreflightFailurePreservesLiveGame(t *testing.T) {
	validSprite := `{"costumeSet":{"path":"sprite.png","nx":1}}`
	tests := []struct {
		name      string
		project   string
		files     reloadConfigFS
		wantError string
	}{
		{
			name:      "malformed project JSON",
			project:   `{`,
			files:     reloadConfigFS{},
			wantError: "load project config",
		},
		{
			name:      "malformed sprite JSON",
			project:   `{"zorder":["Sprite"]}`,
			files:     reloadConfigFS{"sprites/Sprite/index.json": `{`},
			wantError: `load sprite config "Sprite"`,
		},
		{
			name:      "semantically invalid sprite",
			project:   `{"zorder":["Sprite"]}`,
			files:     reloadConfigFS{"sprites/Sprite/index.json": `{}`},
			wantError: `sprite config "Sprite": configuration must define`,
		},
		{
			name:      "invalid z-order sprite",
			project:   `{"zorder":["Missing"]}`,
			files:     reloadConfigFS{},
			wantError: `zorder[0]: sprite "Missing" is not defined`,
		},
		{
			name:      "invalid z-order shape",
			project:   `{"zorder":[{"type":"measure","size":"large","x":0,"y":0}]}`,
			files:     reloadConfigFS{},
			wantError: `stage shape field "size" has type string`,
		},
		{
			name:      "invalid project system settings",
			project:   `{"physics":true,"zorder":[]}`,
			files:     reloadConfigFS{},
			wantError: "autoSetCollisionLayer and physics",
		},
		{
			name:      "null backdrop",
			project:   `{"backdrops":[null],"zorder":[]}`,
			files:     reloadConfigFS{},
			wantError: "backdrops[0] is null",
		},
		{
			name:      "null animation",
			project:   `{"zorder":["Sprite"]}`,
			files:     reloadConfigFS{"sprites/Sprite/index.json": `{"costumeSet":{"nx":1},"fAnimations":{"walk":null}}`},
			wantError: `fAnimations["walk"] is null`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := test.files
			if _, ok := files["sprites/Sprite/index.json"]; !ok && strings.Contains(test.project, `"Sprite"`) {
				files["sprites/Sprite/index.json"] = validSprite
			}
			game, sprite, co := setupReloadPreflightGame(t, files)
			events := game.Game.events
			thread, finishThread := startReloadPreflightSentinelThread(t, co, game)

			err := XGot_Game_Reload(game, strings.NewReader(test.project))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("XGot_Game_Reload error = %v, want substring %q", err, test.wantError)
			}
			assertReloadPreflightPreservedLiveState(t, game, sprite, thread, events)
			finishThread()
		})
	}
}

func TestValidateReloadSpriteConfigLayout(t *testing.T) {
	tests := []struct {
		name      string
		config    coreproject.SpriteConfig
		wantError string
	}{
		{
			name:      "empty costumes",
			config:    coreproject.SpriteConfig{Costumes: []*coreproject.CostumeConfig{}},
			wantError: "costumes must not be empty",
		},
		{
			name:      "null costume",
			config:    coreproject.SpriteConfig{Costumes: []*coreproject.CostumeConfig{nil}},
			wantError: "costumes[0] is null",
		},
		{
			name:      "invalid costume set frame count",
			config:    coreproject.SpriteConfig{CostumeSet: &coreproject.CostumeSet{}},
			wantError: "invalid frame count 0",
		},
		{
			name: "incomplete costume set items",
			config: coreproject.SpriteConfig{CostumeSet: &coreproject.CostumeSet{
				Nx:    2,
				Items: []coreproject.CostumeSetItem{{NamePrefix: "idle", N: 1}},
			}},
			wantError: "incomplete frame loading",
		},
		{
			name:      "empty multipart costume set",
			config:    coreproject.SpriteConfig{CostumeMPSet: &coreproject.CostumeMPSet{}},
			wantError: "costumeMPSet.parts must not be empty",
		},
		{
			name: "animation references missing costume",
			config: coreproject.SpriteConfig{
				CostumeSet: &coreproject.CostumeSet{Nx: 1},
				FAnimations: map[string]*coreproject.AniConfig{
					"walk": {FrameFrom: "missing", FrameTo: "0"},
				},
			},
			wantError: `frameFrom references missing costume "missing"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateReloadSpriteConfig(&test.config)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("validateReloadSpriteConfig error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestValidateReloadSpriteConfigAcceptsSupportedLayouts(t *testing.T) {
	tests := []coreproject.SpriteConfig{
		{Costumes: []*coreproject.CostumeConfig{{Name: "idle"}}},
		{CostumeSet: &coreproject.CostumeSet{Nx: 2}},
		{CostumeMPSet: &coreproject.CostumeMPSet{Parts: []coreproject.CostumeSetPart{{Nx: 1}, {Nx: 2}}}},
		{
			CostumeSet: &coreproject.CostumeSet{Nx: 2, Items: []coreproject.CostumeSetItem{{NamePrefix: "walk", N: 2}}},
			FAnimations: map[string]*coreproject.AniConfig{
				"walk": {FrameFrom: "walk0", FrameTo: "walk1"},
			},
		},
	}

	for i := range tests {
		if err := validateReloadSpriteConfig(&tests[i]); err != nil {
			t.Errorf("validateReloadSpriteConfig(config %d) error = %v", i, err)
		}
	}
}
