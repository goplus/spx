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
	"fmt"

	"github.com/goplus/spx/v3/internal/engine/profiler"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/internal/ui"
)

func (p *Game) showDebugPanel() {
	profiler.Debug = p.debugState.Debug
	if !p.debugState.Debug {
		if p.debugState.DebugPanel != nil {
			p.debugState.DebugPanel.Destroy()
			p.debugState.DebugPanel = nil
		}
		return
	}
	if p.debugState.DebugPanel == nil {
		p.debugState.DebugPanel = ui.NewUiDebug()
	}

	updateInfo, _ := profiler.GetStats("GameUpdate")
	coroInfo, _ := profiler.GetStats("CoroUpdateJobs")
	renderInfo, _ := profiler.GetStats("GameRender")
	lastInfo := gco.UpdateStats()

	msg := fmt.Sprintf("FPS: %.f\n", itime.FPS())
	msg += fmt.Sprintf("Shape: %v\n", p.shapeMgr.count())
	msg += fmt.Sprintf("GameUpdate: %v\n", updateInfo.ActualCall)
	msg += fmt.Sprintf("GameRender: %v\n", renderInfo.ActualCall)
	msg += fmt.Sprintf("CoroUpdateJobs: %v\n", coroInfo.ActualCall)
	msg += fmt.Sprintf("coro: MoveTime: %.2f\n", lastInfo.MoveTime)
	msg += fmt.Sprintf("coro: WaitTime: %.2f\n", lastInfo.WaitTime)
	msg += fmt.Sprintf("coro: TaskProcessing: %.2f\n", lastInfo.JobTime)
	msg += fmt.Sprintf("coro: TaskCounts: %v\n", lastInfo.JobCount)
	msg += fmt.Sprintf("coro: WaitFrameCount: %v\n", lastInfo.FrameWaitCount)
	msg += fmt.Sprintf("coro: WaitMainCount: %v\n", lastInfo.MainThreadCount)
	msg += fmt.Sprintf("coro: NextCount: %v\n", lastInfo.DeferredCount)
	msg += fmt.Sprintf("coro: GCCount: %v\n", lastInfo.GCCount)
	msg += fmt.Sprintf("coro: LoopIterations: %v\n", lastInfo.LoopIterations)
	p.debugState.DebugPanel.Show(msg)
}
