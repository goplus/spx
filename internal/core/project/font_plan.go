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

package project

import "slices"

// defaultDisplayFontPath selects SPX's small bundled Latin font. Keeping
// this separate from project font paths makes the reserved default family
// independent of project-provided CJK fonts.
const defaultDisplayFontPath = "res://engine/fonts/default.ttf"

type RuntimeFontFace struct {
	Path   string
	Family string
}

// RuntimeFontPlan is the complete project font configuration after asset
// paths have been resolved, but before it is flattened for an engine ABI.
type RuntimeFontPlan struct {
	DefaultPath string
	Faces       []RuntimeFontFace
	Preferences []string
}

func ResolveRuntimeFontPlan(fonts ProjectFonts, resolvePath func(string) string) RuntimeFontPlan {
	faceCount := 0
	for _, family := range fonts.Families {
		faceCount += len(family.Faces)
	}

	plan := RuntimeFontPlan{
		DefaultPath: defaultDisplayFontPath,
		Faces:       make([]RuntimeFontFace, 0, faceCount),
		Preferences: slices.Clone(fonts.Preferences),
	}
	for _, family := range fonts.Families {
		for _, face := range family.Faces {
			path := face.Path
			if resolvePath != nil {
				path = resolvePath(path)
			}
			plan.Faces = append(plan.Faces, RuntimeFontFace{
				Path:   path,
				Family: family.Name,
			})
		}
	}
	return plan
}

func (p RuntimeFontPlan) Clone() RuntimeFontPlan {
	p.Faces = slices.Clone(p.Faces)
	p.Preferences = slices.Clone(p.Preferences)
	return p
}
