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
	stime "time"

	"github.com/goplus/spx/v4/internal/log"
	itime "github.com/goplus/spx/v4/internal/time"
)

type waitKind int

const (
	waitFrame waitKind = iota
	waitTime
	waitYield
	waitLoop
	waitNextRound
)

// waitJob holds a script wait.
type waitJob struct {
	thread   Thread // Waiting coroutine.
	kind     waitKind
	deadline float64 // Level-time deadline for a time job.
	frame    int64   // Issuing frame for frame-gated jobs.
}

type updateState struct {
	frame            int64
	levelTime        float64
	watchdogDeadline stime.Time
	workDeadline     stime.Time
}

// waitBatch retains a wait scan's clock, boundary, and unpublished results.
// Keeping it separate from executable work lets engine dispatchers run between Updates.
type waitBatch struct {
	active       bool
	remaining    int
	frame        int64
	levelTime    float64
	runnable     *runnableState
	deferredJobs *queue[*waitJob]
	roundJobs    *queue[*waitJob]
}

type updateAction uint8

const (
	updateMainThreadCall updateAction = iota
	updateRetry
	updateComplete
)

const (
	updateWatchdogTimeout = stime.Second
	// Bound lock hold time and amortize watchdog checks across small chunks.
	waitScanChunkSize = 64
)

// Update processes queued wait jobs and resumes eligible coroutines.
// The engine must serialize Update calls.
func (p *Scheduler) Update() {
	start := stime.Now()
	gcStatsBefore := p.captureGCStats()

	stats, state := p.beginUpdate()
	stats.GCStatsEnabled = gcStatsBefore != nil
	p.runUpdateLoop(&stats, &state)
	p.finishStats(&stats, start, gcStatsBefore)
}

func (p *Scheduler) beginUpdate() (UpdateStats, updateState) {
	start := stime.Now()
	state := updateState{
		frame:            itime.Frame(),
		levelTime:        itime.TimeSinceLevelLoad(),
		watchdogDeadline: p.updateWatchdogNow().Add(updateWatchdogTimeout),
		workDeadline:     start.Add(loopWorkBudget),
	}
	return UpdateStats{InitTime: elapsedMillis(start)}, state
}

func (p *Scheduler) runUpdateLoop(stats *UpdateStats, state *updateState) {
	start := stime.Now()
	iterations := 0

updateLoop:
	for {
		iterations++
		action, pending := p.nextUpdateAction(state, stats)
		switch action {
		case updateComplete:
			if p.queueNextRound(state) {
				continue
			}
			break updateLoop
		case updateMainThreadCall:
			start := stime.Now()
			stats.JobCount++
			if !pending.caller.canceled() {
				pending.run(p)
				stats.MainThreadCount++
			}
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

func (p *Scheduler) nextUpdateAction(state *updateState, stats *UpdateStats) (updateAction, *mainThreadCall) {
	start := stime.Now()
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	// Service engine calls before waiting for scripts or publishing a batch.
	if pending := p.takeMainThreadCallLocked(); pending != nil {
		stats.WaitTime += elapsedMillis(start)
		return updateMainThreadCall, pending
	}
	if p.hasPendingExecutionLocked() {
		p.schedulerCond.Wait()
		stats.WaitTime += elapsedMillis(start)
		return updateRetry, nil
	}
	batch := &p.waitBatch
	if batch.active && batch.remaining == 0 {
		// The previous iteration checked the watchdog before publication.
		p.runnableState, batch.runnable = batch.runnable, p.runnableState
		batch.active = false
		p.scheduleLocked()
		stats.WaitTime += elapsedMillis(start)
		return updateRetry, nil
	}
	if p.currentJobs.len() == 0 {
		stats.WaitTime += elapsedMillis(start)
		return updateComplete, nil
	}
	// Account for selecting the batch before measuring its processing time.
	stats.WaitTime += elapsedMillis(start)
	p.scanWaitBatchLocked(state, stats)
	return updateRetry, nil
}

// Scan a bounded part of the batch without publishing any scripts. Each chunk
// releases schedulerMu and returns to the watchdog. Caller has no pending work.
func (p *Scheduler) scanWaitBatchLocked(state *updateState, stats *UpdateStats) {
	start := stime.Now()
	batch := &p.waitBatch
	if !batch.active {
		batch.active = true
		batch.remaining = p.currentJobs.len()
		batch.frame, batch.levelTime = state.frame, state.levelTime
		batch.runnable.readyQueue.cursor = 0
	}
	for range waitScanChunkSize {
		job := p.currentJobs.popFront()
		batch.remaining--
		stats.JobCount++
		p.stageWaitJobLocked(stats, job)
		if batch.remaining == 0 {
			p.deferredJobs.appendFrom(batch.deferredJobs)
			p.roundJobs.appendFrom(batch.roundJobs)
			break
		}
	}
	stats.JobTime += elapsedMillis(start)
}

// Classify a wait into the batch's runnable, deferred, or next-round work.
func (p *Scheduler) stageWaitJobLocked(stats *UpdateStats, job *waitJob) {
	if job.thread.canceled() {
		return
	}

	batch := &p.waitBatch
	switch job.kind {
	case waitLoop, waitNextRound:
		if job.frame >= batch.frame {
			batch.roundJobs.pushBack(job)
			return
		}
	case waitFrame:
		if job.frame >= batch.frame {
			batch.deferredJobs.pushBack(job)
			return
		}
		stats.FrameWaitCount++
	case waitTime:
		if job.frame >= batch.frame || job.deadline > batch.levelTime {
			batch.deferredJobs.pushBack(job)
			return
		}
	case waitYield:
	default:
		return
	}
	batch.runnable.markPendingLocked(job.thread)
}

func (p *Scheduler) promoteDeferredJobs(stats *UpdateStats) {
	start := stime.Now()
	p.deferredJobs.appendFrom(p.roundJobs)
	// The ready queue orders scripts after the whole wait batch is scanned.
	p.schedulerMu.Lock()
	stats.DeferredCount = p.deferredJobs.len() + p.waitBatch.deferredJobs.len() + p.waitBatch.roundJobs.len()
	p.currentJobs.appendFrom(p.deferredJobs)
	p.schedulerMu.Unlock()
	stats.MoveTime = elapsedMillis(start)
}
