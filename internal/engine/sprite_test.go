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
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

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
	previousManager, previousPaths := gdx.SpriteMgr, assetPaths
	t.Cleanup(func() {
		gdx.SpriteMgr, assetPaths = previousManager, previousPaths
	})
	assetPaths = assetPathState{root: "assets/", projectRoot: "."}

	const id = gdx.Object(42)
	const renderScale = 1.5
	rect := mathf.Rect2{Position: mathf.NewVec2(3, 5), Size: mathf.NewVec2(7, 11)}
	sprite := &Sprite{Sprite: gdx.Sprite{Id: id}}
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
			for _, tt := range []struct {
				name          string
				path          string
				updateTexture bool
			}{
				{name: "update", path: "sprites/cat.png", updateTexture: true},
				{name: "scale only", path: "sprites/cat.png"},
				{name: "invalid path", path: "https://example.com/cat.png", updateTexture: true},
				{name: "invalid path scale only", path: "https://example.com/cat.png"},
				{name: "empty path", updateTexture: true},
				{name: "empty path scale only"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					spy := &textureUpdateSpy{}
					gdx.SpriteMgr = spy
					update.call(tt.path, renderScale, tt.updateTexture)

					var want []textureUpdateCall
					if tt.path != "" {
						if tt.updateTexture {
							want = append(want, textureUpdateCall{method: update.name, id: id, path: ToAssetPath(tt.path), rect: update.rect})
						}
						want = append(want, textureUpdateCall{method: "scale", id: id, scale: mathf.NewVec2(renderScale, renderScale)})
					}
					if !slices.Equal(spy.calls, want) {
						t.Fatalf("engine calls = %+v, want %+v", spy.calls, want)
					}
				})
			}
		})
	}
}
