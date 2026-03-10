package project

import (
	"path/filepath"

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/base/collisionutil"
	"github.com/goplus/spx/v2/internal/base/valueutil"
)

const (
	DefaultPathCellSize     = 16
	DefaultAudioMaxDistance = 2000
)

type LoadedBuilderProject struct {
	Config  Config
	Project ProjectConfig
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
		title = filepath.Base(cwd)
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
		PathCellSizeX:           valueutil.OrDefault(proj.PathCellSizeX, DefaultPathCellSize),
		PathCellSizeY:           valueutil.OrDefault(proj.PathCellSizeY, DefaultPathCellSize),
		AudioAttenuation:        valueutil.OrDefault(proj.AudioAttenuation, 0),
		AudioMaxDistance:        valueutil.OrDefault(proj.AudioMaxDistance, float64(DefaultAudioMaxDistance)),
		CollisionByPixel:        !proj.CollisionByShape && !proj.Physics,
		AutoSetCollisionLayer:   proj.AutoSetCollisionLayer == nil || *proj.AutoSetCollisionLayer,
		PixelCollisionPrecision: collisionutil.ParsePixelCollisionPrecision(proj.PixelCollisionPrecision),
		GlobalGravity:           valueutil.OrDefault(proj.GlobalGravity, 1),
		GlobalFriction:          valueutil.OrDefault(proj.GlobalFriction, 1),
		GlobalAirDrag:           valueutil.OrDefault(proj.GlobalAirDrag, 1),
	}
}
