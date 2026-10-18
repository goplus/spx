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

	p.statsMu.Lock()
	p.lastUpdateStats = *stats
	p.statsMu.Unlock()
}

func elapsedMillis(start stime.Time) float64 {
	return stime.Since(start).Seconds() * 1000
}
