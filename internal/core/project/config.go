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
	"github.com/goplus/spbase/mathf"
)

type Config struct {
	Title              string `json:"title,omitempty"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	KeyDuration        int    `json:"keyDuration,omitempty"`
	ScreenshotKey      string `json:"screenshotKey,omitempty"`
	EventQueuePolicy   string `json:"eventQueuePolicy,omitempty"`
	Index              any    `json:"-"`
	DontParseFlags     bool   `json:"-"`
	FullScreen         bool   `json:"fullScreen,omitempty"`
	DontRunOnUnfocused bool   `json:"pauseOnUnfocused,omitempty"`
}

type CameraConfig struct {
	On string `json:"on"`
}

type MapConfig struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mode   string `json:"mode"`
}

const (
	MapModeFill = iota
	MapModeRepeat
	MapModeFillRatio
	MapModeFillCut
	MapModeActualSize
)

func ToMapMode(mode string) int {
	switch mode {
	case "repeat":
		return MapModeRepeat
	case "actualSize":
		return MapModeActualSize
	case "fillCut":
		return MapModeFillCut
	case "fillRatio":
		return MapModeFillRatio
	}
	return MapModeFill
}

type ProjectConfig struct {
	Zorder        []any             `json:"zorder"`
	Backdrops     []*BackdropConfig `json:"backdrops"`
	BackdropIndex int               `json:"backdropIndex"`
	Map           MapConfig         `json:"map"`
	Camera        *CameraConfig     `json:"camera"`
	Run           *Config           `json:"run"`
	Debug         bool              `json:"debug"`
	Bgm           string            `json:"bgm"`
	// FontPreferences is nil when the project does not declare a preference,
	// which is distinct from an explicitly empty preference string.
	FontPreferences *string `json:"fontPreferences,omitempty"`

	StretchMode *bool   `json:"stretchMode"`
	WindowScale float64 `json:"windowScale"`
	MaxFPS      int     `json:"maxFPS"`

	AutoSetCollisionLayer *bool `json:"autoSetCollisionLayer"`
	CollisionByShape      bool  `json:"collisionByShape"`
	FullScreen            bool  `json:"fullscreen"`

	Physics        bool     `json:"physics"`
	GlobalGravity  *float64 `json:"globalGravity"`
	GlobalFriction *float64 `json:"globalFriction"`
	GlobalAirDrag  *float64 `json:"globalAirDrag"`

	PathCellSizeX *int `json:"pathCellSizeX"`
	PathCellSizeY *int `json:"pathCellSizeY"`

	AudioMaxDistance *float64 `json:"audioMaxDistance"`
	AudioAttenuation *float64 `json:"audioAttenuation"`

	TilemapPath   string `json:"tilemapPath"`
	LayerSortMode string `json:"layerSortMode"`

	PixelCollisionPrecision *string `json:"pixelCollisionPrecision"`
}

func (p *ProjectConfig) GetBackdrops() []*BackdropConfig {
	return p.Backdrops
}

func (p *ProjectConfig) GetBackdropIndex() int {
	return p.BackdropIndex
}

type CostumeSetRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type CostumeSetItem struct {
	NamePrefix string `json:"namePrefix"`
	N          int    `json:"n"`
}

type CostumeSet struct {
	Path             string           `json:"path"`
	ImageWidth       float64          `json:"imageWidth"`
	ImageHeight      float64          `json:"imageHeight"`
	FaceRight        float64          `json:"faceRight"`
	BitmapResolution int              `json:"bitmapResolution"`
	Nx               int              `json:"nx"`
	Rect             *CostumeSetRect  `json:"rect"`
	Items            []CostumeSetItem `json:"items"`
}

type CostumeSetPart struct {
	Nx    int              `json:"nx"`
	Rect  CostumeSetRect   `json:"rect"`
	Items []CostumeSetItem `json:"items"`
}

type CostumeMPSet struct {
	Path             string           `json:"path"`
	FaceRight        float64          `json:"faceRight"`
	BitmapResolution int              `json:"bitmapResolution"`
	Parts            []CostumeSetPart `json:"parts"`
}

type CostumeConfig struct {
	Name             string  `json:"name"`
	Path             string  `json:"path"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	ImageWidth       float64 `json:"imageWidth"`
	ImageHeight      float64 `json:"imageHeight"`
	FaceRight        float64 `json:"faceRight"`
	BitmapResolution int     `json:"bitmapResolution"`
}

