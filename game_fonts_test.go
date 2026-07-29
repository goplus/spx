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
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

type recordingProjectFontBridge struct {
	defaultPath string
	paths       []string
	families    []string
	preferences []string
	message     string
}

func (b *recordingProjectFontBridge) ApplyProjectFonts(defaultPath string, paths, families, preferences engine.Array) string {
	b.defaultPath = defaultPath
	b.paths = paths.([]string)
	b.families = families.([]string)
	b.preferences = preferences.([]string)
	return b.message
}

func TestApplyRuntimeFontPlan(t *testing.T) {
	bridge := &recordingProjectFontBridge{}
	plan := coreproject.RuntimeFontPlan{
		DefaultPath: "res://engine/fonts/default.ttf",
		Faces: []coreproject.RuntimeFontFace{
			{Path: "asset://scratch.ttf", Family: "Scratch"},
			{Path: "asset://chinese.ttf", Family: "Chinese"},
		},
		Preferences: []string{"Scratch", "Chinese", "default"},
	}
	if err := applyRuntimeFontPlan(bridge, plan); err != nil {
		t.Fatal(err)
	}
	if bridge.defaultPath != plan.DefaultPath ||
		!reflect.DeepEqual(bridge.paths, []string{"asset://scratch.ttf", "asset://chinese.ttf"}) ||
		!reflect.DeepEqual(bridge.families, []string{"Scratch", "Chinese"}) ||
		!reflect.DeepEqual(bridge.preferences, plan.Preferences) {
		t.Fatalf("unexpected bridge payload: %+v", bridge)
	}

	bridge.message = "engine rejected project fonts"
	if err := applyRuntimeFontPlan(bridge, plan); err == nil || err.Error() != bridge.message {
		t.Fatalf("applyRuntimeFontPlan error = %v, want %q", err, bridge.message)
	}
	if err := applyRuntimeFontPlan(nil, plan); err == nil {
		t.Fatal("applyRuntimeFontPlan(nil) succeeded")
	}
}

func TestProjectFontPayloadPreservesExplicitEmptyPreferences(t *testing.T) {
	payload := newProjectFontPayload(coreproject.RuntimeFontPlan{Preferences: []string{}})
	if payload.preferences == nil {
		t.Fatal("explicit empty preferences became nil")
	}
}
