//go:build !js && !pure_engine

package spx

import (
	"errors"
	"flag"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

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
