//go:build !js

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
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	spxfs "github.com/goplus/spx/v4/fs"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/core/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type loadGameFS struct {
	gameTestFS
	closeCalls int
	closeErr   error
}

type loadGameResMgr struct {
	gameTestResMgr
	fontError  string
	applyCalls int
}

type loadGamePlatformMgr struct {
	gameTestPlatformMgr
	title string
}

type loadGameAudioMgr struct {
	pkgengine.IAudioMgr
}

type runtimePanicRecorder struct {
	pkgengine.IExtMgr
	messages []string
	exits    []int64
}

type loadGameAssetResMgr struct {
	loadGameResMgr
	filesystemMode bool
}

type preflightLoadGame struct {
	Game
	First, Second *preflightLoadSprite
}

type preflightLoadSprite struct {
	SpriteImpl
	*preflightLoadGame
}

type singleReadProjectFS struct {
	loadGameFS
	reads map[string]int
}

func TestLoadGameLoadsStageBeforeStartingScripts(t *testing.T) {
	files := gameTestFS{
		"index.json":                          `{"run":{"title":"loaded game"},"zorder":["DirectLoadSprite"]}`,
		"sprites/DirectLoadSprite/index.json": `{"costumes":[{"name":"default","path":"sprite.png","imageWidth":10,"imageHeight":10}]}`,
	}
	game := &directSpriteTestGame{}
	base := setupGameTestRuntime(t, files, game, []Sprite{&DirectLoadSprite{}}, false)
	platform := &loadGamePlatformMgr{}
	pkgengine.PlatformMgr = platform
	pkgengine.ResMgr = &loadGameResMgr{}
	previousAudio := pkgengine.AudioMgr
	pkgengine.AudioMgr = &loadGameAudioMgr{}
	t.Cleanup(func() {
		pkgengine.AudioMgr = previousAudio
	})
	setupLoadGameFlags(t)
	base.lifecycleState.IsRunned.Store(false)
	resource := &loadGameFS{gameTestFS: files}

	if err := base.loadGame(resource, base.bootstrapGeneration()); err != nil {
		t.Fatal(err)
	}
	if resource.closeCalls != 0 || base.fs != resource {
		t.Fatalf("resource ownership not transferred: close calls = %d, game fs = %T", resource.closeCalls, base.fs)
	}
	if game.DirectLoadSprite == nil || game.DirectLoadSprite.g != base {
		t.Fatal("stage sprite was not bound to its game")
	}
	if platform.title != "loaded game" {
		t.Fatalf("window title = %q", platform.title)
	}
	if len(base.pendingBootstrap) == 0 || base.lifecycleState.BootstrapDone.Load() {
		t.Fatal("load must queue bootstrap hooks without running them")
	}
	if got := scheduler.LastThreadID(); got != 1 {
		t.Fatalf("game loops = %d, want 1", got)
	}
}

func TestLoadGameAppliesPhysicsBeforeLoadingSprites(t *testing.T) {
	for _, physics := range []bool{false, true} {
		for _, test := range []struct {
			name       string
			game       Gamer
			sprite     Sprite
			spriteName string
		}{
			{"direct", &directSpriteTestGame{}, &DirectLoadSprite{}, "DirectLoadSprite"},
			{"lazy", &stageLoadTestGame{}, &stageLoadTestSprite{}, "stageLoadTestSprite"},
		} {
			t.Run(fmt.Sprintf("%s/physics=%v", test.name, physics), func(t *testing.T) {
				files := gameTestFS{
					"index.json": fmt.Sprintf(`{"physics":%v,"autoSetCollisionLayer":%v,"zorder":[%q]}`,
						physics, !physics, test.spriteName),
					"sprites/" + test.spriteName + "/index.json": `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}],"size":1,"visible":true}`,
				}
				base := setupGameTestRuntime(t, files, test.game, []Sprite{test.sprite}, !physics)
				pkgengine.PlatformMgr = &loadGamePlatformMgr{}
				pkgengine.ResMgr = &loadGameResMgr{}
				previousAudio := pkgengine.AudioMgr
				pkgengine.AudioMgr = &loadGameAudioMgr{}
				t.Cleanup(func() { pkgengine.AudioMgr = previousAudio })
				setupLoadGameFlags(t)

				if err := base.loadGame(&loadGameFS{gameTestFS: files}, base.bootstrapGeneration()); err != nil {
					t.Fatal(err)
				}
				sprite := spriteOf(base.sprs[test.spriteName])
				if sprite == nil || sprite.g != base {
					t.Fatal("loaded sprite is not bound to its game")
				}
				wantCollider := collision.ColliderNone
				if physics {
					wantCollider = collision.ColliderAuto
				}
				if got := sprite.physics().collisionInfo.Type; got != wantCollider {
					t.Fatalf("default collider = %d, want %d", got, wantCollider)
				}
			})
		}
	}
}

func TestLoadGameFontFailureDoesNotStartScripts(t *testing.T) {
	files := gameTestFS{"index.json": `{}`}
	game := setupStageLoadTestGame(t, files)
	pkgengine.ResMgr = &loadGameResMgr{fontError: "font unavailable"}
	setupLoadGameFlags(t)
	resource := &loadGameFS{gameTestFS: files}
	pending := &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	game.inputMgr.frame.pending = pending
	if err := game.loadGame(resource, game.bootstrapGeneration()); err == nil || !strings.Contains(err.Error(), "font unavailable") {
		t.Fatalf("loadGame error = %v", err)
	}
	if resource.closeCalls != 1 {
		t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
	}
	if game.inputMgr.frame.pending != pending || scheduler.LastThreadID() != 0 {
		t.Fatal("font failure changed pending input or the game loops")
	}
}

