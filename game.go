/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"io"
)

type Game struct {
	Base
	fs FileSystem

	width  int
	height int
}

type FileSystem interface {
	Open(file string) (io.ReadCloser, error)
	Close() error
}

type SwitchAction int

const (
	Prev SwitchAction = -1
	Next SwitchAction = 1
)

// -----------------------------------------------------------------------------

func (p *Game) getWidth() int {
	if p.width == 0 {
		p.doSize()
	}
	return p.width
}

func (p *Game) size() (int, int) {
	if p.width == 0 {
		p.doSize()
	}
	return p.width, p.height
}

func (p *Game) doSize() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.width == 0 {
		c := p.costumes[p.currentCostumeIndex]
		img, _, _ := c.needImage(p.fs)
		w, h := img.Size()
		p.width, p.height = w/c.bitmapResolution, h/c.bitmapResolution
	}
}

// -----------------------------------------------------------------------------

func (p *Game) SceneName() string {
	return p.costumeName()
}

func (p *Game) SceneIndex() int {
	return p.costumeIndex()
}

// StartScene func:
//   StartScene(sceneName) or
//   StartScene(sceneIndex) or
//   StartScene(spx.Next)
//   StartScene(spx.Prev)
func (p *Game) StartScene(scene interface{}, wait ...bool) {
	if p.setCostume(scene) {
		// TODO: send event & wait
	}
}

func (p *Game) NextScene(wait ...bool) {
	p.StartScene(Next, wait...)
}

// -----------------------------------------------------------------------------
