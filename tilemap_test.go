package spx

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	internalaudio "github.com/goplus/spx/v3/internal/audio"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	tm "github.com/goplus/spx/v3/internal/tilemap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

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
	originalTilemapMgr, originalCameraMgr := gdx.TilemapMgr, gdx.CameraMgr
	gdx.TilemapMgr = &penCanvasTilemapMgr{events: &spy.events}
	cameraSpy := &penCanvasCameraMgr{}
	gdx.CameraMgr = cameraSpy
	t.Cleanup(func() {
		gdx.TilemapMgr, gdx.CameraMgr = originalTilemapMgr, originalCameraMgr
	})

	const tilemapPath = "tilemaps/map1.json"
	fs := gameTestFS{
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
	game := &Game{fs: fs}
	game.displayState.WorldWidth = 480
	game.displayState.WorldHeight = 360
	game.tilemapMgr.g = game
	game.tilemapMgr.fs = fs
	game.camera = &cameraImpl{g: game}
	game.Camera = game.camera
	game.soundMgr.Init(&penCanvasAudioBackend{})
	return game, spy, cameraSpy, tilemapPath
}

func TestTilemapSyncsPenCanvasAfterWorldSize(t *testing.T) {
	for _, tt := range []struct {
		name   string
		load   func(*Game, string)
		events []string
	}{
		{"setup", func(game *Game, path string) {
			game.tilemapMgr.loadMap(path)
			game.setupAudioAndTilemap(&coreproject.ProjectConfig{})
		}, []string{"tiles", "canvas"}},
		{"dynamic load", func(game *Game, path string) {
			game.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
			game.queuePenMove(1, mathf.NewVec2(10, 20))
			game.LoadTilemap(path)
		}, []string{"tiles", "batch", "canvas"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game, penSpy, cameraSpy, tilemapPath := newPenCanvasTilemapTestGame(t)
			tt.load(game, tilemapPath)
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
			if !reflect.DeepEqual(penSpy.events, tt.events) {
				t.Fatalf("events = %v, want %v", penSpy.events, tt.events)
			}
		})
	}
}

