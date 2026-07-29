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
	"errors"
	"slices"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

type projectFontBridge interface {
	ApplyProjectFonts(defaultPath string, paths, families, preferences engine.Array) string
}

type projectFontPayload struct {
	defaultPath string
	paths       []string
	families    []string
	preferences []string
}

func newProjectFontPayload(plan coreproject.RuntimeFontPlan) projectFontPayload {
	payload := projectFontPayload{
		defaultPath: plan.DefaultPath,
		paths:       make([]string, len(plan.Faces)),
		families:    make([]string, len(plan.Faces)),
		preferences: slices.Clone(plan.Preferences),
	}
	for i, face := range plan.Faces {
		payload.paths[i] = face.Path
		payload.families[i] = face.Family
	}
	return payload
}

func applyRuntimeFontPlan(res projectFontBridge, plan coreproject.RuntimeFontPlan) error {
	if res == nil {
		return errors.New("project font bridge is nil")
	}
	payload := newProjectFontPayload(plan)
	if message := res.ApplyProjectFonts(payload.defaultPath, payload.paths, payload.families, payload.preferences); message != "" {
		return errors.New(message)
	}
	return nil
}
