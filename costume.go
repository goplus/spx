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
	"path"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v2/internal/assets"
	"github.com/goplus/spx/v2/internal/engine"
)

// costumeSetImage represents metadata for a costume set image.
type costumeSetImage struct {
	path   string
	rc     costumeSetRect
	width  float64
	height float64
	nx     int // number of frames in the image
}

// costume represents a single costume (image frame) for a sprite or backdrop.
type costume struct {
	name          SpriteCostumeName
	width, height int
	center        mathf.Vec2 // center point
	imageSize     mathf.Vec2 // actual image dimensions

	faceRight        float64
	bitmapResolution int
	path             string

	setIndex   int // costume index in set (-1 if not part of a set)
	posX, posY int // position in atlas (left top corner)

	atlasUVRect mathf.Vec4 // UV coordinates for atlas texture
}

// newCostumeWithSize creates a costume with specified dimensions (no image file).
func newCostumeWithSize(width, height int) *costume {
	frame := assetutil.NewSizedFrame(width, height)
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
		getImageSizeCached,
	)
	return &costume{
		path:             img.path,
		name:             name,
		setIndex:         frameIndex,
		faceRight:        faceRight,
		bitmapResolution: frame.BitmapResolution,
		imageSize:        frame.ImageSize,
		width:            frame.Width,
		height:           frame.Height,
		posX:             frame.PosX,
		posY:             frame.PosY,
		atlasUVRect:      frame.AtlasUVRect,
		center:           frame.Center,
	}
}

// newCostume creates a costume from a costume configuration.
func newCostume(base string, config *costumeConfig) *costume {
	fullPath := path.Join(base, config.Path)
	frame := assetutil.NewStandaloneFrame(
		config.ImageWidth,
		config.ImageHeight,
		fullPath,
		config.BitmapResolution,
		getImageSizeCached,
	)
	return &costume{
		name:             config.Name,
		setIndex:         -1,
		center:           mathf.Vec2{X: config.X, Y: config.Y},
		faceRight:        config.FaceRight,
		bitmapResolution: frame.BitmapResolution,
		path:             fullPath,
		imageSize:        frame.ImageSize,
		width:            frame.Width,
		height:           frame.Height,
		posX:             frame.PosX,
		posY:             frame.PosY,
		atlasUVRect:      frame.AtlasUVRect,
	}
}

// getImageSizeCached retrieves image size from cache or loads it.
func getImageSizeCached(imagePath string) mathf.Vec2 {
	cache := imageSizeCacheRef()
	if v, ok := cache.Load(imagePath); ok {
		return v.(mathf.Vec2)
	}
	size := getCostumeAssetSize(imagePath)
	cache.Store(imagePath, size)
	return size
}

// getCostumeAssetSize loads the actual image size from the asset.
func getCostumeAssetSize(imagePath string) mathf.Vec2 {
	assetPath := engine.ToAssetPath(imagePath)
	if game, ok := engine.GetGame().(*Game); ok && game != nil {
		return game.engine().ResMgr.GetImageSize(assetPath)
	}
	var engineMgr engineManagers
	return engineMgr.ResMgr.GetImageSize(assetPath)
}

// getSize returns the size of the costume accounting for bitmap resolution.
func (c *costume) getSize() (int, int) {
	return c.width / c.bitmapResolution, c.height / c.bitmapResolution
}

// isAtlas returns true if this costume is part of an atlas/set.
func (c *costume) isAtlas() bool {
	return c.setIndex >= 0
}
