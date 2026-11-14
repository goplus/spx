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
	"sync"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

var costumeSizeCache sync.Map

// costumeSetImage represents metadata for a costume set image.
type costumeSetImage struct {
	path       string
	rect       coreproject.CostumeSetRect
	width      float64
	height     float64
	frameCount int
}

const (
	defaultBitmapResolution = 1
	fullUVRange             = 1.0
)

// costumeFrame stores precomputed size and atlas metadata for a costume frame.
type costumeFrame struct {
	Width            int
	Height           int
	BitmapResolution int
	ImageSize        mathf.Vec2 // standalone logical dimensions or full atlas image pixel dimensions
	Center           mathf.Vec2
	PosX             int
	PosY             int
	AtlasUVRect      mathf.Vec4
}

// costume represents a single costume (image frame) for a sprite or backdrop.
type costume struct {
	name      SpriteCostumeName
	path      string
	pivot     mathf.Vec2
	faceRight float64
	atlas     bool
	frame     costumeFrame
}

type costumeSizeKey struct {
	path    string
	logical bool
}

func (c *costume) getAssetPath() string {
	return costumeAssetPath(c.path)
}

// displaySize returns integer dimensions for the stage and window.
func (c *costume) displaySize() (int, int) {
	return c.frame.Width / c.frame.BitmapResolution, c.frame.Height / c.frame.BitmapResolution
}

// sizeInSPX returns the frame size in SPX coordinates, retaining fractional pixels.
func (c *costume) sizeInSPX() (float64, float64) {
	resolution := float64(c.frame.BitmapResolution)
	size := c.frameSizeInAsset()
	return size.X / resolution, size.Y / resolution
}

// renderAnchorInSPX converts the costume center from top-left, Y-down asset
// coordinates into a local SPX anchor relative to the geometric image center.
func (c *costume) renderAnchorInSPX() mathf.Vec2 {
	size := c.frameSizeInAsset()
	resolution := float64(c.frame.BitmapResolution)
	return mathf.NewVec2(
		(c.frame.Center.X-size.X/2)/resolution,
		(size.Y/2-c.frame.Center.Y)/resolution,
	)
}

// frameSizeInAsset returns standalone logical dimensions or atlas frame pixel dimensions.
func (c *costume) frameSizeInAsset() mathf.Vec2 {
	if !c.isAtlas() && c.frame.ImageSize.X > 0 && c.frame.ImageSize.Y > 0 {
		return c.frame.ImageSize
	}
	return mathf.NewVec2(float64(c.frame.Width), float64(c.frame.Height))
}

// isAtlas returns true if this costume is part of an atlas/set.
func (c *costume) isAtlas() bool {
	return c.atlas
}

// newCostumeWithSize creates a costume with specified dimensions (no image file).
func newCostumeWithSize(width, height int) *costume {
	return &costume{frame: newSizedCostumeFrame(width, height)}
}

// newCostumeFromSet creates a costume from a costume set image.
func newCostumeFromSet(name string, img *costumeSetImage, faceRight float64, frameIndex, bitmapResolution int) *costume {
	frame := newAtlasCostumeFrame(img, frameIndex, bitmapResolution, costumePixelSize)
	return &costume{name: name, path: img.path, atlas: frameIndex >= 0, faceRight: faceRight, frame: frame}
}

// newCostume creates a costume from a costume configuration.
func newCostume(config *coreproject.CostumeConfig) *costume {
	frame := newStandaloneCostumeFrame(config, costumeLogicalSize)
	frame.Center = costumeCenterInAsset(frame, mathf.NewVec2(config.X, config.Y))
	return &costume{name: config.Name, path: config.Path, faceRight: config.FaceRight, frame: frame}
}

// costumeCenterInAsset returns the rotation center to anchor a costume on. An empty
// costume is imported as a 1x1 frame but can still carry a center sized for its
// original canvas, which would displace the sprite, so pixel-less frames stay centered.
// Both dimensions must degenerate: a 1xN line costume has real pixels and may anchor
// outside its center.
func costumeCenterInAsset(frame costumeFrame, declared mathf.Vec2) mathf.Vec2 {
	if frame.Width <= 1 && frame.Height <= 1 {
		return mathf.NewVec2(float64(frame.Width)/2, float64(frame.Height)/2)
	}
	return declared
}

func newBackdropCostume(config *coreproject.BackdropConfig) *costume {
	costume := newCostume(&config.CostumeConfig)
	costume.pivot = config.Pivot
	return costume
}