func TestLoadGameValidatesProjectSettings(t *testing.T) {
	files := gameTestFS{"index.json": `{"physics":true}`}
	setupLoadGameFlags(t)
	err := new(Game).loadGame(&loadGameFS{gameTestFS: files}, 0)
	const want = "project config: autoSetCollisionLayer and physics must have different enabled states"
	if err == nil || err.Error() != want {
		t.Fatalf("loadGame error = %v, want %q", err, want)
	}
}

func TestLoadGameProjectValidationClosesResources(t *testing.T) {
	for _, test := range []struct{ name, project, want string }{
		{"physics settings", `{"physics":true}`, "autoSetCollisionLayer and physics"},
		{"null backdrop", `{"backdrops":[{"name":"valid"},null]}`, "backdrops[1] is null"},
	} {
		t.Run(test.name, func(t *testing.T) {
			closeErr := errors.New("close failed")
			setupLoadGameFlags(t, "-f")
			previousResMgr := pkgengine.ResMgr
			resMgr := &loadGameResMgr{}
			pkgengine.ResMgr = resMgr
			t.Cleanup(func() { pkgengine.ResMgr = previousResMgr })
			resource := &loadGameFS{
				gameTestFS: gameTestFS{"index.json": test.project},
				closeErr:   closeErr,
			}

			err := new(Game).loadGame(resource, 0)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadGame error = %v, want %q", err, test.want)
			}
			if !errors.Is(err, closeErr) {
				t.Fatalf("loadGame error = %v, want joined close error", err)
			}
			if resource.closeCalls != 1 {
				t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
			}
			if fullscreen := flag.CommandLine.Lookup("f"); fullscreen == nil || fullscreen.Value.String() != "true" {
				t.Fatal("command-line flags were not parsed before project validation")
			}
			if resMgr.applyCalls != 0 {
				t.Fatalf("project fonts applied %d times before project validation", resMgr.applyCalls)
			}
		})
	}
}

func TestLoadGameSpriteLoadPanicClosesResources(t *testing.T) {
	files := gameTestFS{"index.json": `{}`}
	game := &stagePlanGame{}
	base := setupGameTestRuntime(t, files, game, nil, false)
	base.typs["Dynamic"] = reflect.TypeFor[int]()
	setupLoadGameFlags(t)
	resource := &loadGameFS{gameTestFS: files}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("loadGame did not panic")
		}
		if resource.closeCalls != 1 {
			t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
		}
	}()
	_ = base.loadGame(resource, base.bootstrapGeneration())
}

func TestLoadGameSpriteFailureClosesResources(t *testing.T) {
	closeErr := errors.New("close failed")
	for _, tt := range []struct {
		name     string
		closeErr error
	}{
		{"close succeeds", nil},
		{"close fails", closeErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := gameTestFS{"index.json": `{}`}
			game := &directSpriteTestGame{}
			base := setupGameTestRuntime(t, files, game, []Sprite{&DirectLoadSprite{}}, false)
			platform := &loadGamePlatformMgr{title: "unchanged"}
			pkgengine.PlatformMgr = platform
			setupLoadGameFlags(t)
			base.lifecycleState.IsRunned.Store(false)
			base.tilemapMgr = gameTilemapMgr{}
			previousFS := &loadGameFS{}
			base.fs = previousFS
			resource := &loadGameFS{
				gameTestFS: files,
				closeErr:   tt.closeErr,
			}

			err := base.loadGame(resource, base.bootstrapGeneration())
			if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "DirectLoadSprite") {
				t.Fatalf("loadGame error = %v, want original sprite load error", err)
			}
			if tt.closeErr != nil && !errors.Is(err, tt.closeErr) {
				t.Fatalf("loadGame error = %v, want joined close error", err)
			}
			if resource.closeCalls != 1 {
				t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
			}
			if base.fs != previousFS || previousFS.closeCalls != 0 {
				t.Fatal("failed load did not preserve the previous resource directory")
			}
			if base.tilemapMgr.g != nil || len(base.pendingBootstrap) != 0 || scheduler.LastThreadID() != 0 {
				t.Fatal("sprite load failure continued into tilemap, bootstrap, or script initialization")
			}
			if platform.title != "unchanged" {
				t.Fatalf("failed load changed the window title to %q", platform.title)
			}
		})
	}
}

