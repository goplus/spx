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
	"github.com/goplus/spx/v4/internal/base/numeric"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

type gamePathfindingState struct {
	once          sync.Once
	PathCellSizeX int
	PathCellSizeY int
}

func (p *Game) SetupPathFinder__0() {
	p.setupPathFinder(true, false)
}

func (p *Game) SetupPathFinder__1(xGridSize, yGridSize, xCellSize, yCellSize float64, withJump, withDebug bool) {
	engine.Managers().NavigationMgr.SetupPathFinderWithSize(
		mathf.NewVec2(xGridSize, yGridSize),
		mathf.NewVec2(xCellSize, yCellSize),
		withJump,
		withDebug,
	)
}

func (p *Game) FindPath__0(fromX, fromY, toX, toY float64) []float64 {
	return p.FindPath__2(fromX, fromY, toX, toY, false, true)
}

func (p *Game) FindPath__1(fromX, fromY, toX, toY float64, withDebug bool) []float64 {
	return p.FindPath__2(fromX, fromY, toX, toY, withDebug, true)
}

func (p *Game) FindPath__2(fromX, fromY, toX, toY float64, withDebug, withJump bool) []float64 {
	p.pathfindingState.once.Do(func() {
		p.setupPathFinder(withJump, withDebug)
	})

	arr := engine.Managers().NavigationMgr.FindPath(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), withJump)
	result := arr.([]float32)
	return numeric.ConvertFloatSlice[float64](result)
}

// Path Finding.
func (p *Game) applyPathFinderSettings(settings coreproject.SystemSettings) {
	p.pathfindingState.PathCellSizeX = settings.PathCellSizeX
	p.pathfindingState.PathCellSizeY = settings.PathCellSizeY
}

func (p *Game) setupPathFinder(withJump, withDebug bool) {
	cellSize := mathf.NewVec2(float64(p.pathfindingState.PathCellSizeX), float64(p.pathfindingState.PathCellSizeY))
	gridSize := mathf.NewVec2(float64(p.displayState.WorldWidth), float64(p.displayState.WorldHeight)).Div(cellSize)
	engine.Managers().NavigationMgr.SetupPathFinderWithSize(gridSize, cellSize, withJump, withDebug)
}
