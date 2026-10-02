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

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type loadGameFS struct{ reloadConfigFS }

func (f loadGameFS) Open(name string) (io.ReadCloser, error) {
	if _, ok := f.reloadConfigFS[name]; !ok {
		return nil, os.ErrNotExist
	}
	return f.reloadConfigFS.Open(name)
}

type closingLoadGameFS struct {
	loadGameFS
	closeCalls int
	closeErr   error
}

func (f *closingLoadGameFS) Close() error {
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
	resource := &closingLoadGameFS{loadGameFS: loadGameFS{files}}

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
	if got := gco.LastThreadID(); got != 3 {
		t.Fatalf("game loops = %d, want 3", got)
	}
}

func TestLoadGameFontFailureDoesNotStartScripts(t *testing.T) {
	files := reloadConfigFS{"index.json": `{}`}
	game := setupReloadCommitGame(t, files)
	pkgengine.ResMgr = &loadGameResMgr{fontError: "font unavailable"}
	setupLoadGameFlags(t)
	resource := &closingLoadGameFS{loadGameFS: loadGameFS{files}}
	events := game.events
	if err := game.loadGame(resource, game.bootstrapGeneration()); err == nil || !strings.Contains(err.Error(), "font unavailable") {
		t.Fatalf("loadGame error = %v", err)
	}
	if resource.closeCalls != 1 {
		t.Fatalf("resource close calls = %d, want 1", resource.closeCalls)
	}
	if game.events != events || gco.LastThreadID() != 0 {
		t.Fatal("font failure changed the game loops")
	}
}

func TestLoadGameAndReloadShareProjectValidation(t *testing.T) {
	const project = `{"physics":true}`
	files := reloadConfigFS{"index.json": project}
	setupLoadGameFlags(t)

	initialErr := new(Game).loadGame(loadGameFS{files}, 0)
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
	resource := &closingLoadGameFS{
		loadGameFS: loadGameFS{reloadConfigFS{"index.json": `{"physics":true}`}},
		closeErr:   closeErr,
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
	resource := &closingLoadGameFS{loadGameFS: loadGameFS{files}}

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

type loadGamePanicExtMgr struct {
	pkgengine.IExtMgr
	messages []string
	exits    []int64
}

func (m *loadGamePanicExtMgr) OnRuntimePanic(message string) {
	m.messages = append(m.messages, message)
}

func (m *loadGamePanicExtMgr) RequestExit(code int64) { m.exits = append(m.exits, code) }

func TestLoadGameSpritesStopsAtFirstErrorWhenPanicReturns(t *testing.T) {
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
	base.tilemapMgr = gameTilemapMgr{}
	panicMgr := &loadGamePanicExtMgr{}
	pkgengine.ExtMgr = panicMgr

	loadGameSprites(base, reflect.ValueOf(game).Elem(), files, &coreproject.ProjectConfig{})

	if game.Skipped == nil || game.First == nil {
		t.Fatal("sprite fields were not allocated before filtering and loading")
	}
	if _, ok := game.Data.(*int); !ok {
		t.Fatalf("non-sprite interface field = %T, want *int", game.Data)
	}
	if game.Later != nil {
		t.Fatal("sprite traversal continued after the first load error")
	}
	if want := []string{"file not found: sprites/First/index.json"}; !reflect.DeepEqual(panicMgr.messages, want) {
		t.Fatalf("runtime panic messages = %v, want %v", panicMgr.messages, want)
	}
	if want := []int64{1}; !reflect.DeepEqual(panicMgr.exits, want) {
		t.Fatalf("runtime exit codes = %v, want %v", panicMgr.exits, want)
	}
	if base.tilemapMgr.g != base || base.tilemapMgr.fs == nil {
		t.Fatal("tilemap initialization was skipped after the panic handler returned")
	}
}
