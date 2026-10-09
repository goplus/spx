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
	assetutil "github.com/goplus/spx/v3/internal/assets"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

var costumeSizeCache sync.Map

// costumeSetImage represents metadata for a costume set image.
type costumeSetImage struct {
	path   string
	rc     coreproject.CostumeSetRect
	width  float64
	height float64
	nx     int // number of frames in the image
}

// costume represents a single costume (image frame) for a sprite or backdrop.
type costume struct {
	name          SpriteCostumeName
	width, height int
	center        mathf.Vec2 // center point
	imageSize     mathf.Vec2 // standalone logical size or atlas image pixel size
	pivot         mathf.Vec2

	faceRight        float64
	bitmapResolution int
	path             string

	setIndex   int // costume index in set (-1 if not part of a set)
	posX, posY int // position in atlas (left top corner)

	atlasUVRect mathf.Vec4 // UV coordinates for atlas texture
}

func (c *costume) getAssetPath() string {
	return costumeAssetPath(c.path)
}

// displaySize returns integer dimensions for the stage and window.
func (c *costume) displaySize() (int, int) {
	return c.width / c.bitmapResolution, c.height / c.bitmapResolution
}

// sizeInSPX returns the frame size in SPX coordinates, retaining fractional pixels.
func (c *costume) sizeInSPX() (float64, float64) {
	resolution := float64(c.bitmapResolution)
	size := c.frameSizeInAsset()
	return size.X / resolution, size.Y / resolution
}

// renderAnchorInSPX converts the costume center from top-left, Y-down asset
// coordinates into a local SPX anchor relative to the geometric image center.
func (c *costume) renderAnchorInSPX() mathf.Vec2 {
	size := c.frameSizeInAsset()
	resolution := float64(c.bitmapResolution)
	return mathf.NewVec2(
		(c.center.X-size.X/2)/resolution,
		(size.Y/2-c.center.Y)/resolution,
	)
}

// frameSizeInAsset returns standalone logical dimensions or atlas frame pixel dimensions.
func (c *costume) frameSizeInAsset() mathf.Vec2 {
	if !c.isAtlas() && c.imageSize.X > 0 && c.imageSize.Y > 0 {
		return c.imageSize
	}
	return mathf.NewVec2(float64(c.width), float64(c.height))
}

// isAtlas returns true if this costume is part of an atlas/set.
func (c *costume) isAtlas() bool {
	return c.setIndex >= 0
}

// newCostumeWithSize creates a costume with specified dimensions (no image file).
func newCostumeWithSize(width, height int) *costume {
	return newCostumeFromFrame(assetutil.NewSizedFrame(width, height))
}

func newCostumeFromFrame(frame assetutil.FrameDescriptor) *costume {
	return &costume{
		setIndex:         -1,
		width:            frame.Width,
		height:           frame.Height,
		bitmapResolution: frame.BitmapResolution,
		posX:             frame.PosX,
		posY:             frame.PosY,
		imageSize:        frame.ImageSize,
		center:           frame.Center,
		atlasUVRect:      frame.AtlasUVRect,
	}
}

// newCostumeWith creates a costume from a costume set image.
func newCostumeWith(name string, img *costumeSetImage, faceRight float64, frameIndex, bitmapResolution int) *costume {
	frame := assetutil.NewAtlasFrame(
		img.width,
		img.height,
		img.path,
		img.rc.X,
		img.rc.Y,
		img.rc.W,
		img.rc.H,
		img.nx,
		frameIndex,
		bitmapResolution,
		costumePixelSize,
	)
	c := newCostumeFromFrame(frame)
	c.path = img.path
	c.name = name
	c.setIndex = frameIndex
	c.faceRight = faceRight
	return c
}

// newCostume creates a costume from a costume configuration.
func newCostume(config *coreproject.CostumeConfig) *costume {
	frame := assetutil.NewStandaloneFrame(
		config.ImageWidth,
		config.ImageHeight,
		config.Path,
		config.BitmapResolution,
		costumeLogicalSize,
	)
	c := newCostumeFromFrame(frame)
	c.name = config.Name
	c.center = mathf.Vec2{X: config.X, Y: config.Y}
	c.faceRight = config.FaceRight
	c.path = config.Path
	return c
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

type costumeSizeKey struct {
	path    string
	logical bool
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
