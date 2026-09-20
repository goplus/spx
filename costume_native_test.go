//go:build !js && !pure_engine && !packmode

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
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type costumeSizeResMgr struct {
	pkgengine.IResMgr
	sizes map[string]mathf.Vec2
	calls int
}

func (m *costumeSizeResMgr) GetImageSize(path string) mathf.Vec2 {
	m.calls++
	return m.sizes[path]
}

func TestCostumeSizeCacheSeparatesAssetRoots(t *testing.T) {
	// Isolate the process-wide filesystem roots from other runtime tests.
	if os.Getenv("SPX_TEST_COSTUME_ASSET_ROOTS") != "1" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCostumeSizeCacheSeparatesAssetRoots$")
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
	sizes := []mathf.Vec2{mathf.NewVec2(32, 48), mathf.NewVec2(96, 128)}
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
			if costume.width != int(sizes[i].X) || costume.height != int(sizes[i].Y) {
				t.Fatalf("project %d, %q: costume size = %dx%d, want %v", i, path, costume.width, costume.height, sizes[i])
			}
		}
	}
	if resources.calls != len(projects) {
		t.Fatalf("image-size loads = %d, want one per asset root (%d)", resources.calls, len(projects))
	}
}
