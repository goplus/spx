package project

import (
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/base/defaults"
)

type DisplaySettings struct {
	WindowScale float64
	StretchMode bool
	Debug       bool
}

func ResolveDisplaySettings(proj *ProjectConfig) DisplaySettings {
	windowScale := 1.0
	if proj.WindowScale >= 0.001 {
		windowScale = proj.WindowScale
	}
	return DisplaySettings{
		WindowScale: windowScale,
		StretchMode: proj.StretchMode == nil || *proj.StretchMode,
		Debug:       proj.Debug,
	}
}

func ResolveMapConfig(cfg MapConfig, hasTilemap bool, defaultWidth, defaultHeight int) MapConfig {
	if hasTilemap {
		defaults.SetDefaultIfZero(&cfg.Width, defaultWidth)
		defaults.SetDefaultIfZero(&cfg.Height, defaultHeight)
	}
	return cfg
}

type WorldWindowMetrics struct {
	WorldWidth   int
	WorldHeight  int
	MinWorldX    int
	MinWorldY    int
	MapMode      int
	WindowWidth  int
	WindowHeight int
}

func ResolveWorldWindowMetrics(worldWidth, worldHeight, windowWidth, windowHeight int, mapMode int) WorldWindowMetrics {
	return WorldWindowMetrics{
		WorldWidth:   worldWidth,
		WorldHeight:  worldHeight,
		MinWorldX:    -worldWidth / 2,
		MinWorldY:    -worldHeight / 2,
		MapMode:      mapMode,
		WindowWidth:  minInt(windowWidth, worldWidth),
		WindowHeight: minInt(windowHeight, worldHeight),
	}
}

type PlatformLayoutInput struct {
	WindowWidth       int
	WindowHeight      int
	WindowScale       float64
	Fullscreen        bool
	IsMobile          bool
	IsWeb             bool
	CurrentWindowSize mathf.Vec2
}

type PlatformLayout struct {
	WindowScale  float64
	WindowWidth  int64
	WindowHeight int64
	Fullscreen   bool
}

func ResolvePlatformLayout(in PlatformLayoutInput) PlatformLayout {
	scale := in.WindowScale
	fullscreen := false

	if in.IsMobile || in.Fullscreen || in.IsWeb {
		if in.Fullscreen || in.IsMobile {
			fullscreen = true
		}
		scaleX := in.CurrentWindowSize.X / float64(in.WindowWidth)
		scaleY := in.CurrentWindowSize.Y / float64(in.WindowHeight)
		scale = math.Min(scaleX, scaleY)
	}

	winWidth := int64(float64(in.WindowWidth) * scale)
	winHeight := int64(float64(in.WindowHeight) * scale)
	if in.IsWeb {
		winWidth = int64(in.CurrentWindowSize.X)
		winHeight = int64(in.CurrentWindowSize.Y)
	}

	return PlatformLayout{
		WindowScale:  scale,
		WindowWidth:  winWidth,
		WindowHeight: winHeight,
		Fullscreen:   fullscreen,
	}
}

func IsWindowWorldSizeEqual(worldWidth, worldHeight, windowWidth, windowHeight int) bool {
	return worldHeight == windowHeight && worldWidth == windowWidth
}

type BackdropLayout struct {
	ScaleX      float64
	ScaleY      float64
	RepeatScale *mathf.Vec4
}

func ResolveBackdropLayout(imgW, imgH, dstW, dstH float64, mapMode int) BackdropLayout {
	imgRatio := imgW / imgH
	worldRatio := dstW / dstH
	isScaleHeight := imgRatio > worldRatio

	var repeatScale *mathf.Vec4
	switch mapMode {
	case MapModeRepeat:
		repeatScale = &mathf.Vec4{
			X: dstW / imgW,
			Y: dstH / imgH,
		}
	case MapModeFillCut:
		if isScaleHeight {
			dstH = dstW / imgRatio
		} else {
			dstW = dstH * imgRatio
		}
	case MapModeFillRatio:
		if isScaleHeight {
			dstW = dstH * imgRatio
		} else {
			dstH = dstW / imgRatio
		}
	}

	return BackdropLayout{
		ScaleX:      dstW / imgW,
		ScaleY:      dstH / imgH,
		RepeatScale: repeatScale,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
