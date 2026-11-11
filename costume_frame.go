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
	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

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
	nx := max(img.nx, 1)
	imageSize := resolveCostumeImageSize(img.width, img.height, img.path, fallback)
	width := int(imageSize.X) / nx
	height := int(imageSize.Y)
	posX := frameIndex * width
	posY := 0

	if img.rc.H != 0 {
		width = int(img.rc.W) / nx
		height = int(img.rc.H)
		posX = int(img.rc.X) + frameIndex*width
		posY = int(img.rc.Y)
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
