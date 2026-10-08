//go:build !pure_engine

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package engine

import (
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func setupSpriteManagerTest(t *testing.T) {
	t.Helper()
	previous := gdx.SpriteMgr
	enginewrap.Init(func(call func()) { call() })
	t.Cleanup(func() {
		gdx.SpriteMgr = previous
		enginewrap.Init(WaitMainThread)
	})
}

type textureUpdateCall struct {
	method string
	id     gdx.Object
	path   string
	rect   mathf.Rect2
	scale  mathf.Vec2
}

type textureUpdateSpy struct {
	gdx.ISpriteMgr
	calls []textureUpdateCall
}

func (s *textureUpdateSpy) SetTexture(id gdx.Object, path string) {
	s.calls = append(s.calls, textureUpdateCall{method: "texture", id: id, path: path})
}

func (s *textureUpdateSpy) SetTextureAtlas(id gdx.Object, path string, rect mathf.Rect2) {
	s.calls = append(s.calls, textureUpdateCall{method: "atlas", id: id, path: path, rect: rect})
}

func (s *textureUpdateSpy) SetRenderScale(id gdx.Object, scale mathf.Vec2) {
	s.calls = append(s.calls, textureUpdateCall{method: "scale", id: id, scale: scale})
}

func TestSpriteTextureUpdates(t *testing.T) {
	setupSpriteManagerTest(t)
	previousPaths := assetPaths
	t.Cleanup(func() { assetPaths = previousPaths })
	assetPaths = assetPathState{root: "assets/", projectRoot: "."}

	const id = gdx.Object(42)
	const renderScale = 1.5
	rect := mathf.Rect2{Position: mathf.NewVec2(3, 5), Size: mathf.NewVec2(7, 11)}
	sprite := &Sprite{Sprite: gdx.Sprite{Id: id}}
	assetPath := ToAssetPath("sprites/cat.png")
	scaleCall := textureUpdateCall{method: "scale", id: id, scale: mathf.NewVec2(renderScale, renderScale)}
	for _, update := range []struct {
		name string
		call func(string, float64, bool)
		rect mathf.Rect2
	}{
		{name: "texture", call: sprite.UpdateTexture},
		{name: "atlas", rect: rect, call: func(path string, scale float64, update bool) {
			sprite.UpdateTextureAtlas(path, rect, scale, update)
		}},
	} {
		t.Run(update.name, func(t *testing.T) {
			textureCall := textureUpdateCall{method: update.name, id: id, path: assetPath, rect: update.rect}
			invalidPathCall := textureUpdateCall{method: update.name, id: id, rect: update.rect}
			for _, tt := range []struct {
				name          string
				path          string
				updateTexture bool
				want          []textureUpdateCall
			}{
				{name: "update", path: "sprites/cat.png", updateTexture: true, want: []textureUpdateCall{textureCall, scaleCall}},
				{name: "scale only", path: "sprites/cat.png", want: []textureUpdateCall{scaleCall}},
				{name: "invalid path", path: "https://example.com/cat.png", updateTexture: true, want: []textureUpdateCall{invalidPathCall, scaleCall}},
				{name: "invalid path scale only", path: "https://example.com/cat.png", want: []textureUpdateCall{scaleCall}},
				{name: "empty path", updateTexture: true},
				{name: "empty path scale only"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					spy := &textureUpdateSpy{}
					gdx.SpriteMgr = spy
					update.call(tt.path, renderScale, tt.updateTexture)

					if !slices.Equal(spy.calls, tt.want) {
						t.Fatalf("engine calls = %+v, want %+v", spy.calls, tt.want)
					}
				})
			}
		})
	}
}

type positionQuerySpy struct {
	gdx.ISpriteMgr
	query func([]int64, []float32) bool
}

func (s *positionQuerySpy) BatchRetrievePositions(ids []int64, out []float32) bool {
	return s.query(ids, out)
}

func TestPositionSyncReusesStorageAndPreservesIDs(t *testing.T) {
	setupSpriteManagerTest(t)
	ids := []int64{0x112233447fc00001, -1}
	calls := 0
	gdx.SpriteMgr = &positionQuerySpy{query: func(input []int64, out []float32) bool {
		calls++
		if len(input) != len(ids) || &input[0] != &ids[0] || input[0] != 0x112233447fc00001 || input[1] != -1 {
			t.Fatal("position query changed the original ID buffer")
		}
		if len(out) != len(ids)*2 {
			t.Fatal("position query has incorrect output length")
		}
		copy(out, []float32{float32(calls), -4, 5, -6})
		return true
	}}

	var buffer SpriteSyncBuffer
	first := buffer.GetPositions(ids)
	storage := &first[0]
	for range 2 {
		buffer.Clear()
		out := buffer.GetPositions(ids)
		if &out[0] != storage || out[0] != float32(calls) || out[3] != -6 {
			t.Fatal("position query did not refresh results in reusable storage")
		}
	}
	if out := buffer.GetPositions(nil); len(out) != 0 || calls != 3 {
		t.Fatal("empty position query reached the engine")
	}
	if out := buffer.GetPositions(ids); &out[0] != storage {
		t.Fatal("empty query discarded reusable storage")
	}

	var other SpriteSyncBuffer
	if out := other.GetPositions(ids); &out[0] == storage || first[0] != 4 {
		t.Fatal("independent sync buffers share position storage")
	}
	ids = append(ids, 2, 3, 4, 5, 6, 7, 8)
	if out := buffer.GetPositions(ids); len(out) != len(ids)*2 || out[0] != float32(calls) {
		t.Fatal("position storage did not grow with the batch")
	}
}

func TestPositionSyncSkipsFailedQueryAndKeepsStorage(t *testing.T) {
	setupSpriteManagerTest(t)
	success := true
	gdx.SpriteMgr = &positionQuerySpy{query: func(_ []int64, out []float32) bool {
		if success {
			copy(out, []float32{3, 4})
		}
		return success
	}}
	var buffer SpriteSyncBuffer
	ids := []int64{1}
	first := buffer.GetPositions(ids)
	success = false
	if got := buffer.GetPositions(ids); len(got) != 0 {
		t.Fatalf("failed query returned stale positions: %v", got)
	}
	success = true
	if got := buffer.GetPositions(ids); &got[0] != &first[0] {
		t.Fatal("failed query discarded reusable storage")
	}
}
