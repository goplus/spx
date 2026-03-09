/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
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
	coreproject "github.com/goplus/spx/v2/internal/core/project"
)

type Config = coreproject.Config
type cameraConfig = coreproject.CameraConfig
type mapConfig = coreproject.MapConfig

const (
	mapModeFill      = coreproject.MapModeFill
	mapModeRepeat    = coreproject.MapModeRepeat
	mapModeFillRatio = coreproject.MapModeFillRatio
	mapModeFillCut   = coreproject.MapModeFillCut
)

func toMapMode(mode string) int {
	return coreproject.ToMapMode(mode)
}

type projConfig = coreproject.ProjectConfig
type costumeSetRect = coreproject.CostumeSetRect
type costumeSetItem = coreproject.CostumeSetItem
type costumeSet = coreproject.CostumeSet
type costumeSetPart = coreproject.CostumeSetPart
type costumeMPSet = coreproject.CostumeMPSet
type costumeConfig = coreproject.CostumeConfig
type backdropConfig = coreproject.BackdropConfig
type aniTypeEnum = coreproject.AniType

const (
	aniTypeFrame = coreproject.AniTypeFrame
	aniTypeMove  = coreproject.AniTypeMove
	aniTypeTurn  = coreproject.AniTypeTurn
	aniTypeGlide = coreproject.AniTypeGlide
)

type costumesConfig = coreproject.CostumesConfig
type actionConfig = coreproject.ActionConfig
type aniConfig = coreproject.AniConfig

// -----------------------------------------------------------------------------
// Sprite configuration
// -----------------------------------------------------------------------------

type spriteConfig = coreproject.SpriteConfig

// -----------------------------------------------------------------------------
// Sound configuration
// -----------------------------------------------------------------------------

type soundConfig = coreproject.SoundConfig
