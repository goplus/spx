/*
 * Copyright (c) 2024 The GoPlus Authors (goplus.org). All rights reserved.
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

	spxfs "github.com/goplus/spx/fs"
)

func loadJson(ret interface{}, fs spxfs.Dir, file string) (err error) {
	f, err := fs.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}

// -------------------------------------------------------------------------------------

type Config struct {
	Title              string      `json:"title,omitempty"`
	Width              int         `json:"width,omitempty"`
	Height             int         `json:"height,omitempty"`
	KeyDuration        int         `json:"keyDuration,omitempty"`
	ScreenshotKey      string      `json:"screenshotKey,omitempty"` // screenshot image capture key
	Index              interface{} `json:"-"`                       // where is index.json, can be file (string) or io.Reader
	DontParseFlags     bool        `json:"-"`
	FullScreen         bool        `json:"fullScreen,omitempty"`
	DontRunOnUnfocused bool        `json:"pauseOnUnfocused,omitempty"`
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
	Zorder              []interface{}    `json:"zorder"`
	Scenes              []*costumeConfig `json:"scenes"`
	Costumes            []*costumeConfig `json:"costumes"`
	CurrentCostumeIndex *int             `json:"currentCostumeIndex"`
	SceneIndex          int              `json:"sceneIndex"`

	Map    mapConfig     `json:"map"`
	Camera *cameraConfig `json:"camera"`

	Run *Config `json:"run"`
}

func (p *projConfig) getScenes() []*costumeConfig {
	if p.Scenes != nil {
		return p.Scenes
	}
	return p.Costumes
}

func (p *projConfig) getSceneIndex() int {
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
	FaceRight        float64 `json:"faceRight"` // turn face to right
	BitmapResolution int     `json:"bitmapResolution"`
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
	From interface{} `json:"from"`
	To   interface{} `json:"to"`
}

type actionConfig struct {
	Play     string          `json:"play"`     //play sound
	Costumes *costumesConfig `json:"costumes"` //play frame
}

type aniConfig struct {
	Duration float64       `json:"duration"`
	Fps      float64       `json:"fps"`
	From     interface{}   `json:"from"`
	To       interface{}   `json:"to"`
	AniType  aniTypeEnum   `json:"anitype"`
	OnStart  *actionConfig `json:"onStart"` //start
	OnPlay   *actionConfig `json:"onPlay"`  //play
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
