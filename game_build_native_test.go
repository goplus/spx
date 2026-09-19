//go:build !js && !pure_engine

package spx

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type loadGameFS struct{ reloadConfigFS }

func (f loadGameFS) Open(name string) (io.ReadCloser, error) {
	if _, ok := f.reloadConfigFS[name]; !ok {
		return nil, os.ErrNotExist
	}
	return f.reloadConfigFS.Open(name)
}

type loadGameResMgr struct {
	reloadCommitResMgr
	fontError string
}

func (m *loadGameResMgr) ApplyProjectFonts(string, pkgengine.Array, pkgengine.Array, pkgengine.Array) string {
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
	previousAudio, previousFlags, previousArgs := pkgengine.AudioMgr, flag.CommandLine, os.Args
	pkgengine.AudioMgr = &loadGameAudioMgr{}
	flag.CommandLine = flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	os.Args = []string{t.Name()}
	t.Cleanup(func() {
		pkgengine.AudioMgr, flag.CommandLine, os.Args = previousAudio, previousFlags, previousArgs
	})
	base.lifecycleState.IsRunned.Store(false)

	if err := base.loadGame(loadGameFS{files}, base.bootstrapGeneration()); err != nil {
		t.Fatal(err)
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
	events := game.events
	if err := game.loadGame(loadGameFS{files}, game.bootstrapGeneration()); err == nil || !strings.Contains(err.Error(), "font unavailable") {
		t.Fatalf("loadGame error = %v", err)
	}
	if game.events != events || gco.LastThreadID() != 0 {
		t.Fatal("font failure changed the game loops")
	}
}
