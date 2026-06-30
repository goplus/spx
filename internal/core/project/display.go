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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/base/defaults"
)

type DisplaySettings struct {
	WindowScale              float64
	StretchMode              bool
	Debug                    bool
	DefaultFontPath          string
	SVGFontFaceRegistrations []SVGFontFaceRegistration
}

const defaultDisplayFontPath = "res://engine/fonts/CnFont.ttf"
const defaultSVGFontFamily = "SPX Default"
const symbolsSVGFontFamily = "Symbols"
const emojiSVGFontFamily = "Emoji"

type SVGFontFaceRegistration struct {
	Path   string
	Family string
}

var scratchSVGFontRegistrations = []SVGFontFaceRegistration{
	{Path: "res://engine/fonts/scratch/NotoSans-Medium.ttf", Family: "Sans Serif"},
	{Path: "res://engine/fonts/scratch/SourceSerifPro-Regular.otf", Family: "Serif"},
	{Path: "res://engine/fonts/scratch/handlee-regular.ttf", Family: "Handwriting"},
	{Path: "res://engine/fonts/scratch/Knewave.ttf", Family: "Marker"},
	{Path: "res://engine/fonts/scratch/Griffy-Regular.ttf", Family: "Curly"},
	{Path: "res://engine/fonts/scratch/Grand9K-Pixel.ttf", Family: "Pixel"},
	{Path: "res://engine/fonts/scratch/Scratch.ttf", Family: "Scratch"},
	{Path: "res://engine/fonts/symbols/NotoSansSymbols2-Regular.ttf", Family: symbolsSVGFontFamily},
	{Path: "res://engine/fonts/emoji/TwitterColorEmoji-SVGinOT.ttf", Family: emojiSVGFontFamily},
	{Path: defaultDisplayFontPath, Family: defaultSVGFontFamily},
}

func ResolveDisplaySettings(proj *ProjectConfig) DisplaySettings {
	if proj == nil {
		proj = &ProjectConfig{}
	}
	windowScale := 1.0
	if proj.WindowScale >= 0.001 {
		windowScale = proj.WindowScale
	}
	return DisplaySettings{
		WindowScale:              windowScale,
		StretchMode:              proj.StretchMode == nil || *proj.StretchMode,
		Debug:                    proj.Debug,
		DefaultFontPath:          defaultDisplayFontPath,
		SVGFontFaceRegistrations: append([]SVGFontFaceRegistration(nil), scratchSVGFontRegistrations...),
	}
}

func RegisterDisplayFonts(
	settings DisplaySettings,
	setDefaultFont func(string),
	registerSVGFontFace func(string, string),
) {
	if setDefaultFont != nil && settings.DefaultFontPath != "" {
		setDefaultFont(settings.DefaultFontPath)
	}
	if registerSVGFontFace == nil {
		return
	}
	// lunasvg resolves one face per family name, so duplicate family registrations
	// replace each other instead of forming a fallback chain. Keep the last
	// registration so callers can override the built-in defaults.
	lastIndex := make(map[string]int, len(settings.SVGFontFaceRegistrations))
	for i, font := range settings.SVGFontFaceRegistrations {
		if font.Path == "" || font.Family == "" {
			continue
		}
		lastIndex[font.Family] = i
	}
	for i, font := range settings.SVGFontFaceRegistrations {
		if font.Path == "" || font.Family == "" {
			continue
		}
		if lastIndex[font.Family] != i {
			continue
		}
		registerSVGFontFace(font.Path, font.Family)
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
	case MapModeActualSize:
		dstW = imgW
		dstH = imgH
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
