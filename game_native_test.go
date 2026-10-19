//go:build !js && !pure_engine

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

	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type loadGameFS struct {
	reloadConfigFS
	closeCalls int
	closeErr   error
}

func (f *loadGameFS) Open(name string) (io.ReadCloser, error) {
	if _, ok := f.reloadConfigFS[name]; !ok {
		return nil, os.ErrNotExist
	}
	return f.reloadConfigFS.Open(name)
}

func (f *loadGameFS) Close() error {
	f.closeCalls++
	return f.closeErr
}

type loadGameResMgr struct {
	reloadCommitResMgr
	fontError  string
	applyCalls int
}

func (m *loadGameResMgr) ApplyProjectFonts(string, pkgengine.Array, pkgengine.Array, pkgengine.Array) string {
	m.applyCalls++
	return m.fontError
}

type loadGamePlatformMgr struct {
	reloadCommitPlatformMgr
	title string
}

func (m *loadGamePlatformMgr) SetWindowTitle(title string) { m.title = title }

type loadGameAudioMgr struct {
	pkgengine.IAudioMgr
}

func (*loadGameAudioMgr) CreateAudio() pkgengine.Object { return 1 }

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

func TestLoadGameLoadsStageBeforeStartingScripts(t *testing.T) {
	files := reloadConfigFS{
		"index.json":                            `{"run":{"title":"loaded game"},"zorder":["DirectCommitSprite"]}`,
		"sprites/DirectCommitSprite/index.json": `{"costumes":[{"name":"default","path":"sprite.png","imageWidth":10,"imageHeight":10}]}`,
	}
	game := &reloadDirectCommitGame{}
	base := setupReloadCommitRuntime(t, files, game, []Sprite{&DirectCommitSprite{}}, false)
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
	resource := &loadGameFS{reloadConfigFS: files}

	if err := base.loadGame(resource, base.bootstrapGeneration()); err != nil {
		t.Fatal(err)
	}
	if resource.closeCalls != 0 || base.fs != resource {
		t.Fatalf("resource ownership not transferred: close calls = %d, game fs = %T", resource.closeCalls, base.fs)
	}
	if game.DirectCommitSprite == nil || game.DirectCommitSprite.g != base {
		t.Fatal("stage sprite was not bound to its game")
	}
	if platform.title != "loaded game" {
		t.Fatalf("window title = %q", platform.title)
	}
	if len(base.pendingBootstrap) == 0 || base.lifecycleState.BootstrapDone.Load() {
		t.Fatal("load must queue bootstrap hooks without running them")
	}
	if got := gco.LastThreadID(); got != 1 {
		t.Fatalf("game loops = %d, want 1", got)
	}
}

func TestLoadGameFontFailureDoesNotStartScripts(t *testing.T) {
	files := reloadConfigFS{"index.json": `{}`}
	game := setupReloadCommitGame(t, files)
	pkgengine.ResMgr = &loadGameResMgr{fontError: "font unavailable"}
	setupLoadGameFlags(t)
	resource := &loadGameFS{reloadConfigFS: files}
	pending := &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	game.inputMgr.frame.pending = pending
	if err := game.loadGame(resource, game.bootstrapGeneration()); err == nil || !strings.Contains(err.Error(), "font unavailable") {
		t.Fatalf("loadGame error = %v", err)
	}
	if resource.closeCalls != 1 {
		t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
	}
	if game.inputMgr.frame.pending != pending || gco.LastThreadID() != 0 {
		t.Fatal("font failure changed pending input or the game loops")
	}
}