func costumeAssetPath(path string) string {
	return engine.ToAssetPath(path)
}

func costumePixelSize(imagePath string) mathf.Vec2 {
	return cachedCostumeSize(imagePath, false)
}

func costumeLogicalSize(imagePath string) mathf.Vec2 {
	return cachedCostumeSize(imagePath, true)
}

func cachedCostumeSize(imagePath string, logical bool) mathf.Vec2 {
	assetPath := costumeAssetPath(imagePath)
	key := costumeSizeKey{path: assetPath, logical: logical}
	if value, ok := costumeSizeCache.Load(key); ok {
		return value.(mathf.Vec2)
	}
	var size mathf.Vec2
	if logical {
		size = engine.Managers().ResMgr.GetImageLogicalSize(assetPath)
	} else {
		size = engine.Managers().ResMgr.GetImageSize(assetPath)
	}
	costumeSizeCache.Store(key, size)
	return size
}

// normalizeBitmapResolution returns a valid bitmap resolution value.
func normalizeBitmapResolution(v int) int {
	if v == 0 {
		return defaultBitmapResolution
	}
	return v
}

// defaultAtlasUV returns the full-range UV rect for non-atlas textures.
func defaultAtlasUV() mathf.Vec4 {
	return mathf.NewVec4(0, 0, fullUVRange, fullUVRange)
}

// calculateAtlasUV computes UV coordinates for an atlas region.
func calculateAtlasUV(posX, posY, width, height int, imageSize mathf.Vec2) mathf.Vec4 {
	uStart := float64(posX) / imageSize.X
	vStart := float64(posY) / imageSize.Y
	uSize := float64(width) / imageSize.X
	vSize := float64(height) / imageSize.Y
	return mathf.NewVec4(uStart, vStart, uSize, vSize)
}

// resolveCostumeImageSize returns the image size from configuration or loads it from fallback.
func resolveCostumeImageSize(cfgWidth, cfgHeight float64, imagePath string, fallback func(string) mathf.Vec2) mathf.Vec2 {
	if cfgWidth > 0 && cfgHeight > 0 {
		return mathf.Vec2{X: cfgWidth, Y: cfgHeight}
	}
	return fallback(imagePath)
}

// newSizedCostumeFrame creates a descriptor for an in-memory sized frame.
func newSizedCostumeFrame(width, height int) costumeFrame {
	return costumeFrame{
		Width:            width,
		Height:           height,
		BitmapResolution: defaultBitmapResolution,
		ImageSize:        mathf.NewVec2(float64(width), float64(height)),
		Center:           mathf.NewVec2(float64(width)/2, float64(height)/2),
		AtlasUVRect:      defaultAtlasUV(),
	}
}

// newAtlasCostumeFrame creates a descriptor for one frame inside an atlas/costume set.
func newAtlasCostumeFrame(img *costumeSetImage, frameIndex, bitmapResolution int, fallback func(string) mathf.Vec2) costumeFrame {
	frameCount := max(img.frameCount, 1)
	imageSize := resolveCostumeImageSize(img.width, img.height, img.path, fallback)
	width := int(imageSize.X) / frameCount
	height := int(imageSize.Y)
	posX := frameIndex * width
	posY := 0

	if img.rect.H != 0 {
		width = int(img.rect.W) / frameCount
		height = int(img.rect.H)
		posX = int(img.rect.X) + frameIndex*width
		posY = int(img.rect.Y)
	}

	return costumeFrame{
		Width:            width,
		Height:           height,
		BitmapResolution: bitmapResolution,
		ImageSize:        imageSize,
		Center:           mathf.NewVec2(float64(width)/2, float64(height)/2),
		PosX:             posX,
		PosY:             posY,
		AtlasUVRect:      calculateAtlasUV(posX, posY, width, height, imageSize),
	}
}

// newStandaloneCostumeFrame creates a descriptor for a single non-atlas texture.
func newStandaloneCostumeFrame(config *coreproject.CostumeConfig, fallback func(string) mathf.Vec2) costumeFrame {
	imageSize := resolveCostumeImageSize(config.ImageWidth, config.ImageHeight, config.Path, fallback)
	return costumeFrame{
		Width:            int(imageSize.X),
		Height:           int(imageSize.Y),
		BitmapResolution: normalizeBitmapResolution(config.BitmapResolution),
		ImageSize:        imageSize,
		Center:           imageSize.Mulf(0.5),
		AtlasUVRect:      defaultAtlasUV(),
	}
}
