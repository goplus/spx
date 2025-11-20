/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
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
	"encoding/json"
	"errors"
	"io"
	"syscall"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
)

func resourceDir(resource any) (fs spxfs.Dir, err error) {
	fs, ok := resource.(spxfs.Dir)
	if !ok {
		fs, err = spxfs.Open(resource.(string))
	}
	return
}

func loadJson(ret any, fs spxfs.Dir, file string) (err error) {
	if _, ok := fs.(spxfs.GdDir); ok {
		filePath := engine.ToAssetPath(file)
		if engine.HasFile(filePath) {
			value := engine.ReadAllText(filePath)
			return json.Unmarshal([]byte(value), ret)
		}
		return errors.New("error : Load json failed,file not exit " + filePath)
	}

	f, err := fs.Open(file)
	if err != nil {
		println("Error: failed to open file", file, err)
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}

func loadProjConfig(proj *projConfig, fs spxfs.Dir, index any) (err error) {
	switch v := index.(type) {
	case io.Reader:
		err = json.NewDecoder(v).Decode(proj)
	case string:
		err = loadJson(&proj, fs, v)
	case nil:
		err = loadJson(&proj, fs, "index.json")
	default:
		return syscall.EINVAL
	}
	return
}

// -------------------------------------------------------------------------------------

type Config struct {
	Title              string `json:"title,omitempty"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	KeyDuration        int    `json:"keyDuration,omitempty"`
	ScreenshotKey      string `json:"screenshotKey,omitempty"` // screenshot image capture key
	Index              any    `json:"-"`                       // where is index.json, can be file (string) or io.Reader
	DontParseFlags     bool   `json:"-"`
	FullScreen         bool   `json:"fullScreen,omitempty"`
	DontRunOnUnfocused bool   `json:"pauseOnUnfocused,omitempty"`
}

type cameraConfig struct {
	On string `json:"on"`
}

type mapConfig struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mode   string `json:"mode"`
}

const (
	mapModeFill = iota
	mapModeRepeat
	mapModeFillRatio
	mapModeFillCut
)

func toMapMode(mode string) int {
	switch mode {
	case "repeat":
		return mapModeRepeat
	case "fillCut":
		return mapModeFillCut
	case "fillRatio":
		return mapModeFillRatio
	}
	return mapModeFill
}

type projConfig struct {
	Zorder        []any             `json:"zorder"`
	Backdrops     []*backdropConfig `json:"backdrops"`
	BackdropIndex *int              `json:"backdropIndex"`
	Map           mapConfig         `json:"map"`
	Camera        *cameraConfig     `json:"camera"`
	Run           *Config           `json:"run"`
	Debug         bool              `json:"debug"`
	Bgm           string            `json:"bgm"`

	// deprecated properties
	Scenes              []*backdropConfig `json:"scenes"`              //this property is deprecated, use Backdrops instead
	Costumes            []*backdropConfig `json:"costumes"`            //this property is deprecated, use Backdrops instead
	CurrentCostumeIndex *int              `json:"currentCostumeIndex"` //this property is deprecated, use BackdropIndex instead
	SceneIndex          int               `json:"sceneIndex"`          //this property is deprecated, use BackdropIndex instead

	StretchMode *bool   `json:"stretchMode"` // whether to use stretch mode, default true
	WindowScale float64 `json:"windowScale"`

	AutoSetCollisionLayer *bool `json:"autoSetCollisionLayer"` // whether to auto set collision layer, default true
	CollisionByShape      bool  `json:"collisionByShape"`      // whether to use collision by shape or pixel, default false
	FullScreen            bool  `json:"fullscreen"`            // whether to use fullscreen, default false

	Physics        bool     `json:"physics"`        // Enable physics mode, default false, compatible with Scratch
	GlobalGravity  *float64 `json:"globalGravity"`  // Global gravity scaling factor, default 1
	GlobalFriction *float64 `json:"globalFriction"` // Global friction scaling factor, default 1
	GlobalAirDrag  *float64 `json:"globalAirDrag"`  // Global air drag scaling factor, default 1

	PathCellSizeX *int `json:"pathCellSizeX"` // Path finding cell width, default 16
	PathCellSizeY *int `json:"pathCellSizeY"` // Path finding cell height, default 16
	// audio volume scale = Math::pow(1.0f - dist / audioMaxDistance, audioAttenuation);
	AudioMaxDistance *float64 `json:"audioMaxDistance"` // default 2000
	AudioAttenuation *float64 `json:"audioAttenuation"` // default 0 indicates no attenuation will occur

	TilemapPath   string `json:"tilemapPath"`
	LayerSortMode string `json:"layerSortMode"` // layer sort method, default "" , options: "vertical"
}

func (p *projConfig) getBackdrops() []*backdropConfig {
	if p.Backdrops != nil {
		return p.Backdrops
	}
	if p.Scenes != nil {
		return p.Scenes
	}
	return p.Costumes
}

func (p *projConfig) getBackdropIndex() int {
	if p.BackdropIndex != nil {
		return *p.BackdropIndex
	}
	if p.CurrentCostumeIndex != nil {
		return *p.CurrentCostumeIndex
	}
	return p.SceneIndex
}

// -------------------------------------------------------------------------------------

type costumeSetRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type costumeSetItem struct {
	NamePrefix string `json:"namePrefix"`
	N          int    `json:"n"`
}

type costumeSet struct {
	Path             string           `json:"path"`
	ImageWidth       float64          `json:"imageWidth"`
	ImageHeight      float64          `json:"imageHeight"`
	FaceRight        float64          `json:"faceRight"` // turn face to right
	BitmapResolution int              `json:"bitmapResolution"`
	Nx               int              `json:"nx"`
	Rect             *costumeSetRect  `json:"rect"`
	Items            []costumeSetItem `json:"items"`
}

type costumeSetPart struct {
	Nx    int              `json:"nx"`
	Rect  costumeSetRect   `json:"rect"`
	Items []costumeSetItem `json:"items"`
}

type costumeMPSet struct {
	Path             string           `json:"path"`
	FaceRight        float64          `json:"faceRight"` // turn face to right
	BitmapResolution int              `json:"bitmapResolution"`
	Parts            []costumeSetPart `json:"parts"`
}

type costumeConfig struct {
	Name             string  `json:"name"`
	Path             string  `json:"path"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	ImageWidth       float64 `json:"imageWidth"`
	ImageHeight      float64 `json:"imageHeight"`
	FaceRight        float64 `json:"faceRight"` // turn face to right
	BitmapResolution int     `json:"bitmapResolution"`
}
type backdropConfig struct {
	costumeConfig
}

// -------------------------------------------------------------------------------------

// frame aniConfig
type aniTypeEnum int8

const (
	aniTypeFrame aniTypeEnum = iota
	aniTypeMove
	aniTypeTurn
	aniTypeGlide
)

type costumesConfig struct {
	From any `json:"from"`
	To   any `json:"to"`
}

type actionConfig struct {
	Play     string          `json:"play"`     //play sound
	Costumes *costumesConfig `json:"costumes"` //play frame
}

type aniConfig struct {
	FrameFrom      any     `json:"frameFrom"`
	FrameTo        any     `json:"frameTo"`
	FrameFps       int     `json:"frameFps"`
	StepDuration   float64 `json:"stepDuration"`
	TurnToDuration float64 `json:"turnToDuration"`

	AniType      aniTypeEnum   `json:"anitype"`
	OnStart      *actionConfig `json:"onStart"` //start
	OnPlay       *actionConfig `json:"onPlay"`  //play
	IsLoop       bool          `json:"isLoop"`
	IsKeepOnStop bool          `json:"isKeepOnStop"` //After finishing playback, it stays on the last frame and does not need to switch to the default animation
	Duration     float64

	// runtime
	IFrameFrom int
	IFrameTo   int

	Speed float64
	From  any
	To    any
	//OnEnd *actionConfig  `json:"onEnd"`   //stop
}

// -------------------------------------------------------------------------------------

type spriteConfig struct {
	Heading             float64               `json:"heading"`
	X                   float64               `json:"x"`
	Y                   float64               `json:"y"`
	Size                float64               `json:"size"`
	RotationStyle       string                `json:"rotationStyle"`
	Costumes            []*costumeConfig      `json:"costumes"`
	CostumeSet          *costumeSet           `json:"costumeSet"`
	CostumeMPSet        *costumeMPSet         `json:"costumeMPSet"`
	CurrentCostumeIndex *int                  `json:"currentCostumeIndex"`
	CostumeIndex        int                   `json:"costumeIndex"`
	FAnimations         map[string]*aniConfig `json:"fAnimations"`
	MAnimations         map[string]*aniConfig `json:"mAnimations"`
	TAnimations         map[string]*aniConfig `json:"tAnimations"`
	Visible             bool                  `json:"visible"`
	IsDraggable         bool                  `json:"isDraggable"`
	Pivot               mathf.Vec2            `json:"pivot"`
	DefaultAnimation    string                `json:"defaultAnimation"`
	AnimBindings        map[string]string     `json:"animBindings"`
	// ColliderShapeParams defines the shape parameters based on ColliderShapeType:
	// - "Rect": [width, height] - Rectangle with specified width and height
	// - "Circle": [radius] - Circle with specified radius
	// - "Capsule": [radius, height] - Capsule with specified radius and height
	// - "Polygon": [x1, y1, x2, y2, ...] - Polygon with vertex coordinates in pairs
	CollisionShapeParams []float64  `json:"collisionShapeParams"`
	CollisionMask        *int64     `json:"collisionMask"`
	CollisionLayer       *int64     `json:"collisionLayer"`
	CollisionShapeType   string     `json:"collisionShapeType"`
	CollisionPivot       mathf.Vec2 `json:"collisionPivot"`

	// TriggerShapeParams defines the shape parameters based on TriggerShapeType:
	// - "Rect": [width, height] - Rectangle with specified width and height
	// - "Circle": [radius] - Circle with specified radius
	// - "Capsule": [radius, height] - Capsule with specified radius and height
	// - "Polygon": [x1, y1, x2, y2, ...] - Polygon with vertex coordinates in pairs
	TriggerShapeParams []float64  `json:"triggerShapeParams"`
	TriggerMask        *int64     `json:"triggerMask"`
	TriggerLayer       *int64     `json:"triggerLayer"`
	TriggerShapeType   string     `json:"triggerShapeType"`
	TriggerPivot       mathf.Vec2 `json:"triggerPivot"`

	// physic
	PhysicsMode string   `json:"physicsMode"`
	Mass        *float64 `json:"mass"`
	Friction    *float64 `json:"friction"`
	AirDrag     *float64 `json:"airDrag"`
	Gravity     *float64 `json:"gravity"`
}

func (p *spriteConfig) getCostumeIndex() int {
	if p.CurrentCostumeIndex != nil { // for backward compatibility
		return *p.CurrentCostumeIndex
	}
	return p.CostumeIndex
}

// -------------------------------------------------------------------------------------

type soundConfig struct {
	Path        string `json:"path"`
	Rate        int    `json:"rate"`
	SampleCount int    `json:"sampleCount"`
}

// -------------------------------------------------------------------------------------
