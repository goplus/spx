package spx

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v3/fs"
	internalaudio "github.com/goplus/spx/v3/internal/audio"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type penCanvasTestDir map[string]string

func (d penCanvasTestDir) Open(name string) (io.ReadCloser, error) {
	content, ok := d[name]
	if !ok {
		return nil, errors.New("file not found")
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (penCanvasTestDir) Close() error { return nil }

type penCanvasTilemapMgr struct {
	gdx.ITilemapMgr
	events *[]string
}

func (*penCanvasTilemapMgr) SetLayerIndex(int64) {}
func (m *penCanvasTilemapMgr) PlaceTilesWithLayer(gdx.Array, string, int64) {
	*m.events = append(*m.events, "tiles")
}

type penCanvasCameraMgr struct {
	gdx.ICameraMgr
	limits [4]int64
	calls  int
}

func (m *penCanvasCameraMgr) SetCameraLimit(side, limit int64) {
	m.calls++
	m.limits[side] = limit
}

func (*penCanvasCameraMgr) SetCameraSmoothing(bool) {}

type penCanvasAudioBackend struct {
	internalaudio.Backend
}

func (*penCanvasAudioBackend) CreateAudio() gdx.Object { return 1 }

func newPenCanvasTilemapTestGame(t *testing.T) (*Game, *spyPenMgr, *penCanvasCameraMgr, string) {
	t.Helper()

	spy := setupSpyPenMgr(t)
	originalTilemapMgr := gdx.TilemapMgr
	gdx.TilemapMgr = &penCanvasTilemapMgr{events: &spy.events}
	t.Cleanup(func() {
		gdx.TilemapMgr = originalTilemapMgr
	})
	originalCameraMgr := gdx.CameraMgr
	cameraSpy := &penCanvasCameraMgr{}
	gdx.CameraMgr = cameraSpy
	t.Cleanup(func() {
		gdx.CameraMgr = originalCameraMgr
	})

	const tilemapPath = "tilemaps/map1.json"
	fs := penCanvasTestDir{
		tilemapPath: `{
			"tilemap": {
				"tile_size": {"width": 32, "height": 16},
				"tileset": {"sources": []},
				"layers": [{"tile_data": [1, 2, 3, 0, 0, 1, 4, 5, 0, 0]}]
			},
			"decorators": [],
			"sprites": []
		}`,
	}
	game := &Game{fs: spxfs.Dir(fs)}
	game.displayState.WorldWidth = 480
	game.displayState.WorldHeight = 360
	game.tilemapMgr.g = game
	game.tilemapMgr.fs = fs
	game.camera = &cameraImpl{g: game}
	game.Camera = game.camera
	game.soundMgr.Init(&penCanvasAudioBackend{})
	return game, spy, cameraSpy, tilemapPath
}

func assertStageGeometryUsesTilemapWorld(t *testing.T, game *Game, penSpy *spyPenMgr, cameraSpy *penCanvasCameraMgr) {
	t.Helper()

	if game.displayState.WorldWidth != 96 || game.displayState.WorldHeight != 48 {
		t.Fatalf("world size = %dx%d, want 96x48", game.displayState.WorldWidth, game.displayState.WorldHeight)
	}
	if penSpy.canvasCalls != 1 || penSpy.canvasWidth != 96 || penSpy.canvasHeight != 48 {
		t.Fatalf("pen canvas calls/size = %d/%dx%d, want 1/96x48", penSpy.canvasCalls, penSpy.canvasWidth, penSpy.canvasHeight)
	}
	wantLimits := [4]int64{64, -80, 160, -32}
	if cameraSpy.calls != 4 || cameraSpy.limits != wantLimits {
		t.Fatalf("camera limit calls/values = %d/%v, want 4/%v", cameraSpy.calls, cameraSpy.limits, wantLimits)
	}
}

func TestSetupAudioAndTilemapSyncsPenCanvasAfterWorldSize(t *testing.T) {
	game, penSpy, cameraSpy, tilemapPath := newPenCanvasTilemapTestGame(t)
	game.tilemapMgr.loadMap(tilemapPath)

	game.setupAudioAndTilemap(&coreproject.ProjectConfig{})

	assertStageGeometryUsesTilemapWorld(t, game, penSpy, cameraSpy)
	if want := []string{"tiles", "canvas"}; !reflect.DeepEqual(penSpy.events, want) {
		t.Fatalf("events = %v, want %v", penSpy.events, want)
	}
}

func TestLoadTilemapSyncsPenCanvasAfterWorldSize(t *testing.T) {
	game, penSpy, cameraSpy, tilemapPath := newPenCanvasTilemapTestGame(t)
	game.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
	game.queuePenMove(1, mathf.NewVec2(10, 20))

	game.LoadTilemap(tilemapPath)

	assertStageGeometryUsesTilemapWorld(t, game, penSpy, cameraSpy)
	if want := []string{"tiles", "batch", "canvas"}; !reflect.DeepEqual(penSpy.events, want) {
		t.Fatalf("events = %v, want %v", penSpy.events, want)
	}
}
