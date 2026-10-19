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
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
}

func (mainThreadTestPlatform) IsMainThread() bool {
	return true
}

// TestMain supplies the raw manager normally installed by gdengine.Link.
func TestMain(m *testing.M) {
	gdx.PlatformMgr = mainThreadTestPlatform{}
	os.Exit(m.Run())
}

func setupImmediateEngineCalls(t testing.TB) {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	t.Cleanup(func() { enginewrap.Init(engine.WaitMainThread) })
}

func TestApplyRuntimeConfig(t *testing.T) {
	t.Setenv("SPX_PROJECT_DIR", filepath.Join(t.TempDir(), "my-project"))
	for _, tt := range []struct {
		name       string
		config     *Config
		project    coreproject.ProjectConfig
		wantTitle  string
		wantKey    string
		wantWidth  int
		wantHeight int
		wantFull   bool
	}{
		{name: "defaults reset previous config", wantTitle: "my-project (by spx)", wantKey: "F8"},
		{
			name: "explicit settings", config: &Config{Title: "Game", Width: 640, Height: 360, FullScreen: true, ScreenshotKey: "F9"},
			project: coreproject.ProjectConfig{Physics: true}, wantTitle: "Game", wantKey: "F9", wantWidth: 640, wantHeight: 360, wantFull: true,
		},
		{
			name: "project fullscreen and environment key", config: &Config{Width: 480, Height: 320},
			project: coreproject.ProjectConfig{FullScreen: true}, wantTitle: "my-project (by spx)", wantKey: "F8", wantWidth: 480, wantHeight: 320, wantFull: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SPX_SCREENSHOT_KEY", "F8")
			game := &Game{runtimeConfigInput: Config{Title: "stale", Width: 99, Height: 88, FullScreen: true}}
			var original Config
			if tt.config != nil {
				original = *tt.config
			}
			game.applyRuntimeConfig(tt.config, &tt.project)
			if game.runtimeConfigInput.Title != tt.wantTitle || game.displayState.WindowWidth != tt.wantWidth || game.displayState.WindowHeight != tt.wantHeight {
				t.Fatalf("runtime config = title %q, window %dx%d; want %q, %dx%d", game.runtimeConfigInput.Title, game.displayState.WindowWidth, game.displayState.WindowHeight, tt.wantTitle, tt.wantWidth, tt.wantHeight)
			}
			if tt.project.FullScreen != tt.wantFull || game.physicsEnabled != tt.project.Physics {
				t.Fatalf("fullscreen/physics = %v/%v, want %v/%v", tt.project.FullScreen, game.physicsEnabled, tt.wantFull, tt.project.Physics)
			}
			if got := os.Getenv("SPX_SCREENSHOT_KEY"); got != tt.wantKey {
				t.Fatalf("screenshot key = %q, want %q", got, tt.wantKey)
			}
			if tt.config != nil && *tt.config != original {
				t.Fatalf("caller config mutated: got %+v, want %+v", *tt.config, original)
			}
		})
	}
}

func TestStoredRuntimeConfigSurvivesProjectReload(t *testing.T) {
	t.Setenv("SPX_PROJECT_DIR", filepath.Join(t.TempDir(), "original"))
	t.Setenv("SPX_SCREENSHOT_KEY", "")
	config := Config{Width: 640, Height: 360}
	game := &Game{}
	game.applyRuntimeConfig(&config, &coreproject.ProjectConfig{Physics: true})
	config.Title, config.Width = "changed by caller", 1
	t.Setenv("SPX_PROJECT_DIR", filepath.Join(t.TempDir(), "replacement"))
	game.applyStoredRuntimeConfig(&coreproject.ProjectConfig{FullScreen: true})
	if game.runtimeConfigInput.Title != "original (by spx)" || game.displayState.WindowWidth != 640 || game.displayState.WindowHeight != 360 {
		t.Fatalf("reload lost stored config: %+v, display %+v", game.runtimeConfigInput, game.displayState)
	}
	if game.physicsEnabled {
		t.Fatal("reload retained the previous project's physics setting")
	}
}

func TestRuntimeConfigTitleFallsBackToWorkingDirectory(t *testing.T) {
	t.Setenv("SPX_PROJECT_DIR", "")
	t.Setenv("SPX_SCREENSHOT_KEY", "")
	dir := t.TempDir()
	t.Chdir(dir)
	got := resolveGameRuntimeConfig(Config{}, &coreproject.ProjectConfig{})
	if want := filepath.Base(dir) + " (by spx)"; got.Title != want {
		t.Fatalf("title = %q, want %q", got.Title, want)
	}
}

func TestResolveTargetPosition(t *testing.T) {
	game := &Game{}
	game.initShapeMgr()
	game.inputMgr.init(game)
	game.inputMgr.setMousePos(mathf.Vec2{X: 17, Y: -23})
	original := newTargetLookupSprite(game, "actor", false)
	original.components.transform = &transformComponent{sprite: &original.SpriteImpl, x: 12, y: -34}
	clone := newTargetLookupSprite(game, "actor", true)
	clone.components.transform = &transformComponent{sprite: &clone.SpriteImpl, x: 56, y: 78}
	game.shapeMgr.add(&clone.SpriteImpl)
	game.shapeMgr.add(&original.SpriteImpl)
	var nilSprite *targetLookupSprite
	for _, tt := range []struct {
		name   string
		target Target
		x, y   float64
		found  bool
	}{
		{"named original", "actor", 12, -34, true},
		{"direct clone", clone, 56, 78, true},
		{"mouse", Mouse, 17, -23, true},
		{"missing name", "absent", 0, 0, false},
		{"nil", nil, 0, 0, false},
		{"typed nil sprite", nilSprite, 0, 0, false},
		{"unsupported edge", Edge, 0, 0, false},
		{"unsupported position", Pos(12345), 0, 0, false},
		{"unsupported type", struct{}{}, 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			x, y, found := game.resolveTargetPosition(tt.target)
			if x != tt.x || y != tt.y || found != tt.found {
				t.Fatalf("position = (%v, %v, %v), want (%v, %v, %v)", x, y, found, tt.x, tt.y, tt.found)
			}
		})
	}
	original.transform().setPositionRaw(0, 0)
	if x, y, found := game.resolveTargetPosition("actor"); x != 0 || y != 0 || !found {
		t.Fatalf("target at origin = (%v, %v, %v), want (0, 0, true)", x, y, found)
	}
}

func TestResolveRandomTargetUsesWorldBoundsAndSeed(t *testing.T) {
	game := &Game{displayState: gameDisplayState{WorldWidth: 5, WorldHeight: 3, WindowWidth: 1, WindowHeight: 1}}
	t.Cleanup(ResetRandomSeed)
	sequence := func() [32][2]float64 {
		SetRandomSeed(42)
		var points [32][2]float64
		for i := range points {
			x, y, found := game.resolveTargetPosition(Random)
			if !found || x < -2 || x > 2 || y < -1 || y > 1 {
				t.Fatalf("random target = (%v, %v, %v), outside centered 5x3 world", x, y, found)
			}
			points[i] = [2]float64{x, y}
		}
		return points
	}
	first, second := sequence(), sequence()
	if first != second {
		t.Fatal("the same seed produced different random target sequences")
	}
	var outsideWindow bool
	for _, point := range first {
		outsideWindow = outsideWindow || point != [2]float64{}
	}
	if !outsideWindow {
		t.Fatal("random targets stayed inside the 1x1 window instead of using the world")
	}
}
