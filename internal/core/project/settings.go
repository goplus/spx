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

import (
	"path/filepath"

	spxfs "github.com/goplus/spx/v3/fs"
	"github.com/goplus/spx/v3/internal/base/collision"
	"github.com/goplus/spx/v3/internal/base/defaults"
)

const (
	DefaultPathCellSize     = 16
	DefaultAudioMaxDistance = 2000
)

type LoadedBuilderProject struct {
	Config  Config
	Project ProjectConfig
	Fonts   ProjectFonts
}

func LoadBuilderProject(fs spxfs.Dir, gameConf *Config) (LoadedBuilderProject, error) {
	var loaded LoadedBuilderProject
	var index any
	if gameConf != nil {
		loaded.Config = *gameConf
		index = loaded.Config.Index
	}

	if err := LoadConfig(&loaded.Project, fs, index); err != nil {
		return LoadedBuilderProject{}, err
	}
	normalizeProjectConfigPaths(&loaded.Project)
	fonts, err := LoadProjectFonts(fs, loaded.Project.FontPreferences)
	if err != nil {
		return LoadedBuilderProject{}, err
	}
	loaded.Fonts = fonts

	if gameConf == nil && loaded.Project.Run != nil {
		loaded.Config = *loaded.Project.Run
	}
	return loaded, nil
}

type RuntimeConfig struct {
	Title            string
	FullScreen       bool
	PhysicsEnabled   bool
	EventQueuePolicy string
	WindowWidth      int
	WindowHeight     int
	ScreenshotKey    string
}

func ResolveRuntimeConfig(conf *Config, proj *ProjectConfig, cwd string, screenshotEnv string) RuntimeConfig {
	title := conf.Title
	if title == "" {
		title = filepath.Base(cwd) + " (by XGo Builder)"
	}

	key := conf.ScreenshotKey
	if key == "" {
		key = screenshotEnv
	}

	return RuntimeConfig{
		Title:            title,
		FullScreen:       proj.FullScreen || conf.FullScreen,
		PhysicsEnabled:   proj.Physics,
		EventQueuePolicy: conf.EventQueuePolicy,
		WindowWidth:      conf.Width,
		WindowHeight:     conf.Height,
		ScreenshotKey:    key,
	}
}

type SystemSettings struct {
	LayerSortMode           string
	PathCellSizeX           int
	PathCellSizeY           int
	AudioAttenuation        float64
	AudioMaxDistance        float64
	CollisionByPixel        bool
	AutoSetCollisionLayer   bool
	PixelCollisionPrecision int64
	GlobalGravity           float64
	GlobalFriction          float64
	GlobalAirDrag           float64
}

func ResolveSystemSettings(proj *ProjectConfig) SystemSettings {
	return SystemSettings{
		LayerSortMode:           proj.LayerSortMode,
		PathCellSizeX:           defaults.OrDefault(proj.PathCellSizeX, DefaultPathCellSize),
		PathCellSizeY:           defaults.OrDefault(proj.PathCellSizeY, DefaultPathCellSize),
		AudioAttenuation:        defaults.OrDefault(proj.AudioAttenuation, 0),
		AudioMaxDistance:        defaults.OrDefault(proj.AudioMaxDistance, float64(DefaultAudioMaxDistance)),
		CollisionByPixel:        !proj.CollisionByShape && !proj.Physics,
		AutoSetCollisionLayer:   proj.AutoSetCollisionLayer == nil || *proj.AutoSetCollisionLayer,
		PixelCollisionPrecision: collision.ParsePixelCollisionPrecision(proj.PixelCollisionPrecision),
		GlobalGravity:           defaults.OrDefault(proj.GlobalGravity, 1),
		GlobalFriction:          defaults.OrDefault(proj.GlobalFriction, 1),
		GlobalAirDrag:           defaults.OrDefault(proj.GlobalAirDrag, 1),
	}
}