func TestLoadGameStageFailureClosesResources(t *testing.T) {
	for _, tt := range []struct {
		name    string
		project string
		want    string
		cause   error
	}{
		{"named sprite", `{"zorder":["stageLoadTestSprite"]}`, `zorder[0]: load sprite "stageLoadTestSprite"`, os.ErrNotExist},
		{"sprite group", `{"zorder":[{"type":"sprites","target":"Sprites","items":[{}]}]}`, `zorder[0]: load sprite "stageLoadTestSprite"`, os.ErrNotExist},
		{"undefined sprite", `{"zorder":["Missing"]}`, `zorder[0]: sprite "Missing" is not defined`, nil},
		{"malformed entry after empty group", `{"zorder":[{"type":"sprites","target":"Empty","items":[]},{}]}`, `zorder[1]: invalid stage shape type`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := gameTestFS{"index.json": tt.project}
			game := setupStageLoadTestGame(t, files)
			platform := &loadGamePlatformMgr{title: "unchanged"}
			pkgengine.PlatformMgr = platform
			panicMgr := &runtimePanicRecorder{IExtMgr: pkgengine.ExtMgr}
			pkgengine.ExtMgr = panicMgr
			setupLoadGameFlags(t)
			game.lifecycleState.IsRunned.Store(false)
			previousFS := &loadGameFS{}
			game.fs = previousFS
			game.tilemapMgr.fs = previousFS
			game.tilemapMgr.currentMap = "previous"
			previousTilemap := game.tilemapMgr
			closeErr := errors.New("close failed")
			resource := &loadGameFS{gameTestFS: files, closeErr: closeErr}

			err := game.loadGame(resource, game.bootstrapGeneration())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadGame error = %v, want %q", err, tt.want)
			}
			if tt.cause != nil && !errors.Is(err, tt.cause) {
				t.Fatalf("loadGame error = %v, want original cause %v", err, tt.cause)
			}
			if !errors.Is(err, closeErr) || resource.closeCalls != 1 {
				t.Fatalf("failed load did not close resources once and preserve close error: err=%v closes=%d", err, resource.closeCalls)
			}
			if game.fs != previousFS || previousFS.closeCalls != 0 || game.tilemapMgr != previousTilemap {
				t.Fatal("failed stage load did not restore resource ownership")
			}
			if len(game.pendingBootstrap) != 0 || scheduler.LastThreadID() != 0 || platform.title != "unchanged" {
				t.Fatal("stage failure continued into bootstrap, scripts, or window title update")
			}
			if len(panicMgr.messages) != 0 || len(panicMgr.exits) != 0 {
				t.Fatalf("stage loading reported a fatal error: messages=%v exits=%v", panicMgr.messages, panicMgr.exits)
			}
		})
	}
}

func TestSchedLoopTimeoutWaitsBeforeReporting(t *testing.T) {
	// Restore real frame timing after the bubble advances it with its fake clock.
	t.Cleanup(func() { clock.Start() })
	synctest.Test(t, func(t *testing.T) {
		var warnings bytes.Buffer
		spxlog.SetStdoutOutput(&warnings)
		recorder := &runtimePanicRecorder{}
		originalExt := pkgengine.ExtMgr
		pkgengine.ExtMgr = recorder
		wrap.Init(engine.WaitMainThread)
		t.Cleanup(func() {
			pkgengine.ExtMgr = originalExt
			spxlog.SetStdoutOutput(os.Stdout)
		})
		currentScheduler := setupRuntimeScheduler(t)
		timedOut := make(chan struct{})
		returned := false
		thread := currentScheduler.Create("loop", func(coroutine.Thread) {
			// Start the frame's timeout clock, then expire it without advancing a frame.
			if got := Sched(); got != 0 {
				t.Errorf("initial Sched() = %d, want 0", got)
			}
			time.Sleep(schedTimeoutMs*time.Millisecond + time.Nanosecond)
			close(timedOut)
			Sched()
			returned = true
		})
		<-timedOut
		synctest.Wait()

		if got := warnings.String(); !strings.Contains(got, "[WARN] [SPX] "+coreruntime.LoopExecutionTimedOutMsg) {
			t.Fatalf("timeout warning = %q, want loop timeout warning", got)
		}
		if got := warnings.String(); !strings.Contains(got, "goroutine ") || !strings.Contains(got, "spx/v4.Sched(") {
			t.Fatalf("timeout warning lacks the calling goroutine stack: %q", got)
		}
		// Processing the current frame must leave the timeout suspended.
		currentScheduler.Update()
		synctest.Wait()
		if returned || len(recorder.messages) != 0 || len(recorder.exits) != 0 || thread.Stopped() {
			t.Fatalf("timeout reported before the next frame: returned=%v, messages=%v, exits=%v, stopped=%v",
				returned, recorder.messages, recorder.exits, thread.Stopped())
		}

		clock.Update(0, 0)
		currentScheduler.Update()
		synctest.Wait()
		if want := []string{coreruntime.LoopExecutionTimedOutMsg}; !slices.Equal(recorder.messages, want) {
			t.Errorf("runtime panic messages = %v, want %v", recorder.messages, want)
		}
		if want := []int64{1}; !slices.Equal(recorder.exits, want) {
			t.Errorf("runtime exit codes = %v, want %v", recorder.exits, want)
		}
		// Reporting requests exit and cancels this script during its engine call.
		if returned || !thread.Stopped() {
			t.Errorf("timeout completion: returned=%v, stopped=%v, want false/true", returned, thread.Stopped())
		}
	})
}

func TestGameNativeEntryAndDuplicateStartup(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	setupRuntimeScheduler(t)
	game := &stageLoadTestGame{}
	XGot_Game_Main(game, &stageLoadTestSprite{})
	if engine.GetGame() != &game.Game || game.gamer != game || game.typs["stageLoadTestSprite"] != reflect.TypeFor[stageLoadTestSprite]() {
		t.Fatal("main did not bind and initialize the game")
	}
	defer func() {
		if err, ok := recover().(error); !ok || !errors.Is(err, engine.ErrGameAlreadyRunning) {
			t.Fatalf("second startup panic=%v", err)
		}
	}()
	XGot_Game_Main(&stageLoadTestGame{})
}

