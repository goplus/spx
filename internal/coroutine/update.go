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

	"github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/time"
)

type updateState struct {
	frame         int64
	levelTime     float64
	watchdogStart float64
}

// Update processes queued wait jobs and resumes eligible coroutines.
func (p *Coroutines) Update() {
	start := stime.Now()
	gcStatsBefore := p.readGCStatsBeforeUpdate()

	stats, state := p.beginUpdate()
	stats.GCStatsEnabled = gcStatsBefore != nil
	p.runUpdateLoop(&stats, &state)
	p.finishUpdate(&stats, start, gcStatsBefore)
	p.statsMu.Lock()
	p.lastUpdateStats = stats
	p.statsMu.Unlock()
}

func (p *Coroutines) readGCStatsBeforeUpdate() *sdebug.GCStats {
	if !p.perfDebug.Load() {
		return nil
	}
	stats := &sdebug.GCStats{}
	p.readGCStats(stats)
	return stats
}

func (p *Coroutines) beginUpdate() (UpdateJobsStats, updateState) {
	start := stime.Now()
	state := updateState{
		frame:         time.Frame(),
		levelTime:     time.TimeSinceLevelLoad(),
		watchdogStart: time.RealTimeSinceStart(),
	}
	return UpdateJobsStats{InitTime: elapsedMillis(start)}, state
}

func (p *Coroutines) runUpdateLoop(stats *UpdateJobsStats, state *updateState) {
	start := stime.Now()
	iterations := 0
	for {
		iterations++
		done, retry := p.waitForWork(stats)
		if done {
			break
		}
		if retry {
			continue
		}

		p.processNextWaitJob(state, stats)
		if time.RealTimeSinceStart()-state.watchdogStart > 1 {
			log.Warn("engine update exceeded 1 second - please review your code (waitMainCount=%d)", stats.WaitMainCount)
			break
		}
	}

	stats.LoopTime = elapsedMillis(start)
	stats.LoopIterations = iterations
	p.promoteDeferredJobs(stats)
}

func (p *Coroutines) waitForWork(stats *UpdateJobsStats) (done, retry bool) {
	if !p.hasInited.Load() {
		if p.currentJobs.Count() == 0 {
			start := stime.Now()
			time.Sleep(0.05)
			stats.WaitTime += elapsedMillis(start)
			return false, true
		}
		return false, false
	}

	start := stime.Now()
	p.schedulerMu.Lock()
	if p.currentJobs.Count() == 0 {
		if p.runnableThreadCountLocked() == 0 {
			done = true
		} else {
			p.schedulerCond.Wait()
			retry = true
		}
	}
	p.schedulerMu.Unlock()
	stats.WaitTime += elapsedMillis(start)
	return done, retry
}

func (p *Coroutines) processNextWaitJob(state *updateState, stats *UpdateJobsStats) {
	start := stime.Now()
	job := p.currentJobs.PopFront()
	stats.TaskCounts++
	p.processWaitJob(state, stats, job)
	stats.TaskProcessing += elapsedMillis(start)
}

func (p *Coroutines) processWaitJob(state *updateState, stats *UpdateJobsStats, job *WaitJob) {
	switch job.Type {
	case waitTypeFrame:
		if job.Frame >= state.frame {
			p.deferredJobs.PushBack(job)
		} else {
			job.Call()
			stats.WaitFrameCount++
		}
	case waitTypeTime:
		if job.Time >= state.levelTime {
			p.deferredJobs.PushBack(job)
		} else {
			job.Call()
		}
	case waitTypeYield:
		job.Call()
	case waitTypeMainThread:
		job.Call()
		stats.WaitMainCount++
	}
}

func (p *Coroutines) promoteDeferredJobs(stats *UpdateJobsStats) {
	start := stime.Now()
	stats.NextCount = p.deferredJobs.Count()
	p.currentJobs.Move(p.deferredJobs)
	stats.MoveTime = elapsedMillis(start)
}

func (p *Coroutines) finishUpdate(stats *UpdateJobsStats, start stime.Time, before *sdebug.GCStats) {
	if before != nil {
		var after sdebug.GCStats
		p.readGCStats(&after)
		stats.GCCount = int(after.NumGC - before.NumGC)
		stats.GCPauses = float64(after.PauseTotal-before.PauseTotal) / float64(stime.Millisecond)
	}

	total := elapsedMillis(start)
	accounted := stats.InitTime + stats.LoopTime + stats.MoveTime
	stats.TotalTime = total
	stats.ExternalTime = total - accounted
	stats.TimeDifference = total - accounted
}

func elapsedMillis(start stime.Time) float64 {
	return stime.Since(start).Seconds() * 1000
}
