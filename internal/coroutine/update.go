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
	"cmp"
	stime "time"

	"github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
)

type waitKind int

const (
	waitFrame waitKind = iota
	waitTime
	waitMainThread
	waitYield
	waitLoop
	waitNextRound
)

// waitJob holds a script wait or engine-thread call.
type waitJob struct {
	thread   Thread // Waiting coroutine; may be nil for waitMainThread.
	kind     waitKind
	call     func()  // Engine-thread action, only for waitMainThread.
	deadline float64 // Level-time deadline for a time job.
	frame    int64   // Issuing frame for frame-gated jobs.
}

type updateState struct {
	frame            int64
	levelTime        float64
	watchdogDeadline stime.Time
	workDeadline     stime.Time
}

type updateAction uint8

const (
	updateProcessJob updateAction = iota
	updateRetry
	updateComplete
)

const updateWatchdogTimeout = stime.Second

// Update processes queued wait jobs and resumes eligible coroutines.
// The engine must serialize Update calls.
func (p *Coroutines) Update() {
	start := stime.Now()
	gcStatsBefore := p.captureGCStats()

	stats, state := p.beginUpdate()
	stats.GCStatsEnabled = gcStatsBefore != nil
	p.runUpdateLoop(&stats, &state)
	p.finishStats(&stats, start, gcStatsBefore)
}

func (p *Coroutines) beginUpdate() (UpdateStats, updateState) {
	start := stime.Now()
	state := updateState{
		frame:            itime.Frame(),
		levelTime:        itime.TimeSinceLevelLoad(),
		watchdogDeadline: p.updateWatchdogNow().Add(updateWatchdogTimeout),
		workDeadline:     start.Add(loopWorkBudget),
	}
	return UpdateStats{InitTime: elapsedMillis(start)}, state
}

func (p *Coroutines) runUpdateLoop(stats *UpdateStats, state *updateState) {
	start := stime.Now()
	iterations := 0

updateLoop:
	for {
		iterations++
		action, job := p.nextUpdateAction(stats)
		switch action {
		case updateComplete:
			if p.queueNextRound(state) {
				continue
			}
			break updateLoop
		case updateProcessJob:
			start := stime.Now()
			stats.JobCount++
			p.processJob(state, stats, job)
			stats.JobTime += elapsedMillis(start)
		case updateRetry:
		}

		if !p.updateWatchdogNow().Before(state.watchdogDeadline) {
			// A slow frame is not necessarily a runaway script. Leave queued work
			// for the next Update without canceling scripts or waiting for cleanup.
			log.Warn("engine update exceeded 1 second - deferring remaining work to next frame (waitMainCount=%d)", stats.MainThreadCount)
			break updateLoop
		}
	}

	stats.LoopTime = elapsedMillis(start)
	stats.LoopIterations = iterations
	p.promoteDeferredJobs(stats)
}

func (p *Coroutines) nextUpdateAction(stats *UpdateStats) (updateAction, *waitJob) {
	start := stime.Now()
	action := updateProcessJob
	p.schedulerMu.Lock()
	job, queued := p.currentJobs.peekFront()
	// Service engine calls before waiting for scripts, including startup, to yield.
	if (!queued || job.kind != waitMainThread) && p.hasRunnableLocked() {
		p.schedulerCond.Wait()
		action = updateRetry
	} else if !queued {
		action = updateComplete
	} else {
		// Engine waiters may also take jobs, so select and remove under one lock.
		job = p.currentJobs.popFront()
	}
	p.schedulerMu.Unlock()
	stats.WaitTime += elapsedMillis(start)
	return action, job
}

func (p *Coroutines) processJob(state *updateState, stats *UpdateStats, job *waitJob) {
	if job.thread.canceled() {
		return
	}

	switch job.kind {
	case waitLoop, waitNextRound:
		if job.frame < state.frame {
			p.wakeThread(job.thread)
		} else {
			p.roundJobs.pushBack(job)
		}
	case waitFrame:
		if job.frame >= state.frame {
			p.deferredJobs.pushBack(job)
		} else {
			p.wakeThread(job.thread)
			stats.FrameWaitCount++
		}
	case waitTime:
		if job.frame >= state.frame || job.deadline > state.levelTime {
			p.deferredJobs.pushBack(job)
		} else {
			p.wakeThread(job.thread)
		}
	case waitYield:
		p.wakeThread(job.thread)
	case waitMainThread:
		job.call()
		stats.MainThreadCount++
	}
}

func (p *Coroutines) promoteDeferredJobs(stats *UpdateStats) {
	start := stime.Now()
	p.deferredJobs.appendFrom(p.roundJobs)
	// Deferred waits retain script order, including restarted handlers.
	p.deferredJobs.sortStable(func(a, b *waitJob) int {
		return cmp.Compare(a.thread.resumeOrder, b.thread.resumeOrder)
	})
	stats.DeferredCount = p.deferredJobs.len()
	p.schedulerMu.Lock()
	p.currentJobs.appendFrom(p.deferredJobs)
	p.schedulerMu.Unlock()
	stats.MoveTime = elapsedMillis(start)
}