func TestLoadGameRejectsInvalidResourceAndSetsAssetDirectory(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	setupLoadGameFlags(t)
	setupImmediateEngineCalls(t)
	var game Game
	if err := game.loadGame(42, 0); err == nil || !strings.Contains(err.Error(), "unsupported resource") {
		t.Fatalf("invalid resource=%v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	resources := &loadGameAssetResMgr{loadGameResMgr: loadGameResMgr{fontError: "font rejected"}}
	pkgengine.ResMgr = resources
	if err := game.loadGame(dir, 0); err == nil || !strings.Contains(err.Error(), "font rejected") {
		t.Fatalf("asset directory load=%v", err)
	}
	if !resources.filesystemMode {
		t.Fatal("asset directory did not select filesystem loading")
	}
}

func TestGameCommandLineHelp(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	setupLoadGameFlags(t, "-h")
	parseCommandLineFlags(&Config{})
	t.Fatal("help returned instead of terminating successfully")
}

func TestGameCommandLineVerboseAndErrors(t *testing.T) {
	for _, arg := range []string{"-v", "-unknown"} {
		t.Run(arg, func(t *testing.T) {
			recorder := setupRuntimeFailureRecorder(t)
			currentScheduler := scheduler
			engine.SetScheduler(nil)
			t.Cleanup(func() { engine.SetScheduler(currentScheduler) })
			originalFlags := defaultDebugFlags.Load()
			t.Cleanup(func() { defaultDebugFlags.Store(originalFlags); spxlog.SetLevel(spxlog.LevelInfo) })
			setupLoadGameFlags(t, arg)
			parseCommandLineFlags(&Config{})
			if arg == "-v" {
				if defaultDebugFlags.Load() != uint32(DbgFlagInstr|DbgFlagEvent|DbgFlagPerf) {
					t.Fatalf("verbose flags=%v", defaultDebugFlags.Load())
				}
			} else if len(recorder.messages) != 1 || !slices.Equal(recorder.exits, []int64{1}) {
				t.Fatalf("parse error messages=%v, exits=%v", recorder.messages, recorder.exits)
			}
		})
	}
}

func TestGameInitializationReportsClaimedInput(t *testing.T) {
	recorder := setupRuntimeFailureRecorder(t)
	currentScheduler := scheduler
	engine.SetScheduler(nil)
	t.Cleanup(func() { engine.SetScheduler(currentScheduler) })
	game := &Game{inputClaimed: true}
	game.initGame(nil)
	if len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], ErrInputSessionActive.Error()) || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("claimed input panic=%v, exits=%v", recorder.messages, recorder.exits)
	}
}

func TestUnknownTouchTargetReportsRuntimeFailure(t *testing.T) {
	recorder := setupRuntimeFailureRecorder(t)
	currentScheduler := scheduler
	engine.SetScheduler(nil)
	t.Cleanup(func() { engine.SetScheduler(currentScheduler) })
	info := new(Game).getTouchFilter("missing")
	if *info != (touchFilter{}) || len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], "Unknown sprite missing") || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("unknown touch filter=%+v, messages=%v, exits=%v", info, recorder.messages, recorder.exits)
	}
}

func TestLoadGameRejectsLateConfigurationBeforeEngineChanges(t *testing.T) {
	const valid = `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}]}`
	for _, test := range []struct{ name, project, second, want string }{
		{"late sprite", `{}`, `{"costumes":[null]}`, `load sprite "Second"`},
		{"late stage", `{"zorder":["First",{}]}`, valid, `zorder[1]: invalid stage shape type`},
		{"lazy sprite", `{"zorder":["First","Missing"]}`, valid, `zorder[1]: sprite "Missing" is not defined`},
		{"tilemap", `{"tilemapPath":"broken.json"}`, valid, `load tilemap`},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := gameTestFS{
				"index.json":                test.project,
				"sprites/First/index.json":  valid,
				"sprites/Second/index.json": test.second,
				"broken.json":               `{`,
			}
			game := &preflightLoadGame{}
			base := setupGameTestRuntime(t, files, game, nil, false)
			base.typs = map[string]reflect.Type{"First": reflect.TypeFor[preflightLoadSprite](), "Second": reflect.TypeFor[preflightLoadSprite]()}
			first, second := &preflightLoadSprite{}, &preflightLoadSprite{}
			game.First, game.Second = first, second
			fonts := &loadGameResMgr{}
			pkgengine.ResMgr = fonts
			mgr := pkgengine.SpriteMgr.(*gameTestSpriteMgr)
			base.fs = &loadGameFS{gameTestFS: files}
			base.tilemapMgr.fs = base.fs
			beforeID, beforeFS, beforeTilemap := mgr.nextID, base.fs, base.tilemapMgr
			setupLoadGameFlags(t)
			resource := &loadGameFS{gameTestFS: files}

			err := base.loadGame(resource, base.bootstrapGeneration())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadGame error = %v, want %q", err, test.want)
			}
			if fonts.applyCalls != 0 || mgr.nextID != beforeID {
				t.Fatalf("preparation changed engine: fonts=%d sprite IDs=%d -> %d", fonts.applyCalls, beforeID, mgr.nextID)
			}
			if game.First != first || game.Second != second || len(base.sprs) != 0 || len(base.getAllShapes()) != 0 {
				t.Fatal("preparation published sprite fields, prototypes, or shapes")
			}
			if base.fs != beforeFS || base.tilemapMgr != beforeTilemap || resource.closeCalls != 1 {
				t.Fatal("preparation changed resource ownership")
			}
			if len(base.pendingBootstrap) != 0 || scheduler.LastThreadID() != 0 {
				t.Fatal("preparation started scripts")
			}
		})
	}
}