func TestLoadStageWithNativeTilemapSkipsBackdrops(t *testing.T) {
	for _, test := range []struct{ name, decorators string }{
		{"missing decorators", ""},
		{"empty decorators", `{"decorators":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := gameTestFS{"tilemaps/map/tilemap.json": `{}`}
			if test.decorators != "" {
				files["tilemaps/map/decorator.json"] = test.decorators
			}
			game := setupStageLoadTestGame(t, files)
			previousParser := gdx.TilemapparserMgr
			parser := &gameTestTilemapparserMgr{}
			gdx.TilemapparserMgr = parser
			t.Cleanup(func() { gdx.TilemapparserMgr = previousParser })
			project := &coreproject.ProjectConfig{
				TilemapPath: "tilemaps/map",
				Map:         coreproject.MapConfig{Width: 640, Height: 480},
				Backdrops: []*coreproject.BackdropConfig{{CostumeConfig: coreproject.CostumeConfig{
					Name: "unused", Path: "unused.png", ImageWidth: 10, ImageHeight: 10,
				}}},
			}
			game.tilemapMgr.init(&game.Game, files, project.TilemapPath)
			if err := game.loadStage(reflect.ValueOf(game).Elem(), project, game.bootstrapGeneration()); err != nil {
				t.Fatal(err)
			}
			if game.TilemapName() != "map" || parser.loadedPath != internalengine.ToAssetPath("tilemaps/map/tilemap.json") {
				t.Fatalf("native tilemap name/path = %q/%q", game.TilemapName(), parser.loadedPath)
			}
			if game.getCostumePath() != "" || game.displayState.WorldWidth != 640 || game.displayState.WorldHeight != 480 {
				t.Fatalf("tilemap stage used backdrop: path=%q, world=%dx%d",
					game.getCostumePath(), game.displayState.WorldWidth, game.displayState.WorldHeight)
			}
			if parser.destroyCalls != 0 {
				t.Fatalf("initial tilemap was destroyed %d times", parser.destroyCalls)
			}
		})
	}
}

type tilemapUnloadSceneMgr struct {
	gdx.ISceneMgr
	clearCalls int
}

func (m *tilemapUnloadSceneMgr) ClearPureSprites() { m.clearCalls++ }

func TestUnloadTilemapClearsStateOnce(t *testing.T) {
	setupImmediateEngineCalls(t)
	for _, tt := range []struct {
		name        string
		newLoader   bool
		wantDestroy int
	}{
		{"legacy", false, 0},
		{"native", true, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			previousScene, previousParser := gdx.SceneMgr, gdx.TilemapparserMgr
			scene := &tilemapUnloadSceneMgr{}
			parser := &gameTestTilemapparserMgr{}
			gdx.SceneMgr, gdx.TilemapparserMgr = scene, parser
			t.Cleanup(func() { gdx.SceneMgr, gdx.TilemapparserMgr = previousScene, previousParser })
			var game Game
			game.tilemapMgr.currentMap = "map1"
			game.tilemapMgr.tilemapDir = "tilemaps/map1"
			game.tilemapMgr.useNewLoader = tt.newLoader
			game.UnloadTilemap()
			game.UnloadTilemap()
			if game.TilemapName() != "" || game.tilemapMgr.tilemapDir != "" || game.tilemapMgr.useNewLoader || game.tilemapMgr.hasData() {
				t.Fatalf("unloaded map retained state: %+v", game.tilemapMgr)
			}
			if scene.clearCalls != 1 || parser.destroyCalls != tt.wantDestroy {
				t.Fatalf("cleanup calls: scene=%d, native=%d; want 1, %d", scene.clearCalls, parser.destroyCalls, tt.wantDestroy)
			}
		})
	}
}

type tilemapAPIMgr struct {
	gdx.ITilemapMgr
	positions []float32
	path      string
	pos       mathf.Vec2
	layer     int64
	erased    bool
	collision []float32
}

func (m *tilemapAPIMgr) PlaceTiles(positions gdx.Array, path string) {
	m.positions = positions.([]float32)
	m.path = path
}
func (m *tilemapAPIMgr) PlaceTile(pos mathf.Vec2, path string) {
	m.pos, m.path, m.erased = pos, path, false
}
func (m *tilemapAPIMgr) EraseTile(pos mathf.Vec2) { m.pos, m.erased = pos, true }
func (m *tilemapAPIMgr) EraseTileWithLayer(pos mathf.Vec2, layer int64) {
	m.pos, m.layer, m.erased = pos, layer, true
}
func (m *tilemapAPIMgr) GetTile(pos mathf.Vec2) string {
	m.pos = pos
	if m.erased {
		return ""
	}
	return m.path
}
func (m *tilemapAPIMgr) GetTileWithLayer(pos mathf.Vec2, layer int64) string {
	m.layer = layer
	return m.GetTile(pos)
}
func (m *tilemapAPIMgr) SetTileWithCollisionInfo(path string, points gdx.Array) {
	m.path, m.collision = path, points.([]float32)
}

func TestGameTilePlacementAndQueries(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := gdx.TilemapMgr
	manager := &tilemapAPIMgr{}
	gdx.TilemapMgr = manager
	t.Cleanup(func() { gdx.TilemapMgr = previous })
	var game Game
	game.PlaceTiles__0([]float64{1.5, 2.5, 3.5, 4.5}, "tile.png")
	if !reflect.DeepEqual(manager.positions, []float32{1.5, 2.5, 3.5, 4.5}) || manager.path != internalengine.ToAssetPath("tile.png") {
		t.Fatalf("placement=%+v", manager)
	}
	game.PlaceTile(4, 5, "tile.png")
	if got := game.GetTile__0(4, 5); got != internalengine.ToAssetPath("tile.png") || manager.pos != mathf.NewVec2(4, 5) {
		t.Fatalf("tile=%q, position=%v", got, manager.pos)
	}
	if got := game.GetTile__1(4, 5, 2); got != manager.path || manager.layer != 2 {
		t.Fatalf("layered tile=%q, layer=%d", got, manager.layer)
	}
	game.EraseTile__0(4, 5)
	if game.GetTile__0(4, 5) != "" {
		t.Fatal("erased tile still present")
	}
	game.PlaceTile(4, 5, "tile.png")
	game.EraseTile__1(4, 5, 2)
	if game.GetTile__1(4, 5, 2) != "" {
		t.Fatal("erased layer tile still present")
	}
	game.setTileInfo("tile.png", []float64{0, 0, 1, 0, 1, 1})
	if !reflect.DeepEqual(manager.collision, []float32{0, 0, 1, 0, 1, 1}) {
		t.Fatalf("collision points=%v", manager.collision)
	}
}

type decoratorSceneMgr struct {
	gdx.ISceneMgr
	path                                  string
	position, pivot, scale, colliderPivot mathf.Vec2
	rotation                              float64
	layer, collider                       int64
	params                                []float64
}

func (m *decoratorSceneMgr) CreateStaticSprite(path string, pos mathf.Vec2, rotation float64, scale mathf.Vec2, layer int64, pivot mathf.Vec2, collider int64, colliderPivot mathf.Vec2, params gdx.Array) gdx.Object {
	m.path, m.position, m.rotation, m.scale, m.layer, m.pivot, m.collider, m.colliderPivot, m.params = path, pos, rotation, scale, layer, pivot, collider, colliderPivot, params.([]float64)
	return 991
}

type decoratorResMgr struct{ gdx.IResMgr }

func (*decoratorResMgr) GetImageSize(string) mathf.Vec2 { return mathf.NewVec2(20, 10) }

func TestTilemapDecoratorCoordinates(t *testing.T) {
	setupImmediateEngineCalls(t)
	previousScene, previousRes := gdx.SceneMgr, gdx.ResMgr
	scene := &decoratorSceneMgr{}
	gdx.SceneMgr, gdx.ResMgr = scene, &decoratorResMgr{}
	t.Cleanup(func() { gdx.SceneMgr, gdx.ResMgr = previousScene, previousRes })
	var game Game
	game.tilemapMgr = gameTilemapMgr{g: &game, useNewLoader: true, tilemapDir: "tilemaps/map", decoratorDatas: &DecoratorJSON{Decorators: []tm.DecoratorNode{{Path: "tree.png", Position: tm.Vec2{X: 3, Y: 4}, Pivot: tm.Vec2{X: 12, Y: 8}, Scale: tm.Vec2{X: 2, Y: 3}, Ratation: 120, ZIndex: 7, ColliderType: "rect", ColliderPivot: tm.Vec2{X: 1, Y: 2}, ColliderParams: []float64{4, 5}}}}}
	game.tilemapMgr.parseTilemap()
	if scene.path != internalengine.ToAssetPath("tilemaps/map/tree.png") || scene.position != mathf.NewVec2(3, 4) || scene.pivot != mathf.NewVec2(2, 3) || scene.rotation != 30 || scene.scale != mathf.NewVec2(2, 3) || scene.layer != 7 || scene.collider != physicsColliderRect || scene.colliderPivot != mathf.NewVec2(13, 10) || !reflect.DeepEqual(scene.params, []float64{4, 5}) {
		t.Fatalf("decorator=%+v", scene)
	}
}

func TestTilemapLoadAndEmptyBounds(t *testing.T) {
	var game Game
	files := gameTestFS{"map.json": `{"tilemap":{"layers":[]}}`}
	game.tilemapMgr.init(&game, files, "map.json")
	if game.TilemapName() != "map" {
		t.Fatalf("map name=%q", game.TilemapName())
	}
	game.tilemapMgr.loadMap("")
	if game.TilemapName() != "map" {
		t.Fatal("empty path replaced map")
	}
	game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
	for _, raw := range []string{`{}`, `{"tilemap":{"layers":[{"tile_data":[]}]}}`, `null`} {
		var data *tm.TscnMapData
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatal(err)
		}
		game.tilemapMgr.datas = data
		game.tilemapMgr.calcWorldSize()
		if game.displayState.WorldWidth != 480 || game.displayState.WorldHeight != 360 {
			t.Fatal("empty bounds changed world")
		}
	}
	defer func() {
		if got := recover(); got == nil || !strings.Contains(fmt.Sprint(got), "Failed to load tilemap") {
			t.Fatalf("missing map panic=%v", got)
		}
	}()
	game.tilemapMgr.loadMap("missing.json")
}
