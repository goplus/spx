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

package assets

import "github.com/goplus/spbase/mathf"

const (
	DefaultBitmapResolution = 1
	FullUVRange             = 1.0
)

// ToBitmapResolution returns a valid bitmap resolution value.
func ToBitmapResolution(v int) int {
	if v == 0 {
		return DefaultBitmapResolution
	}
	return v
}

// DefaultAtlasUV returns the full-range UV rect for non-atlas textures.
func DefaultAtlasUV() mathf.Vec4 {
	return mathf.NewVec4(0, 0, FullUVRange, FullUVRange)
}

// CalculateAtlasUV computes UV coordinates for an atlas region.
func CalculateAtlasUV(posX, posY, width, height int, imageSize mathf.Vec2) mathf.Vec4 {
	uStart := float64(posX) / imageSize.X
	vStart := float64(posY) / imageSize.Y
	uSize := float64(width) / imageSize.X
	vSize := float64(height) / imageSize.Y
	return mathf.NewVec4(uStart, vStart, uSize, vSize)
}

// ResolveImageSize returns the image size from configuration or loads it from fallback.
func ResolveImageSize(cfgWidth, cfgHeight float64, imagePath string, fallback func(string) mathf.Vec2) mathf.Vec2 {
	if cfgWidth > 0 && cfgHeight > 0 {
		return mathf.Vec2{X: cfgWidth, Y: cfgHeight}
	}
	return fallback(imagePath)
}

// FrameDescriptor stores precomputed size and atlas metadata for a costume frame.
type FrameDescriptor struct {
	Width            int
	Height           int
	BitmapResolution int
	ImageSize        mathf.Vec2
	Center           mathf.Vec2
	PosX             int
	PosY             int
	AtlasUVRect      mathf.Vec4
}

// NewSizedFrame creates a descriptor for an in-memory sized frame.
func NewSizedFrame(width, height int) FrameDescriptor {
	return FrameDescriptor{
		Width:            width,
		Height:           height,
		BitmapResolution: DefaultBitmapResolution,
		ImageSize:        mathf.NewVec2(float64(width), float64(height)),
		Center:           mathf.NewVec2(float64(width)/2, float64(height)/2),
		AtlasUVRect:      DefaultAtlasUV(),
	}
}

// NewAtlasFrame creates a descriptor for one frame inside an atlas/costume set.
func NewAtlasFrame(
	cfgWidth, cfgHeight float64,
	imagePath string,
	rectX, rectY, rectW, rectH float64,
	nx, frameIndex, bitmapResolution int,
	fallback func(string) mathf.Vec2,
) FrameDescriptor {
	imageSize := ResolveImageSize(cfgWidth, cfgHeight, imagePath, fallback)
	width := int(imageSize.X) / nx
	height := int(imageSize.Y)
	posX := frameIndex * width
	posY := 0

	if rectH != 0 {
		width = int(rectW) / nx
		height = int(rectH)
		posX = int(rectX) + frameIndex*width
		posY = int(rectY)
	}

	return FrameDescriptor{
		Width:            width,
		Height:           height,
		BitmapResolution: bitmapResolution,
		ImageSize:        imageSize,
		Center:           mathf.NewVec2(float64(width)/2, float64(height)/2),
		PosX:             posX,
		PosY:             posY,
		AtlasUVRect:      CalculateAtlasUV(posX, posY, width, height, imageSize),
	}
}

// NewStandaloneFrame creates a descriptor for a single non-atlas texture.
func NewStandaloneFrame(
	cfgWidth, cfgHeight float64,
	imagePath string,
	bitmapResolution int,
	fallback func(string) mathf.Vec2,
) FrameDescriptor {
	imageSize := ResolveImageSize(cfgWidth, cfgHeight, imagePath, fallback)
	return FrameDescriptor{
		Width:            int(imageSize.X),
		Height:           int(imageSize.Y),
		BitmapResolution: ToBitmapResolution(bitmapResolution),
		ImageSize:        imageSize,
		AtlasUVRect:      DefaultAtlasUV(),
	}
}
