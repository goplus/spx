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
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"math"
	"strings"
	"testing"
)

func TestPrepareSpineConfig(t *testing.T) {
	source := &coreproject.SpriteConfig{SpineAnim: &coreproject.SpineAnimConfig{Atlas: "hero.atlas", Skeleton: "hero.skel", AnimMap: map[string]string{"walk": "run"}}, FAnimations: map[string]*coreproject.AniConfig{"walk": {IsLoop: true}}}
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
		if _, err := prepareSpineAnimation(config); err == nil {
			t.Fatal("accepted missing resource")
		}
	}
	if _, err := prepareSpineAnimations(map[string]*coreproject.AniConfig{"bad": nil}); err == nil {
		t.Fatal("accepted null metadata")
	}
	if _, err := prepareSpineAnimations(map[string]*coreproject.AniConfig{"bad": {StepDuration: math.NaN()}}); err == nil {
		t.Fatal("accepted invalid metadata timing")
	}
}

func TestPrepareSpineRejectsEmptyAnimationNames(t *testing.T) {
	for _, names := range []map[string]string{{"": "walk"}, {"walk": ""}} {
		if _, err := prepareSpineAnimation(&coreproject.SpineAnimConfig{Atlas: "a", Skeleton: "s", AnimMap: names}); err == nil {
			t.Fatal("accepted empty animation name")
		}
	}
}
