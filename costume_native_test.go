//go:build !js && !packmode

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
	"flag"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v4/internal/assets"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type costumeSizeResMgr struct {
	pkgengine.IResMgr
	sizes map[string]mathf.Vec2
	calls int
}

func TestCostumeSizeCacheSeparatesAssetRoots(t *testing.T) {
	// Isolate the process-wide filesystem roots from other runtime tests.
	if os.Getenv("SPX_TEST_COSTUME_ASSET_ROOTS") != "1" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCostumeSizeCacheSeparatesAssetRoots$")
		// Include the isolated test's counters in the parent coverage report.
		if coverDir := flag.Lookup("test.gocoverdir"); coverDir != nil && coverDir.Value.String() != "" {
			cmd.Args = append(cmd.Args, "-test.gocoverdir="+coverDir.Value.String())
		}
		cmd.Env = append(os.Environ(), "SPX_TEST_COSTUME_ASSET_ROOTS=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("asset-root regression: %v\n%s", err, output)
		}
		return
	}

	enginewrap.Init(func(call func()) { call() })
	resources := &costumeSizeResMgr{sizes: make(map[string]mathf.Vec2)}
	pkgengine.ResMgr = resources
	projects := []string{t.TempDir(), t.TempDir()}
	sizes := []mathf.Vec2{mathf.NewVec2(32.75, 48.25), mathf.NewVec2(96.5, 128.125)}
	for i, project := range projects {
		assetDir := filepath.Join(project, "assets")
		if err := os.Mkdir(assetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		image := filepath.Join(assetDir, "image.png")
		if err := os.WriteFile(image, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		resources.sizes[filepath.ToSlash(image)] = sizes[i]
	}

	for _, i := range []int{0, 1, 0} {
		project := projects[i]
		if err := engine.SetFilesystemRoots(project, filepath.Join(project, "assets")); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"image.png", "./image.png"} {
			costume := newCostume(&coreproject.CostumeConfig{Path: path})
			if costume.frame.ImageSize != sizes[i] || costume.frame.Width != int(sizes[i].X) || costume.frame.Height != int(sizes[i].Y) {
				t.Fatalf("project %d, %q: costume size = %dx%d, want %v", i, path, costume.frame.Width, costume.frame.Height, sizes[i])
			}
		}
	}
	if resources.calls != len(projects) {
		t.Fatalf("image-size loads = %d, want one per asset root (%d)", resources.calls, len(projects))
	}
	// Atlas regions must still see pixel dimensions for the same resource.
	if got := costumePixelSize("image.png"); got != mathf.NewVec2(33, 49) {
		t.Fatalf("atlas image size = %v, want {33 49}", got)
	}
	if resources.calls != len(projects)+1 {
		t.Fatalf("logical and pixel queries shared a cache entry: calls = %d", resources.calls)
	}
}