func TestLoadGameAppliesPreparedMetadataToLiveFields(t *testing.T) {
	files := gameTestFS{
		"index.json": `{
			"tilemapPath":"tilemaps/map",
			"map":{"width":640,"height":480},
			"zorder":[
				{"type":"sprite","target":"projectTestSprite","x":11},
				{"type":"sprite","target":"Dynamic","x":22},
				{"type":"sprite","target":"Value","x":33},
				{"type":"sprites","target":"Items","items":[{"x":44},{"x":55}]},
				"Lazy"
			]
		}`,
		"sprites/projectTestSprite/index.json": testSpriteConfig,
		"sprites/Dynamic/index.json":           testSpriteConfig,
		"sprites/Value/index.json":             testSpriteConfig,
		"sprites/Lazy/index.json":              testSpriteConfig,
		"tilemaps/map/tilemap.json":            `{}`,
		"tilemaps/map/decorator.json":          `{"decorators":[]}`,
	}
	game := &projectTestGame{}
	base := setupGameTestRuntime(t, files, game, nil, false)
	for _, name := range []string{"projectTestSprite", "Dynamic", "Value", "Lazy"} {
		base.typs[name] = reflect.TypeFor[projectTestSprite]()
	}
	pkgengine.PlatformMgr = &loadGamePlatformMgr{}
	pkgengine.ResMgr = &loadGameResMgr{}
	previousAudio, previousParser := pkgengine.AudioMgr, pkgengine.TilemapparserMgr
	pkgengine.AudioMgr = &loadGameAudioMgr{}
	parser := &gameTestTilemapparserMgr{}
	pkgengine.TilemapparserMgr = parser
	t.Cleanup(func() {
		pkgengine.AudioMgr, pkgengine.TilemapparserMgr = previousAudio, previousParser
	})
	setupLoadGameFlags(t)
	resource := &singleReadProjectFS{
		loadGameFS: loadGameFS{gameTestFS: files},
		reads:      make(map[string]int),
	}

	if err := base.loadGame(resource, base.bootstrapGeneration()); err != nil {
		t.Fatal(err)
	}
	for name := range files {
		if resource.reads[name] != 1 {
			t.Errorf("metadata %q read %d times, want once", name, resource.reads[name])
		}
	}
	if game.projectTestSprite == nil || game.Dynamic == nil || len(game.Items) != 2 || base.sprs["Lazy"] == nil {
		t.Fatal("project did not publish its pointer, interface, group, or lazy sprite")
	}
	want := []Sprite{game.projectTestSprite, game.Dynamic, &game.Value, game.Items[0], game.Items[1], base.sprs["Lazy"]}
	positions := []float64{11, 22, 33, 44, 55, 0}
	shapes := base.getAllShapes()
	if len(shapes) != len(want) {
		t.Fatalf("stage has %d shapes, want %d", len(shapes), len(want))
	}
	for i, sprite := range want {
		live, ok := sprite.(*projectTestSprite)
		if !ok || live == nil || shapes[i] != &live.SpriteImpl || live.sprite != sprite || live.g != base || live.projectTestGame != game {
			t.Fatalf("stage sprite %d is not bound to its live game field and owner", i)
		}
		if live.Xpos() != positions[i] || live.runtimeState.layer != firstSpriteLayer+i {
			t.Errorf("sprite %d: x=%v layer=%d, want x=%v layer=%d", i, live.Xpos(), live.runtimeState.layer, positions[i], firstSpriteLayer+i)
		}
	}
	if base.TilemapName() != "map" || parser.loadedPath == "" {
		t.Fatal("prepared tilemap was not applied")
	}
	if resource.closeCalls != 0 || base.fs != resource {
		t.Fatal("successful loading did not retain ownership of the resource directory")
	}
}

func (f *loadGameFS) Open(name string) (io.ReadCloser, error) {
	if _, ok := f.gameTestFS[name]; !ok {
		return nil, os.ErrNotExist
	}
	return f.gameTestFS.Open(name)
}

func (f *loadGameFS) Close() error {
	f.closeCalls++
	return f.closeErr
}

func (m *loadGameResMgr) ApplyProjectFonts(string, pkgengine.Array, pkgengine.Array, pkgengine.Array) string {
	m.applyCalls++
	return m.fontError
}

func (m *loadGamePlatformMgr) SetWindowTitle(title string) { m.title = title }

func (*loadGameAudioMgr) CreateAudio() pkgengine.Object { return 1 }

func (m *runtimePanicRecorder) OnRuntimePanic(message string) {
	m.messages = append(m.messages, message)
}

func (m *runtimePanicRecorder) RequestExit(code int64) {
	m.exits = append(m.exits, code)
}

func (m *loadGameAssetResMgr) SetLoadMode(filesystem bool) { m.filesystemMode = filesystem }

func (*loadGameAssetResMgr) HasFile(string) bool { return false }

func (*preflightLoadSprite) Main() {}

func (fs *singleReadProjectFS) Open(name string) (io.ReadCloser, error) {
	if _, exists := fs.gameTestFS[name]; exists {
		fs.reads[name]++
		if fs.reads[name] > 1 {
			return nil, fmt.Errorf("project metadata %q was read more than once", name)
		}
	}
	return fs.loadGameFS.Open(name)
}

func setupLoadGameFlags(t *testing.T, args ...string) {
	t.Helper()
	previousFlags, previousArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = append([]string{t.Name()}, args...)
	t.Cleanup(func() {
		flag.CommandLine, os.Args = previousFlags, previousArgs
	})
}