func TestLoadGameAndReloadShareProjectValidation(t *testing.T) {
	const project = `{"physics":true}`
	files := reloadConfigFS{"index.json": project}
	setupLoadGameFlags(t)

	initialErr := new(Game).loadGame(&loadGameFS{reloadConfigFS: files}, 0)
	if initialErr == nil {
		t.Fatal("loadGame accepted invalid project settings")
	}
	reloadGame := &Game{fs: files}
	_, reloadErr := prepareReload(reloadGame, reflect.Value{}, strings.NewReader(project))
	if reloadErr == nil {
		t.Fatal("prepareReload accepted invalid project settings")
	}

	initialReason := strings.TrimPrefix(initialErr.Error(), "project config: ")
	reloadReason := strings.TrimPrefix(reloadErr.Error(), "reload preflight: project config: ")
	if initialReason != reloadReason {
		t.Fatalf("project validation differs: initial=%q reload=%q", initialReason, reloadReason)
	}
}

func TestLoadGameProjectValidationClosesResources(t *testing.T) {
	closeErr := errors.New("close failed")
	setupLoadGameFlags(t, "-f")
	previousResMgr := pkgengine.ResMgr
	resMgr := &loadGameResMgr{}
	pkgengine.ResMgr = resMgr
	t.Cleanup(func() { pkgengine.ResMgr = previousResMgr })
	resource := &loadGameFS{
		reloadConfigFS: reloadConfigFS{"index.json": `{"physics":true}`},
		closeErr:       closeErr,
	}

	err := new(Game).loadGame(resource, 0)
	if err == nil || !strings.Contains(err.Error(), "autoSetCollisionLayer and physics") {
		t.Fatalf("loadGame error = %v, want project validation failure", err)
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
}

func TestLoadGameSpriteLoadPanicClosesResources(t *testing.T) {
	files := reloadConfigFS{"index.json": `{}`}
	game := &stagePlanGame{}
	base := setupReloadCommitRuntime(t, files, game, nil, false)
	base.typs["Dynamic"] = reflect.TypeFor[int]()
	setupLoadGameFlags(t)
	resource := &loadGameFS{reloadConfigFS: files}

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
			files := reloadConfigFS{"index.json": `{}`}
			game := &reloadDirectCommitGame{}
			base := setupReloadCommitRuntime(t, files, game, []Sprite{&DirectCommitSprite{}}, false)
			platform := &loadGamePlatformMgr{title: "unchanged"}
			pkgengine.PlatformMgr = platform
			pkgengine.ResMgr = &loadGameResMgr{}
			setupLoadGameFlags(t)
			base.lifecycleState.IsRunned.Store(false)
			base.tilemapMgr = gameTilemapMgr{}
			previousFS := &loadGameFS{}
			base.fs = previousFS
			resource := &loadGameFS{
				reloadConfigFS: files,
				closeErr:       tt.closeErr,
			}

			err := base.loadGame(resource, base.bootstrapGeneration())
			if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "DirectCommitSprite") {
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
			if base.tilemapMgr.g != nil || len(base.pendingBootstrap) != 0 || gco.LastThreadID() != 0 {
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
		{"named sprite", `{"zorder":["reloadCommitSprite"]}`, `zorder[0]: load sprite "reloadCommitSprite"`, os.ErrNotExist},
		{"sprite group", `{"zorder":[{"type":"sprites","target":"Sprites","items":[{}]}]}`, `zorder[0]: load sprite "reloadCommitSprite"`, os.ErrNotExist},
		{"undefined sprite", `{"zorder":["Missing"]}`, `zorder[0]: sprite "Missing" is not defined`, nil},
		{"malformed entry after empty group", `{"zorder":[{"type":"sprites","target":"Empty","items":[]},{}]}`, `zorder[1]: invalid stage shape type`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := reloadConfigFS{"index.json": tt.project}
			game := setupReloadCommitGame(t, files)
			platform := &loadGamePlatformMgr{title: "unchanged"}
			pkgengine.PlatformMgr = platform
			pkgengine.ResMgr = &loadGameResMgr{}
			previousAudio := pkgengine.AudioMgr
			pkgengine.AudioMgr = &loadGameAudioMgr{}
			t.Cleanup(func() { pkgengine.AudioMgr = previousAudio })
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
			resource := &loadGameFS{reloadConfigFS: files, closeErr: closeErr}

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
			if len(game.pendingBootstrap) != 0 || gco.LastThreadID() != 0 || platform.title != "unchanged" {
				t.Fatal("stage failure continued into bootstrap, scripts, or window title update")
			}
			if len(panicMgr.messages) != 0 || len(panicMgr.exits) != 0 {
				t.Fatalf("stage loading reported a fatal error: messages=%v exits=%v", panicMgr.messages, panicMgr.exits)
			}
		})
	}
}