type BackdropConfig struct {
	CostumeConfig
	Pivot mathf.Vec2 `json:"pivot"`
}

type AniType int8

const (
	AniTypeFrame AniType = iota
	AniTypeMove
	AniTypeTurn
	AniTypeGlide
)

type CostumesConfig struct {
	From any `json:"from"`
	To   any `json:"to"`
}

type ActionConfig struct {
	Play     string          `json:"play"`
	Costumes *CostumesConfig `json:"costumes"`
}

type AniConfig struct {
	FrameFrom      any     `json:"frameFrom"`
	FrameTo        any     `json:"frameTo"`
	FrameFps       int     `json:"frameFps"`
	StepDuration   float64 `json:"stepDuration"`
	TurnToDuration float64 `json:"turnToDuration"`

	AniType      AniType       `json:"anitype"`
	OnStart      *ActionConfig `json:"onStart"`
	OnPlay       *ActionConfig `json:"onPlay"`
	IsLoop       bool          `json:"isLoop"`
	IsKeepOnStop bool          `json:"isKeepOnStop"`
	Duration     float64

	IFrameFrom int
	IFrameTo   int

	AdaptAnimBitmapResolution int

	Speed float64
	From  any
	To    any
}

type SpriteConfig struct {
	Heading          float64               `json:"heading"`
	X                float64               `json:"x"`
	Y                float64               `json:"y"`
	Size             float64               `json:"size"`
	RotationStyle    string                `json:"rotationStyle"`
	Costumes         []*CostumeConfig      `json:"costumes"`
	CostumeSet       *CostumeSet           `json:"costumeSet"`
	CostumeMPSet     *CostumeMPSet         `json:"costumeMPSet"`
	CostumeIndex     int                   `json:"costumeIndex"`
	FAnimations      map[string]*AniConfig `json:"fAnimations"`
	MAnimations      map[string]*AniConfig `json:"mAnimations"`
	TAnimations      map[string]*AniConfig `json:"tAnimations"`
	Visible          bool                  `json:"visible"`
	IsDraggable      bool                  `json:"isDraggable"`
	Pivot            mathf.Vec2            `json:"pivot"`
	DefaultAnimation string                `json:"defaultAnimation"`
	AnimBindings     map[string]string     `json:"animBindings"`

	CollisionShapeParams []float64  `json:"collisionShapeParams"`
	CollisionMask        *int64     `json:"collisionMask"`
	CollisionLayer       *int64     `json:"collisionLayer"`
	CollisionShapeType   string     `json:"collisionShapeType"`
	CollisionPivot       mathf.Vec2 `json:"collisionPivot"`

	TriggerShapeParams []float64  `json:"triggerShapeParams"`
	TriggerMask        *int64     `json:"triggerMask"`
	TriggerLayer       *int64     `json:"triggerLayer"`
	TriggerShapeType   string     `json:"triggerShapeType"`
	TriggerPivot       mathf.Vec2 `json:"triggerPivot"`

	PhysicsMode string   `json:"physicsMode"`
	Mass        *float64 `json:"mass"`
	Friction    *float64 `json:"friction"`
	AirDrag     *float64 `json:"airDrag"`
	Gravity     *float64 `json:"gravity"`
}

func (p *SpriteConfig) GetCostumeIndex() int {
	return p.CostumeIndex
}

type SoundConfig struct {
	Path        string `json:"path"`
	Rate        int    `json:"rate"`
	SampleCount int    `json:"sampleCount"`
}

type FontFaceConfig struct {
	Path string `json:"path"`
}

type FontFamilyConfig struct {
	Faces []FontFaceConfig `json:"faces"`
}