func isolatedGameTest(t *testing.T) bool {
	t.Helper()
	if os.Getenv("SPX_TEST_NATIVE_GAME_CASE") == t.Name() {
		return false
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
	// Keep native startup and os.Exit coverage in the same report as the parent.
	if coverDir := flag.Lookup("test.gocoverdir"); coverDir != nil && coverDir.Value.String() != "" {
		command.Args = append(command.Args, "-test.gocoverdir="+coverDir.Value.String())
		command.Env = append(os.Environ(), "GOCOVERDIR="+coverDir.Value.String())
	} else {
		command.Env = os.Environ()
	}
	command.Env = append(command.Env, "SPX_TEST_NATIVE_GAME_CASE="+t.Name())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated game test: %v\n%s", err, output)
	}
	return true
}

func TestTouchFilterFailurePreservesMasks(t *testing.T) {
	for _, abort := range []bool{false, true} {
		t.Run(fmt.Sprintf("callback panics=%v", abort), func(t *testing.T) {
			recorder := setupRuntimeFailureRecorder(t)
			game := &Game{autoTouchLayers: true, touchFilters: map[string]*touchFilter{
				"A": {layerIndex: 0, Layer: 1, Mask: 8},
				"B": {layerIndex: 1, Layer: 2, Mask: 16},
				"C": {layerIndex: 2, Layer: 4, Mask: 32},
			}}
			a := newCollisionLayerOrderSprite(game, "A", nil)
			b := newCollisionLayerOrderSprite(game, "B", nil)
			c := newCollisionLayerOrderSprite(game, "C", nil)
			previous := pkgengine.SpriteMgr
			mgr := &polygonColliderSpriteMgr{}
			pkgengine.SpriteMgr = mgr
			t.Cleanup(func() { pkgengine.SpriteMgr = previous })
			for i, sprite := range []*collisionLayerOrderSprite{a, b, c} {
				sprite.runtimeState.syncSprite = &engine.Sprite{Sprite: pkgengine.Sprite{Id: int64(i + 1)}}
			}
			a.physics().touchTargets["C"] = true
			b.physics().touchTargets["missing"] = true
			var callbacks int
			pkgengine.ExtMgr = &touchFilterPanicObserver{IExtMgr: recorder, observe: func(message string) {
				callbacks++
				if !strings.Contains(message, "Unknown sprite missing") {
					t.Fatalf("panic message = %q", message)
				}
				if got := []int64{game.touchFilters["A"].Mask, game.touchFilters["B"].Mask, game.touchFilters["C"].Mask}; !slices.Equal(got, []int64{8, 16, 32}) {
					t.Fatalf("masks during panic = %v, want [8 16 32]", got)
				}
				if abort {
					panic("stop collision refresh")
				}
			}}
			game.setupTouchEntries([]Sprite{a, b, c})
			func() {
				defer func() {
					got := recover()
					if abort && got != "stop collision refresh" || !abort && got != nil {
						t.Fatalf("refresh panic = %v", got)
					}
				}()
				game.refreshTouchFilters()
			}()
			want := []int64{8, 16, 32}
			if got := []int64{game.touchFilters["A"].Mask, game.touchFilters["B"].Mask, game.touchFilters["C"].Mask}; !slices.Equal(got, want) {
				t.Fatalf("masks after failure = %v, want %v", got, want)
			}
			if mgr.triggerLayerWrites != 0 || mgr.triggerMaskWrites != 0 || mgr.collisionFilterWrites != 0 {
				t.Fatal("failed refresh wrote proxy filters")
			}
			var wantExits []int64
			if !abort {
				wantExits = []int64{1}
			}
			if callbacks != 1 || !slices.Equal(recorder.exits, wantExits) {
				t.Fatalf("panic callbacks = %d, exit requests = %v", callbacks, recorder.exits)
			}
		})
	}
}

type touchFilterPanicObserver struct {
	pkgengine.IExtMgr
	observe func(string)
}

func (m *touchFilterPanicObserver) OnRuntimePanic(message string) { m.observe(message) }

type configTestResMgr struct {
	pkgengine.IResMgr
	files map[string]string
	reads map[string]int
	lists map[string]string
}

type sourceConfigDir struct {
	resourceTestDir
	files   map[string]string
	openErr error
}

type closeErrorReader struct {
	io.Reader
	err error
}

type closeErrorDir struct {
	resourceTestDir
	err error
}