func TestCostumeFrameGeometryFixtures(t *testing.T) {
	fixtures := []struct {
		name    string
		costume *costume
		display [2]int
		size    [2]float64
		anchor  mathf.Vec2
		isAtlas bool
	}{
		{"sized", newCostumeWithSize(11, 17), [2]int{11, 17}, [2]float64{11, 17}, mathf.Vec2{}, false},
		{"standalone", newCostume(&coreproject.CostumeConfig{
			Name: "fractional", Path: "sprites/fractional.svg", ImageWidth: 55.75, ImageHeight: 54.25,
			BitmapResolution: 2, X: 52, Y: 89, FaceRight: 90,
		}), [2]int{27, 27}, [2]float64{27.875, 27.125}, mathf.NewVec2(12.0625, -30.9375), false},
		{"atlas", newCostumeWith("run1", &costumeSetImage{
			path: "sprites/atlas.png", width: 256, height: 128, nx: 3,
			rc: coreproject.CostumeSetRect{X: 16, Y: 8, W: 131, H: 43},
		}, 90, 1, 2), [2]int{21, 21}, [2]float64{21.5, 21.5}, mathf.Vec2{}, true},
		{"negative atlas index", newCostumeWith("negative", &costumeSetImage{
			path: "sprites/atlas.png", width: 256, height: 128, nx: 3,
			rc: coreproject.CostumeSetRect{X: 16, Y: 8, W: 131, H: 43},
		}, 90, -1, 2), [2]int{21, 21}, [2]float64{128, 64}, mathf.NewVec2(-53.25, 21.25), false},
		{"missing logical size", &costume{frame: assetutil.FrameDescriptor{Width: 11, Height: 17, BitmapResolution: 2}},
			[2]int{5, 8}, [2]float64{5.5, 8.5}, mathf.NewVec2(-2.75, 4.25), false},
		{"zero logical height", &costume{frame: assetutil.FrameDescriptor{Width: 11, Height: 17, ImageSize: mathf.NewVec2(11.5, 0), BitmapResolution: 2}},
			[2]int{5, 8}, [2]float64{5.5, 8.5}, mathf.NewVec2(-2.75, 4.25), false},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			c := fixture.costume
			w, h := c.displaySize()
			x, y := c.sizeInSPX()
			if [2]int{w, h} != fixture.display || [2]float64{x, y} != fixture.size || c.renderAnchorInSPX() != fixture.anchor || c.isAtlas() != fixture.isAtlas {
				t.Fatalf("geometry = %v / %v / %v / %v", [2]int{w, h}, [2]float64{x, y}, c.renderAnchorInSPX(), c.isAtlas())
			}
		})
	}
}

func TestCostumeFrameMetadataOwnership(t *testing.T) {
	c := &costume{
		path: "backdrop.svg",
		frame: assetutil.FrameDescriptor{
			Width: 11, Height: 17, BitmapResolution: 2, ImageSize: mathf.NewVec2(55.75, 54.25),
			Center: mathf.NewVec2(52, 89), PosX: 4, PosY: 6, AtlasUVRect: mathf.NewVec4(0.1, 0.2, 0.3, 0.4),
		},
	}
	costumes := []*costume{c, newCostumeWithSize(7, 9)}
	sources := buildAnimationSources(costumes)
	if len(sources) != len(costumes) {
		t.Fatalf("animation sources = %d, want %d", len(sources), len(costumes))
	}
	for i, source := range sources {
		if source.Path != costumes[i].path || source.Frame != costumes[i].frame {
			t.Fatalf("animation source %d lost metadata: %+v", i, source)
		}
	}
	sources[0].Frame.Center.X++
	if c.frame.Center == sources[0].Frame.Center {
		t.Fatal("animation source aliases the costume descriptor")
	}
	var clone baseObj
	original := baseObj{costumes: []*costume{c}}
	clone.initFrom(&original)
	if clone.currentCostume() != c {
		t.Fatal("clones must continue sharing costume metadata")
	}
	if got := buildAnimationSources(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty animation sources = %#v, want non-nil empty slice", got)
	}
}

func TestBackdropPreservesConfiguredMetadata(t *testing.T) {
	config := &coreproject.BackdropConfig{
		CostumeConfig: coreproject.CostumeConfig{Name: "stage", Path: "stage.svg", ImageWidth: 11.5, ImageHeight: 17.25, FaceRight: 90},
		Pivot:         mathf.NewVec2(3, 4),
	}
	c := newBackdropCostume(config)
	if c.name != config.Name || c.path != config.Path || c.faceRight != config.FaceRight || c.pivot != config.Pivot || c.frame.Center != (mathf.Vec2{}) || c.isAtlas() {
		t.Fatalf("configured backdrop metadata = %+v", c)
	}
}

func (m *costumeSizeResMgr) GetImageSize(path string) mathf.Vec2 {
	m.calls++
	size := m.sizes[path]
	return mathf.NewVec2(math.Ceil(size.X), math.Ceil(size.Y))
}

func (m *costumeSizeResMgr) GetImageLogicalSize(path string) mathf.Vec2 {
	m.calls++
	return m.sizes[path]
}