func TestLoadSpriteFieldsReturnsFirstError(t *testing.T) {
	game := &struct {
		Game
		Skipped *reloadPreflightSprite
		Data    any
		First   *reloadPreflightSprite
		Later   *reloadPreflightSprite
	}{}
	files := reloadConfigFS{}
	base := setupReloadCommitRuntime(t, files, game, nil, false)
	base.typs["Data"] = reflect.TypeFor[int]()
	base.typs["First"] = reflect.TypeFor[reloadPreflightSprite]()
	base.typs["Later"] = reflect.TypeFor[reloadPreflightSprite]()
	panicMgr := &runtimePanicRecorder{}
	pkgengine.ExtMgr = panicMgr
	base.startLoad(&loadGameFS{reloadConfigFS: files})

	err := base.loadSpriteFields(reflect.ValueOf(game).Elem())
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), `load sprite "First"`) {
		t.Fatalf("loadSpriteFields error = %v, want named sprite error wrapping os.ErrNotExist", err)
	}
	if game.Skipped == nil || game.First == nil {
		t.Fatal("sprite fields were not allocated before filtering and loading")
	}
	if _, ok := game.Data.(*int); !ok {
		t.Fatalf("non-sprite interface field = %T, want *int", game.Data)
	}
	if game.Later != nil {
		t.Fatal("sprite traversal continued after the first load error")
	}
	if len(panicMgr.messages) != 0 || len(panicMgr.exits) != 0 {
		t.Fatalf("loading helper reported a fatal error: messages=%v exits=%v", panicMgr.messages, panicMgr.exits)
	}
}

type reloadWorkerPlatformMgr struct {
	pkgengine.IPlatformMgr
}

func (*reloadWorkerPlatformMgr) IsMainThread() bool { return false }

func TestReloadRejectsNonEngineThreadBeforePreflight(t *testing.T) {
	game, sprite, _ := setupReloadPreflightGame(t, nil)
	pkgengine.PlatformMgr = &reloadWorkerPlatformMgr{}

	err := XGot_Game_Reload(game, strings.NewReader("{"))
	if !errors.Is(err, errReloadWrongThread) {
		t.Fatalf("XGot_Game_Reload error = %v, want %v", err, errReloadWrongThread)
	}
	if game.Game.sprs["Sprite"] != sprite || game.Sprite != sprite {
		t.Fatal("off-thread reload mutated the live game")
	}
	assertReloadAvailable(t, &game.Game)
}

type runtimePanicRecorder struct {
	pkgengine.IExtMgr
	messages []string
	exits    []int64
}

func (m *runtimePanicRecorder) OnRuntimePanic(message string) {
	m.messages = append(m.messages, message)
}

func (m *runtimePanicRecorder) RequestExit(code int64) {
	m.exits = append(m.exits, code)
}