func TestOpenResourcesKeepsEngineReadsBelowPackedOverlay(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index.json": `{
			"run":{"title":"source"},
			"fontPreferences":["Shared"]
		}`,
		"res://assets/index_pack.json": `{
			"run":{"title":"packed"},
			"fontPreferences":["Shared"],
			"sprites":{"Hero":{"costumes":[{"name":"packed","path":"packed.png"}]}},
			"sounds":{"Jump":{"path":"packed.wav","rate":2}},
			"fonts":{"Shared":{"faces":[{"path":"packed.ttf"}]}}
		}`,
		"res://assets/sprites/Hero/index.json": `{"costumes":[{"name":"source","path":"source.png"}]}`,
		"res://assets/sounds/Jump/index.json":  `{"path":"source.wav","rate":1}`,
		"res://assets/fonts/Shared/index.json": `{"faces":[{"path":"source.ttf"}]}`,
		"res://assets/fonts/Shared/packed.ttf": "packed-font",
		"res://assets/fonts/Shared/source.ttf": "source-font",
	})
	base := &resourceTestDir{path: "res://assets"}

	opened, err := openGameResources(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Config.Title != "packed" {
		t.Fatalf("project title = %q, want packed", opened.Config.Title)
	}
	if len(opened.Fonts.Families) != 1 || len(opened.Fonts.Families[0].Faces) != 1 {
		t.Fatalf("font families = %#v, want one packed face", opened.Fonts.Families)
	}
	if got := opened.Fonts.Families[0].Faces[0].Path; got != "fonts/Shared/packed.ttf" {
		t.Fatalf("font face = %q, want packed face", got)
	}

	sprite, err := coreproject.LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatal(err)
	}
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/packed.png" {
		t.Fatalf("sprite costume = %q, want packed costume", got)
	}
	sound, err := coreproject.LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatal(err)
	}
	if got := sound.Path; got != "sounds/Jump/packed.wav" {
		t.Fatalf("sound path = %q, want packed sound", got)
	}

	if err := opened.FS.Close(); err != nil {
		t.Fatal(err)
	}
	if base.closed != 1 {
		t.Fatalf("base directory close count = %d, want 1", base.closed)
	}
	if got := manager.reads["res://assets/index.json"]; got != 1 {
		t.Fatalf("source root engine reads = %d, want only the overlay merge read", got)
	}
	for _, path := range []string{
		"res://assets/sprites/Hero/index.json",
		"res://assets/sounds/Jump/index.json",
		"res://assets/fonts/Shared/index.json",
	} {
		if manager.reads[path] != 0 {
			t.Fatalf("packed load bypassed overlay and read engine source %q", path)
		}
	}
}

func TestOpenResourcesReadsUnpackedEngineConfigStrictly(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid", content: `{"run":{"title":"engine"}}`},
		{name: "trailing value", content: `{"run":{"title":"engine"}} {}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installConfigTestResMgr(t, map[string]string{"res://assets/index.json": tt.content})
			base := &resourceTestDir{path: "res://assets"}

			opened, err := openGameResources(base, nil)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unexpected content after JSON value") {
					t.Fatalf("openGameResources() error = %v, want trailing JSON error", err)
				}
				if base.closed != 1 {
					t.Fatalf("failed load close count = %d, want 1", base.closed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opened.Config.Title != "engine" {
				t.Fatalf("project title = %q, want engine", opened.Config.Title)
			}
			if err := opened.FS.Close(); err != nil {
				t.Fatal(err)
			}
			if base.closed != 1 {
				t.Fatalf("successful load close count = %d, want 1", base.closed)
			}
		})
	}
}

