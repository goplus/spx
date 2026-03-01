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
	stdtime "time"

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/engine/profiler"
	itime "github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/ui"
)

func (p *Game) showDebugPanel() {
	engine.SetDebugMode(p.debug)
	profiler.Debug = p.debug
	if !p.debug {
		if p.debugPanel != nil {
			p.debugPanel.Destroy()
			p.debugPanel = nil
		}
		return
	}
	if p.debugPanel == nil {
		p.debugPanel = ui.NewUiDebug()
	}

	updateInfo, _ := profiler.GetStats("GameUpdate")
	coroInfo, _ := profiler.GetStats("CoroUpdateJobs")
	renderInfo, _ := profiler.GetStats("GameRender")
	lastInfo := gco.GetLastUpdateStats()
	eventQ := p.eventQueueSnapshot()

	msg := fmt.Sprintf("FPS: %.f\n", itime.FPS())
	msg += fmt.Sprintf("Shape: %v\n", p.spriteMgr.count())
	msg += fmt.Sprintf("GameUpdate: %v\n", updateInfo.ActualCall)
	msg += fmt.Sprintf("GameRender: %v\n", renderInfo.ActualCall)
	msg += fmt.Sprintf("CoroUpdateJobs: %v\n", coroInfo.ActualCall)
	msg += fmt.Sprintf("coro: MoveTime: %.2f\n", lastInfo.MoveTime)
	msg += fmt.Sprintf("coro: WaitTime: %.2f\n", lastInfo.WaitTime)
	msg += fmt.Sprintf("coro: TaskProcessing: %.2f\n", lastInfo.TaskProcessing)
	msg += fmt.Sprintf("coro: TaskCounts: %v\n", lastInfo.TaskCounts)
	msg += fmt.Sprintf("coro: WaitFrameCount: %v\n", lastInfo.WaitFrameCount)
	msg += fmt.Sprintf("coro: WaitMainCount: %v\n", lastInfo.WaitMainCount)
	msg += fmt.Sprintf("coro: NextCount: %v\n", lastInfo.NextCount)
	msg += fmt.Sprintf("coro: GCCount: %v\n", lastInfo.GCCount)
	msg += fmt.Sprintf("coro: LoopIterations: %v\n", lastInfo.LoopIterations)
	msg += fmt.Sprintf("eventQ: policy=%s\n", eventQ.Policy)
	msg += fmt.Sprintf("eventQ: len/cap/max=%d/%d/%d\n", eventQ.QueueLen, eventQ.QueueCap, eventQ.MaxQueueLenSeen)
	msg += fmt.Sprintf("eventQ: enqueued=%d dropped=%d\n", eventQ.EnqueuedTotal, eventQ.DroppedTotal)
	if !eventQ.LastDropAt.IsZero() {
		msg += fmt.Sprintf("eventQ: lastDrop=%s\n", eventQ.LastDropAt.Format(stdtime.RFC3339Nano))
	}
	p.debugPanel.Show(msg)
}