func TestSchedLoopTimeoutWaitsBeforeReporting(t *testing.T) {
	// Restore real frame timing after the bubble advances it with its fake clock.
	t.Cleanup(func() { itime.Start(nil) })
	synctest.Test(t, func(t *testing.T) {
		var warnings bytes.Buffer
		spxlog.SetStdoutOutput(&warnings)
		recorder := &runtimePanicRecorder{}
		originalExt := pkgengine.ExtMgr
		pkgengine.ExtMgr = recorder
		enginewrap.Init(engine.WaitMainThread)
		t.Cleanup(func() {
			pkgengine.ExtMgr = originalExt
			spxlog.SetStdoutOutput(os.Stdout)
		})
		co := setupRuntimeScheduler(t)
		timedOut := make(chan struct{})
		returned := false
		thread := co.Create("loop", func(coroutine.Thread) {
			// Start the frame's timeout clock, then expire it without advancing a frame.
			if got := Sched(); got != 0 {
				t.Errorf("initial Sched() = %d, want 0", got)
			}
			time.Sleep(schedTimeoutMs*time.Millisecond + time.Nanosecond)
			close(timedOut)
			if got := Sched(); got != 0 {
				t.Errorf("timed-out Sched() = %d, want 0", got)
			}
			returned = true
		})
		<-timedOut
		synctest.Wait()

		if got := warnings.String(); !strings.Contains(got, "[WARN] [SPX] "+coreruntime.LoopExecutionTimedOutMsg) {
			t.Fatalf("timeout warning = %q, want loop timeout warning", got)
		}
		// Processing the current frame must leave the timeout suspended.
		co.Update()
		synctest.Wait()
		if returned || len(recorder.messages) != 0 || len(recorder.exits) != 0 || thread.Stopped() {
			t.Fatalf("timeout reported before the next frame: returned=%v, messages=%v, exits=%v, stopped=%v",
				returned, recorder.messages, recorder.exits, thread.Stopped())
		}

		itime.Update(0, 0)
		co.Update()
		synctest.Wait()
		if want := []string{coreruntime.LoopExecutionTimedOutMsg}; !slices.Equal(recorder.messages, want) {
			t.Errorf("runtime panic messages = %v, want %v", recorder.messages, want)
		}
		if want := []int64{1}; !slices.Equal(recorder.exits, want) {
			t.Errorf("runtime exit codes = %v, want %v", recorder.exits, want)
		}
		if !returned || !thread.Stopped() {
			t.Errorf("timeout completion: returned=%v, stopped=%v, want both true", returned, thread.Stopped())
		}
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

func TestGameNativeEntryAndDuplicateStartup(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	setupRuntimeScheduler(t)
	game := &reloadCommitGame{}
	XGot_Game_Main(game, &reloadCommitSprite{})
	if engine.GetGame() != &game.Game || game.gamer != game || game.typs["reloadCommitSprite"] != reflect.TypeFor[reloadCommitSprite]() {
		t.Fatal("main did not bind and initialize the game")
	}
	defer func() {
		if err, ok := recover().(error); !ok || !errors.Is(err, engine.ErrGameAlreadyRunning) {
			t.Fatalf("second startup panic=%v", err)
		}
	}()
	XGot_Game_Main(&reloadCommitGame{})
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

type loadGameAssetResMgr struct {
	loadGameResMgr
	filesystemMode bool
}

func (m *loadGameAssetResMgr) SetLoadMode(filesystem bool) { m.filesystemMode = filesystem }
func (*loadGameAssetResMgr) HasFile(string) bool           { return false }

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
			co := gco
			engine.SetCoroutines(nil)
			t.Cleanup(func() { engine.SetCoroutines(co) })
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
	co := gco
	engine.SetCoroutines(nil)
	t.Cleanup(func() { engine.SetCoroutines(co) })
	game := &Game{inputClaimed: true}
	game.initGame(nil)
	if len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], ErrInputSessionActive.Error()) || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("claimed input panic=%v, exits=%v", recorder.messages, recorder.exits)
	}
}

func TestUnknownSpriteCollisionReportsRuntimeFailure(t *testing.T) {
	recorder := setupRuntimeFailureRecorder(t)
	co := gco
	engine.SetCoroutines(nil)
	t.Cleanup(func() { engine.SetCoroutines(co) })
	info := new(Game).getSpriteCollisionInfo("missing")
	if *info != (spriteCollisionInfo{}) || len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], "Unknown sprite missing") || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("unknown collision=%+v, messages=%v, exits=%v", info, recorder.messages, recorder.exits)
	}
}