func TestLoadJSONReadsBareEngineConfigStrictly(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid", content: `{"name":"engine"}`},
		{name: "trailing value", content: `{"name":"engine"} {}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			manager := installConfigTestResMgr(t, map[string]string{"res://assets/direct.json": tt.content})
			base := &resourceTestDir{path: "res://assets"}
			var config struct {
				Name string `json:"name"`
			}

			err := coreproject.LoadJSON(&config, withEngineAssets(base), "direct.json")
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unexpected content after JSON value") {
					t.Fatalf("LoadJSON() error = %v, want trailing JSON error", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if config.Name != "engine" {
					t.Fatalf("config name = %q, want engine", config.Name)
				}
			}
			if got := manager.reads["res://assets/direct.json"]; got != 1 {
				t.Fatalf("engine reads = %d, want 1", got)
			}
		})
	}
}

func TestOpenResourcesPreservesNativeAndWebFontSources(t *testing.T) {
	for _, test := range []struct {
		name, root, engineFile string
	}{
		{"native", "assets", ""},
		{"web binary fallback", "assets", "res://assets/fonts/Local/local.ttf"},
		{"Windows root binary fallback", `\`, "fonts/Local/local.ttf"},
	} {
		t.Run(test.name, func(t *testing.T) {
			engineFiles := map[string]string{}
			base := &sourceConfigDir{
				resourceTestDir: resourceTestDir{path: test.root},
				files: map[string]string{
					"index.json":             `{}`,
					"fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
				},
			}
			if test.engineFile == "" {
				base.files["fonts/Local/local.ttf"] = "native font"
			} else {
				engineFiles[test.engineFile] = "mounted font"
			}
			manager := installConfigTestResMgr(t, engineFiles)
			opened, err := openGameResources(base, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.FS.Close()
			if len(opened.Fonts.Families) != 1 || opened.Fonts.Families[0].Name != "Local" {
				t.Fatalf("font families = %+v, want Local", opened.Fonts.Families)
			}
			if len(manager.reads) != 0 {
				t.Fatalf("local JSON or binary font read through text bridge: %v", manager.reads)
			}
		})
	}
}

func TestPackedOverlayDelegatesEngineFontDiscovery(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index_pack.json":        `{}`,
		"res://assets/fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
		"res://assets/fonts/Local/local.ttf":  "font",
	})
	manager.lists = map[string]string{"res://assets/fonts": `["Local"]`}
	opened, err := openGameResources(&resourceTestDir{path: "res://assets"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	if len(opened.Fonts.Families) != 1 || opened.Fonts.Families[0].Name != "Local" {
		t.Fatalf("font families = %+v, want engine-discovered Local", opened.Fonts.Families)
	}
}

func TestEngineFontVerificationDoesNotFallBackToSource(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://assets/index.json":             `{}`,
		"res://assets/fonts/Local/index.json": `{"faces":[{"path":"local.ttf"}]}`,
	})
	manager.lists = map[string]string{"res://assets/fonts": `["Local"]`}
	base := &sourceConfigDir{
		resourceTestDir: resourceTestDir{path: "res://assets"},
		files:           map[string]string{"fonts/Local/local.ttf": "source font"},
	}
	if _, err := openGameResources(base, nil); err == nil || !strings.Contains(err.Error(), "open font face") {
		t.Fatalf("missing engine font error = %v", err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1 after failed load", base.closed)
	}
}

func TestOpenResourcesPreservesStringAssetDirWithoutPathCapability(t *testing.T) {
	base := &sourceConfigDir{files: map[string]string{"index.json": `{}`}}
	spxfs.RegisterSchema("projectloader-test", func(string) (spxfs.Dir, error) {
		return struct{ spxfs.Dir }{base}, nil
	})
	const resource = "projectloader-test://assets"
	opened, err := openGameResources(resource, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened.AssetDir != resource {
		t.Fatalf("asset directory = %q, want original resource %q", opened.AssetDir, resource)
	}
	if base.closed != 0 {
		t.Fatal("loader closed successfully transferred resources")
	}
	if err := opened.FS.Close(); err != nil {
		t.Fatal(err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1", base.closed)
	}
}

func TestOpenResourcesReportsMalformedEngineDirectoryListing(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{"res://assets/index.json": `{}`})
	manager.lists = map[string]string{"res://assets/fonts": `{}`}
	base := &resourceTestDir{path: "res://assets"}
	if _, err := openGameResources(base, nil); err == nil || !strings.Contains(err.Error(), "scan fonts") {
		t.Fatalf("directory listing error = %v", err)
	}
	if base.closed != 1 {
		t.Fatalf("source closes = %d, want 1 after failed listing", base.closed)
	}
}

func TestVerifyFileDoesNotMaskSourceCloseErrorWithEngineFallback(t *testing.T) {
	installConfigTestResMgr(t, map[string]string{"res://assets/font.ttf": "mounted font"})
	wantErr := errors.New("source close failed")
	dir := withEngineAssets(&closeErrorDir{resourceTestDir: resourceTestDir{path: "assets"}, err: wantErr})
	if err := spxfs.VerifyFile(dir, "font.ttf"); err != wantErr {
		t.Fatalf("font verification error = %v, want original close error %v", err, wantErr)
	}
}

func TestVerifyFilePreservesSourceOpenErrorWhenEngineFileMissing(t *testing.T) {
	manager := installConfigTestResMgr(t, nil)
	for _, wantErr := range []error{os.ErrNotExist, os.ErrPermission} {
		base := &sourceConfigDir{resourceTestDir: resourceTestDir{path: "assets"}, openErr: wantErr}
		if err := spxfs.VerifyFile(withEngineAssets(base), "font.ttf"); err != wantErr {
			t.Fatalf("font verification error = %v, want original source error %v", err, wantErr)
		}
	}
	if len(manager.reads) != 0 {
		t.Fatalf("binary verification read through text bridge: %v", manager.reads)
	}
}

func TestOpenResourcesDoesNotReadPreviousProjectAssets(t *testing.T) {
	manager := installConfigTestResMgr(t, map[string]string{
		"res://next-assets/index.json": `{}`,
	})
	for _, root := range []string{"../assets", "res://assets"} {
		manager.files[root+"/sprites/Hero/index.json"] = `{"costumes":[{"path":"old.png"}]}`
		manager.files[root+"/fonts/Local/local.ttf"] = "old-font"
	}
	opened, err := openGameResources(&resourceTestDir{path: "res://next-assets"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	if _, err := coreproject.LoadSpriteConfig(opened.FS, "Hero"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing sprite error = %v, want ErrNotExist without previous project fallback", err)
	}
	if err := spxfs.VerifyFile(opened.FS, "fonts/Local/local.ttf"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing font error = %v, want ErrNotExist without previous project fallback", err)
	}
	for file := range manager.reads {
		if file != "res://next-assets/index.json" {
			t.Fatalf("read %q from the previous project", file)
		}
	}
}

func (m *configTestResMgr) HasFile(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *configTestResMgr) ReadAllText(path string) string {
	m.reads[path]++
	return m.files[path]
}

func (m *configTestResMgr) ListDirectories(path string) string {
	if listing, ok := m.lists[path]; ok {
		return listing
	}
	return "[]"
}

func (d *sourceConfigDir) Open(name string) (io.ReadCloser, error) {
	if value, ok := d.files[name]; ok {
		return io.NopCloser(strings.NewReader(value)), nil
	}
	if d.openErr != nil {
		return nil, d.openErr
	}
	return nil, os.ErrNotExist
}

func (d *sourceConfigDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	if name == "fonts" {
		return []spxfs.DirEntry{{Name: "Local", IsDir: true}, {Name: "README.txt"}}, nil
	}
	return nil, os.ErrNotExist
}

func (r closeErrorReader) Close() error { return r.err }

func (d *closeErrorDir) Open(string) (io.ReadCloser, error) {
	return closeErrorReader{err: d.err}, nil
}

func installConfigTestResMgr(t *testing.T, files map[string]string) *configTestResMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	manager := &configTestResMgr{files: files, reads: make(map[string]int)}
	previous := pkgengine.ResMgr
	pkgengine.ResMgr = manager
	t.Cleanup(func() { pkgengine.ResMgr = previous })
	return manager
}
