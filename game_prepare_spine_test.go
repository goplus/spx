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

package spx

import (
	"maps"
	"math"
	"reflect"
	"strings"
	"testing"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

func TestPrepareSpineConfig(t *testing.T) {
	source := &coreproject.SpriteConfig{SpineAnim: &coreproject.SpineAnimConfig{Atlas: "hero.atlas", Skeleton: "hero.skel", AnimMap: map[string]string{"walk": "run", "idle": "stand"}}, FAnimations: map[string]*coreproject.AniConfig{"walk": {IsLoop: true}}}
	prepared, err := prepareSpriteConfig(source)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.layout != nil {
		t.Fatal("Spine must not require frame costumes")
	}
	spine := prepared.config.SpineAnim
	if spine.DefaultMix != .1 || spine.StepDuration != .01 || spine.TurnToDuration != 1 {
		t.Fatalf("defaults: %+v", spine)
	}
	animation := prepared.config.FAnimations["walk"]
	if !animation.IsLoop || animation.StepDuration != .01 || animation.TurnToDuration != 1 || animation.Speed != 1 {
		t.Fatalf("metadata: %+v", animation)
	}
	alias := prepared.config.FAnimations["idle"]
	if alias == nil || alias.StepDuration != .01 || alias.TurnToDuration != 1 || alias.Speed != 1 {
		t.Fatalf("alias defaults: %+v", alias)
	}
	spine.AnimMap["walk"] = "idle"
	if source.SpineAnim.AnimMap["walk"] != "run" || source.SpineAnim.DefaultMix != 0 || source.FAnimations["walk"].StepDuration != 0 {
		t.Fatal("preparation mutated input")
	}
	// Spine frame metadata does not resolve references against costumes.
	source.FAnimations["walk"].FrameFrom = "not a costume"
	source.Costumes = []*coreproject.CostumeConfig{nil}
	if _, err := prepareSpriteConfig(source); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSpineConfigValidation(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, field := range []string{"defaultMix", "stepDuration", "turnToDuration"} {
			config := &coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s"}
			switch field {
			case "defaultMix":
				config.DefaultMix = value
			case "stepDuration":
				config.StepDuration = value
			case "turnToDuration":
				config.TurnToDuration = value
			}
			if _, err := prepareSpineAnimation(config); err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("%s=%v: %v", field, value, err)
			}
		}
	}
	for _, config := range []*coreproject.SpineAnimConfig{{Atlas: "a"}, {Skeleton: "s"}, {}} {
		if _, err := prepareSpriteConfig(&coreproject.SpriteConfig{SpineAnim: config}); err == nil {
			t.Fatal("accepted missing resource")
		}
	}
	if _, err := prepareSpineAnimations(map[string]*coreproject.AniConfig{"bad": nil}, &coreproject.SpineAnimConfig{}); err == nil {
		t.Fatal("accepted null metadata")
	}
	if _, err := prepareSpineAnimations(map[string]*coreproject.AniConfig{"bad": {StepDuration: math.NaN()}}, &coreproject.SpineAnimConfig{}); err == nil {
		t.Fatal("accepted invalid metadata timing")
	}
}

func TestPrepareSpineAnimationDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		animations map[string]*coreproject.AniConfig
		aliases    map[string]string
		want       map[string]*coreproject.AniConfig
	}{
		{"nil metadata", nil, map[string]string{"walk": "run"}, map[string]*coreproject.AniConfig{"walk": {StepDuration: .04, TurnToDuration: 2, Speed: 1}}},
		{"empty metadata", map[string]*coreproject.AniConfig{}, map[string]string{"walk": "run"}, map[string]*coreproject.AniConfig{"walk": {StepDuration: .04, TurnToDuration: 2, Speed: 1}}},
		{"nil without aliases", nil, nil, nil},
		{"empty without aliases", map[string]*coreproject.AniConfig{}, nil, map[string]*coreproject.AniConfig{}},
		{"explicit metadata takes precedence", map[string]*coreproject.AniConfig{"walk": {IsLoop: true}, "wave": {StepDuration: .09, TurnToDuration: 3}},
			map[string]string{"walk": "run", "idle": "stand"}, map[string]*coreproject.AniConfig{
				"walk": {IsLoop: true, StepDuration: .01, TurnToDuration: 1, Speed: 1},
				"wave": {StepDuration: .09, TurnToDuration: 3, Speed: 1},
				"idle": {StepDuration: .04, TurnToDuration: 2, Speed: 1},
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := maps.Clone(tc.animations)
			for name, definition := range original {
				copy := *definition
				original[name] = &copy
			}
			source := &coreproject.SpriteConfig{
				SpineAnim:   &coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s", StepDuration: .04, TurnToDuration: 2, AnimMap: tc.aliases},
				FAnimations: tc.animations,
			}
			prepared, err := prepareSpriteConfig(source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(prepared.config.FAnimations, tc.want) {
				t.Fatalf("definitions = %+v; want %+v", prepared.config.FAnimations, tc.want)
			}
			if !reflect.DeepEqual(source.FAnimations, original) {
				t.Fatal("preparation mutated source definitions")
			}
			for name, definition := range source.FAnimations {
				if prepared.config.FAnimations[name] == definition {
					t.Fatalf("definition %q was not cloned", name)
				}
			}
			again, err := prepareSpriteConfig(&prepared.config)
			if err != nil || !reflect.DeepEqual(again, prepared) {
				t.Fatalf("preparation is not idempotent: %+v, %v", again, err)
			}
		})
	}
	// An explicit invalid declaration must not be replaced by its alias.
	if _, err := prepareSpriteConfig(&coreproject.SpriteConfig{
		SpineAnim:   &coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s", AnimMap: map[string]string{"walk": "run"}},
		FAnimations: map[string]*coreproject.AniConfig{"walk": nil},
	}); err == nil || !strings.Contains(err.Error(), `fAnimations["walk"] is null`) {
		t.Fatalf("null metadata alias accepted: %v", err)
	}
}

func TestPrepareSpineRejectsEmptyAnimationNames(t *testing.T) {
	for _, names := range []map[string]string{{"": "walk"}, {"walk": ""}} {
		if _, err := prepareSpineAnimation(&coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s", AnimMap: names}); err == nil {
			t.Fatal("accepted empty animation name")
		}
	}
}

func TestPrepareSpineRejectsInvalidTurnTiming(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		source := &coreproject.SpriteConfig{
			SpineAnim:   &coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s"},
			FAnimations: map[string]*coreproject.AniConfig{"walk": {TurnToDuration: value}},
		}
		if _, err := prepareSpriteConfig(source); err == nil || !strings.Contains(err.Error(), `fAnimations["walk"].turnToDuration`) {
			t.Fatalf("turnToDuration=%v: %v", value, err)
		}
	}
}
