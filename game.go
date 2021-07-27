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

type Game struct {
	Base
}

type SwitchAction int

const (
	Prev SwitchAction = -1
	Next SwitchAction = 1
)

// -----------------------------------------------------------------------------

func (p *Game) SceneName() string {
	panic("todo")
}

func (p *Game) SceneIndex() int {
	panic("todo")
}

// StartScene func:
//   StartScene(sceneName) or
//   StartScene(sceneIndex) or
//   StartScene(spx.Next)
//   StartScene(spx.Prev)
func (p *Game) StartScene(scene interface{}, wait ...bool) {
	panic("todo")
}

func (p *Game) NextScene(wait ...bool) {
	panic("todo")
}

// -----------------------------------------------------------------------------
