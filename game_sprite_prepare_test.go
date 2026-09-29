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
	"reflect"
	"strings"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func TestInitialLoadAndReloadShareSpritePreparationErrors(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		want       string
	}{
		{
			name:       "null animation",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":null}}`,
			want:       `fAnimations["walk"] is null`,
		},
		{
			name:       "missing frame name",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":{"frameFrom":"missing"}}}`,
			want:       `fAnimations["walk"].frameFrom references missing costume "missing"`,
		},
		{
			name:       "frame index out of range",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":{"frameTo":1}}}`,
			want:       `fAnimations["walk"].frameTo index 1 is outside 1 costumes`,
		},
		{
			name:       "missing costume declaration",
			configJSON: `{}`,
			want:       "configuration must define costumes, costumeSet, or costumeMPSet",
		},
		{
			name:       "multiple invalid animations use stable order",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"z":null,"a":null}}`,
			want:       `fAnimations["a"] is null`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := reloadConfigFS{"sprites/Sprite/index.json": test.configJSON}
			game, _, _ := setupReloadPreflightGame(t, files)
			gamer := reflect.ValueOf(game).Elem()

			initialSprite := &reloadPreflightSprite{reloadPreflightGame: game}
			initialErr := game.loadSprite(initialSprite, "Sprite", gamer)
			if initialErr == nil {
				t.Fatal("initial load accepted invalid sprite config")
			}
			_, reloadErr := prepareReload(&game.Game, gamer, strings.NewReader(`{"zorder":["Sprite"]}`))
			if reloadErr == nil {
				t.Fatal("reload accepted invalid sprite config")
			}

			want := `sprite config "Sprite": ` + test.want
			reloadReason := strings.TrimPrefix(reloadErr.Error(), "reload preflight: ")
			if initialErr.Error() != want || reloadReason != want {
				t.Fatalf("sprite preparation errors: initial=%q reload=%q, want %q", initialErr, reloadReason, want)
			}
		})
	}
}

func TestPrepareSpriteConfigBuildsStableAnimationDefinitions(t *testing.T) {
	sourceAnimation := &coreproject.AniConfig{FrameFrom: "idle", FrameTo: "walk"}
	source := coreproject.SpriteConfig{
		Costumes: []*coreproject.CostumeConfig{{Name: "idle"}, {Name: "walk"}},
		FAnimations: map[string]*coreproject.AniConfig{
			"walk": sourceAnimation,
		},
	}
	sourceSnapshot := *sourceAnimation

	first, err := prepareSpriteConfig(&source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareSpriteConfig(&source)
	if err != nil {
		t.Fatal(err)
	}

	firstAnimation := first.config.FAnimations["walk"]
	secondAnimation := second.config.FAnimations["walk"]
	if firstAnimation == sourceAnimation || secondAnimation == sourceAnimation || firstAnimation == secondAnimation {
		t.Fatal("preparation reused a mutable input or prior prepared animation definition")
	}
	if !reflect.DeepEqual(*sourceAnimation, sourceSnapshot) {
		t.Fatalf("preparation mutated source animation: got %+v, want %+v", *sourceAnimation, sourceSnapshot)
	}
	if !reflect.DeepEqual(*firstAnimation, *secondAnimation) {
		t.Fatalf("repeated preparation differs: first=%+v second=%+v", *firstAnimation, *secondAnimation)
	}
	if firstAnimation.FrameFps != 25 || firstAnimation.TurnToDuration != 1 || firstAnimation.StepDuration != 0.01 ||
		firstAnimation.IFrameFrom != 0 || firstAnimation.IFrameTo != 1 || firstAnimation.Speed != 1 ||
		firstAnimation.Duration != 2.0/25 {
		t.Fatalf("prepared animation = %+v", *firstAnimation)
	}

	definitionSnapshot := *firstAnimation
	sprite := &SpriteImpl{name: "Sprite"}
	sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	animation := &animationComponent{}
	animation.initialize(sprite, &first.config)
	child := animation.cloneFor(&SpriteImpl{name: "child"})
	grandchild := child.cloneFor(&SpriteImpl{name: "grandchild"})
	if child.shared != animation.shared || grandchild.shared != animation.shared {
		t.Fatal("animation clones did not share the prepared definitions")
	}
	if !reflect.DeepEqual(*firstAnimation, definitionSnapshot) {
		t.Fatalf("initialization or cloning mutated prepared animation: got %+v, want %+v", *firstAnimation, definitionSnapshot)
	}
}
