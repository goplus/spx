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

package scratchcompat_test

import (
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

type costumeSprite struct{ spx.SpriteImpl }

func (*costumeSprite) Main() {}

type customCostumeResolver struct{ costume string }

func (r *customCostumeResolver) ResolveCostumeIndex(costume string) int {
	if r == nil {
		return -7
	}
	r.costume = costume
	return 23
}

type overridingCostumeSprite struct {
	costumeSprite
	costume string
}

func (s *overridingCostumeSprite) ResolveCostumeIndex(costume string) int {
	s.costume = costume
	return 42
}

func TestResolveCostumeIndexAcceptsSprites(t *testing.T) {
	var sprite spx.Sprite = &costumeSprite{}
	for _, tt := range []struct {
		name     string
		resolver scratchcompat.CostumeResolver
	}{
		{"SpriteImpl", &spx.SpriteImpl{}},
		{"Sprite interface", sprite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for name, want := range map[string]int{"3": 2, "-2": -3, "missing": -1} {
				if got := scratchcompat.ResolveCostumeIndex(tt.resolver, name); got != want {
					t.Errorf("ResolveCostumeIndex(%q) = %d, want %d", name, got, want)
				}
			}
		})
	}
}

func TestResolveCostumeIndexPreservesMethodDispatch(t *testing.T) {
	custom := &customCostumeResolver{}
	if got := scratchcompat.ResolveCostumeIndex(custom, "custom"); got != 23 || custom.costume != "custom" {
		t.Fatalf("custom resolver = (%d, %q), want (23, custom)", got, custom.costume)
	}
	override := &overridingCostumeSprite{}
	var sprite spx.Sprite = override
	if got := scratchcompat.ResolveCostumeIndex(sprite, "override"); got != 42 || override.costume != "override" {
		t.Fatalf("sprite override = (%d, %q), want (42, override)", got, override.costume)
	}
}

func TestResolveCostumeIndexPreservesTypedNilBehavior(t *testing.T) {
	t.Run("custom nil receiver", func(t *testing.T) {
		var resolver *customCostumeResolver
		if got := scratchcompat.ResolveCostumeIndex(resolver, "custom"); got != -7 {
			t.Fatalf("nil custom resolver = %d, want -7", got)
		}
	})
	t.Run("nil SpriteImpl panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("nil SpriteImpl did not panic")
			}
		}()
		var sprite *spx.SpriteImpl
		scratchcompat.ResolveCostumeIndex(sprite, "3")
	})
}
