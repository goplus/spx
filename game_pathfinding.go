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
	"github.com/goplus/spx/v2/internal/base/valueutil"
	"github.com/goplus/spx/v2/internal/engine"
)

// -----------------------------------------------------------------------------
// Path Finding System

const (
	defaultPathCellSize = 16 // default path finding cell size
)

// setupPathFinderConfig initializes path finder configuration from project settings.
func (p *Game) setupPathFinderConfig(proj *projConfig) {
	p.pathCellSizeX = valueutil.OrDefault(proj.PathCellSizeX, defaultPathCellSize)
	p.pathCellSizeY = valueutil.OrDefault(proj.PathCellSizeY, defaultPathCellSize)
}

func (p *Game) SetupPathFinder__0() {
	p.setupPathFinder(true, false)
}

func (p *Game) SetupPathFinder__1(x_grid_size, y_grid_size, x_cell_size, y_cell_size float64, with_jump, with_debug bool) {
	p.engine().NavigationMgr.SetupPathFinderWithSize(mathf.NewVec2(x_grid_size, y_grid_size), mathf.NewVec2(x_cell_size, y_cell_size), with_jump, with_debug)
}

func (p *Game) setupPathFinder(with_jump, with_debug bool) {
	cellSize := mathf.NewVec2(float64(p.pathCellSizeX), float64(p.pathCellSizeY))
	gridSize := mathf.NewVec2(float64(p.worldWidth), float64(p.worldHeight)).Div(cellSize)
	p.engine().NavigationMgr.SetupPathFinderWithSize(gridSize, cellSize, with_jump, with_debug)
}

func (p *Game) setObstacle(sprite Sprite, enabled bool) {
	impl := spriteOf(sprite)
	if impl != nil {
		p.engine().NavigationMgr.SetObstacle(impl.getSpriteId(), enabled)
	}
}

func (p *Game) FindPath__0(x_from, y_from, x_to, y_to float64) []float64 {
	return p.FindPath__2(x_from, y_from, x_to, y_to, false, true)
}

func (p *Game) FindPath__1(x_from, y_from, x_to, y_to float64, with_debug bool) []float64 {
	return p.FindPath__2(x_from, y_from, x_to, y_to, with_debug, true)
}

func (p *Game) FindPath__2(x_from, y_from, x_to, y_to float64, with_debug, with_jump bool) []float64 {
	p.oncePathFinder.Do(func() {
		p.setupPathFinder(with_jump, with_debug)
	})

	arr := p.engine().NavigationMgr.FindPath(mathf.NewVec2(x_from, y_from), mathf.NewVec2(x_to, y_to), with_jump)
	result := arr.([]float32)
	return engine.F32Tof64(result)
}
