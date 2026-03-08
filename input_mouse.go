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
	"time"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	inputstate "github.com/goplus/spx/v2/internal/input"
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

const (
	// Minimum interval between two mouse click events.
	mouseClickInterval = 50 * time.Millisecond
)

// inputManager handles runtime input state and delegates specialized logic to helpers.
type inputManager struct {
	g        *Game
	mousePos mathf.Vec2

	clickGate inputstate.ClickGate
	swipe     coreruntime.SwipeState[*SpriteImpl]
}

func (p *inputManager) init(g *Game) {
	p.mousePos = mathf.Vec2{}
	p.g = g
	p.clickGate.Init(mouseClickInterval)
	p.swipe.Init()
}

func (p *inputManager) currentMousePos() mathf.Vec2 {
	return p.mousePos
}

func (p *inputManager) setMousePos(pos mathf.Vec2) {
	p.mousePos = pos
}

func (p *inputManager) canTriggerClickEvent(id engine.Object) bool {
	return p.clickGate.Allow(id)
}
