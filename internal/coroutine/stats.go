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

package coroutine

import (
	sdebug "runtime/debug"
	stime "time"
)

// UpdateStats reports timings in milliseconds and job counts for one Update.
type UpdateStats struct {
	InitTime        float64 // Update preparation.
	LoopTime        float64 // Update loop.
	MoveTime        float64 // Promoting deferred jobs.
	WaitTime        float64 // Selecting jobs, including scheduler waits.
	JobTime         float64 // Processing dequeued jobs.
	GCPauseTime     float64 // GC pauses during the update.
	ExternalTime    float64 // Unaccounted time, including runtime scheduling overhead.
	TotalTime       float64 // Total update time.
	JobCount        int     // Dequeued jobs, including canceled jobs.
	FrameWaitCount  int     // Frame-wait jobs processed.
	MainThreadCount int     // Main-thread jobs processed.
	DeferredCount   int     // Jobs deferred at the end of the update loop.
	GCCount         int     // GC cycles during the update.
	GCStatsEnabled  bool    // GCCount and GCPauseTime are valid only when true.
	LoopIterations  int     // Update-loop iterations.
	// LoopYieldCount 是本次 Update 内脚本到达 YieldLoopFor 的次数。
	// 它统计循环检查点，不是调度器自身的 LoopIterations。
	LoopYieldCount int
	// LoopInlineCount 是检查点直接在当前协程继续下一轮的次数。
	LoopInlineCount int
	// LoopScheduledCount 是检查点进入 enqueueAndYield 完整调度路径的次数。
	LoopScheduledCount int
	// LoopSameFrameCount 是 waitLoop 在本次 Update 内进入 roundJobs 后，
	// 又被转换为 waitYield 并恢复的次数。
	LoopSameFrameCount int
	// LoopDeferredCount 是本次 Update 结束时仍留在 roundJobs、被推迟到
	// 后续 Update 的 waitLoop 数量，是“当前帧未完成循环续接”的直接指标。
	LoopDeferredCount int
	// LoopNextFrameCount 是本次 Update 恢复上一帧推迟的 waitLoop 数量。
	LoopNextFrameCount int
}

// SetPerfDebug enables or disables GC statistics collection during Update.
func (p *Coroutines) SetPerfDebug(enabled bool) {
	p.perfDebug.Store(enabled)
}

// UpdateStats returns this manager's most recent update statistics.
func (p *Coroutines) UpdateStats() UpdateStats {
	p.statsMu.RLock()
	stats := p.lastUpdateStats
	p.statsMu.RUnlock()
	return stats
}

func (p *Coroutines) captureGCStats() *sdebug.GCStats {
	if !p.perfDebug.Load() {
		return nil
	}
	stats := &sdebug.GCStats{}
	p.readGCStats(stats)
	return stats
}

func (p *Coroutines) finishStats(stats *UpdateStats, start stime.Time, before *sdebug.GCStats) {
	if before != nil {
		var after sdebug.GCStats
		p.readGCStats(&after)
		stats.GCCount = int(after.NumGC - before.NumGC)
		stats.GCPauseTime = float64(after.PauseTotal-before.PauseTotal) / float64(stime.Millisecond)
	}

	total := elapsedMillis(start)
	accounted := stats.InitTime + stats.LoopTime + stats.MoveTime
	stats.TotalTime = total
	stats.ExternalTime = total - accounted
	if p.perfDebug.Load() {
		stats.LoopYieldCount = int(p.loopYieldCount.Load())
		stats.LoopInlineCount = int(p.loopInlineCount.Load())
		stats.LoopScheduledCount = int(p.loopScheduledCount.Load())
		stats.LoopSameFrameCount = int(p.loopSameFrameCount.Load())
		stats.LoopDeferredCount = int(p.loopDeferredCount.Load())
		stats.LoopNextFrameCount = int(p.loopNextFrameCount.Load())
	}

	p.statsMu.Lock()
	p.lastUpdateStats = *stats
	p.statsMu.Unlock()
}

func elapsedMillis(start stime.Time) float64 {
	return stime.Since(start).Seconds() * 1000
}
