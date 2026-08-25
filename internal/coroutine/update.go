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
	itime "github.com/goplus/spx/v3/internal/time"
)

type updateState struct {
	frame            int64
	levelTime        float64
	watchdogDeadline stime.Time
}

type updateAction uint8

const (
	updateProcessJob updateAction = iota
	updateRetry
	updateComplete
	updateAwaitInitialization

	updateWatchdogTimeout = stime.Second
)

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
		frame:            itime.Frame(),
		levelTime:        itime.TimeSinceLevelLoad(),
		watchdogDeadline: p.updateWatchdogNow().Add(updateWatchdogTimeout),
	}
	return UpdateJobsStats{InitTime: elapsedMillis(start)}, state
}

func (p *Coroutines) runUpdateLoop(stats *UpdateJobsStats, state *updateState) {
	start := stime.Now()
	iterations := 0

updateLoop:
	for {
		iterations++
		switch p.nextUpdateAction(stats) {
		case updateComplete:
			break updateLoop
		case updateAwaitInitialization:
			state.watchdogDeadline = p.updateWatchdogNow().Add(updateWatchdogTimeout)
			continue
		case updateProcessJob:
			p.processNextWaitJob(state, stats)
		case updateRetry:
		}

		if !p.updateWatchdogNow().Before(state.watchdogDeadline) {
			log.Warn("engine update exceeded 1 second - stopping runaway scripts (waitMainCount=%d)", stats.WaitMainCount)
			p.stopRunawayThreads()
			break updateLoop
		}
	}

	stats.LoopTime = elapsedMillis(start)
	stats.LoopIterations = iterations
	p.promoteDeferredJobs(stats)
}

func (p *Coroutines) stopRunawayThreads() {
	// This remains cooperative: an active thread must release runMu first.
	p.shutdownMu.Lock()
	defer p.shutdownMu.Unlock()

	p.runMu.Lock()
	p.creationMu.Lock()
	p.stopping = true
	p.AbortAll()
	p.creationMu.Unlock()
	p.runMu.Unlock()

	for {
		p.waitForThreadsToStop(0, nil)

		// Close the race between observing an empty registry and a concurrent
		// registration. Creations during shutdown are registered as canceled.
		p.creationMu.Lock()
		if !p.hasThreadsOtherThan(nil) {
			p.stopping = false
			p.creationMu.Unlock()
			return
		}
		p.creationMu.Unlock()
	}
}

func (p *Coroutines) nextUpdateAction(stats *UpdateJobsStats) updateAction {
	if !p.hasInited.Load() {
		if p.currentJobs.Count() == 0 {
			start := stime.Now()
			itime.Sleep(0.05)
			stats.WaitTime += elapsedMillis(start)
			return updateAwaitInitialization
		}
		return updateProcessJob
	}

	start := stime.Now()
	action := updateProcessJob
	p.schedulerMu.Lock()
	if p.currentJobs.Count() == 0 {
		if p.runnableThreadCountLocked() == 0 {
			action = updateComplete
		} else {
			p.schedulerCond.Wait()
			action = updateRetry
		}
	}
	p.schedulerMu.Unlock()
	stats.WaitTime += elapsedMillis(start)
	return action
}

func (p *Coroutines) processNextWaitJob(state *updateState, stats *UpdateJobsStats) {
	start := stime.Now()
	job := p.currentJobs.PopFront()
	stats.TaskCounts++
	p.processWaitJob(state, stats, job)
	stats.TaskProcessing += elapsedMillis(start)
}

func (p *Coroutines) processWaitJob(state *updateState, stats *UpdateJobsStats, job *WaitJob) {
	if job.Th != nil && p.isThreadCanceled(job.Th) {
		return
	}

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
