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

	"github.com/goplus/spx/v4/internal/time"
)

// Scratch budgets 75% of the default stepping interval for script rounds.
//
// 这不是“目标帧率”，而是一次 Update 中允许脚本连续推进的时间上限。
// 例如默认 30 FPS 时，一个引擎帧约为 33.3ms，这里的预算约为 25ms；
// 剩余时间留给渲染、引擎事件和其他收尾工作。超过预算后，循环必须回到
// 原来的调度路径，把剩余工作留到后续 Update，避免脚本独占整个引擎帧。
const loopWorkBudget = stime.Second * 3 / (4 * time.DefaultFPS)

// ScriptRound counts additional script rounds within engine frames.
// Pair it with the frame number to identify a scheduling round.
func (p *Coroutines) ScriptRound() uint64 {
	return p.scriptRound.Load()
}

// WaitNextFrame suspends the current coroutine until the next frame.
func (p *Coroutines) WaitNextFrame() {
	p.WaitNextFrameFor(p.callerThread())
}

// WaitNextFrameFor suspends me until the next frame.
func (p *Coroutines) WaitNextFrameFor(me Thread) {
	p.yieldAtFrame(me, waitFrame)
}

// YieldLoopFor yields until the next eligible script round.
//
// SPX 生成的 Repeat/Forever 等循环会在每轮末尾调用这里。原有流程会把
// 当前脚本包装成 waitLoop 任务，放入 currentJobs，然后执行一次完整的：
//
//	入队 -> 挂起当前协程 -> Update 取任务 -> 唤醒协程 -> 重新取得执行权
//
// 这个流程是必要的通用兜底，但在“只有当前脚本在运行、同一帧仍有预算、
// 没有重绘请求”的情况下，完整调度不会产生实际工作。此时只推进
// scriptRound，让循环继续执行，省掉协程挂起/唤醒和队列操作。
func (p *Coroutines) YieldLoopFor(me Thread) {
	p.requireCurrent(me)
	profile := p.perfDebug.Load()
	if profile {
		p.loopYieldCount.Add(1)
	}
	if p.canInlineLoop(me) {
		// 仍然推进逻辑脚本轮次。scriptRound 不能省略，否则依赖“下一轮”
		// 的脚本会观察到错误的轮次；这里省略的只是没有其他脚本可服务
		// 时的调度器往返。
		p.scriptRound.Add(1)
		if profile {
			p.loopInlineCount.Add(1)
		}
		return
	}
	if profile {
		p.loopScheduledCount.Add(1)
	}
	p.yieldAtFrame(me, waitLoop)
}

// canInlineLoop 判断当前循环检查点能否在当前 Go 协程中直接继续。
//
// 这个判断故意比较保守：任何一个条件不满足就走原有 enqueueAndYield
// 路径。这样优化只减少“确定没有调度工作”的成本，不改变多脚本之间的
// 公平性、取消语义、重绘时机或帧预算。
func (p *Coroutines) canInlineLoop(me Thread) bool {
	// 被取消脚本必须回到原路径，让取消清理和 ErrAbortThread 按既有流程
	// 发生；stopAtNextYield 也要求下一次让出时立即停止，不能被跳过。
	if me.canceled() || me.stopAtNextYield.Load() || p.loopWaiters.Load() != 0 {
		return false
	}

	now := p.loopNow()
	frame := time.Frame()
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()

	work := p.loopWork
	// loopWork 只在 Update 期间有效，并且必须属于当前引擎帧。到达预算
	// 截止时间、请求重绘、当前脚本不再拥有调度许可，都会强制走让出路径。
	if !work.active || work.frame != frame || !now.Before(work.deadline) ||
		p.redrawFrame.Load() == frame || p.selectedThread != me || p.Current() != me {
		return false
	}
	// pendingExecution 只有当前脚本一个条目，且 readyQueue 为空，表示没有
	// 已经准备好运行的其他脚本。loopWaiters 的第二次检查用于覆盖加锁前后
	// 其他协程刚好发布 waitLoop 的竞争窗口。
	if _, pending := p.pendingExecution[me]; !pending || len(p.pendingExecution) != 1 ||
		!p.readyQueue.empty() || p.loopWaiters.Load() != 0 {
		return false
	}
	return !p.hasEligiblePeerJobLocked(me, work.frame, work.levelTime)
}

// hasEligiblePeerJobLocked 检查是否有需要当前脚本真正让出执行权的工作。
//
// currentJobs 中不只有“可以马上运行”的任务：Wait(1) 之类的任务可能还
// 没到时间，WaitNextFrame 也可能还没跨帧。因此这里按任务类型判断是否
// 已经具备执行资格，只有具备资格的其他脚本才会阻止当前脚本内联继续。
// 调用方持有 schedulerMu，保证检查期间队列状态不变。
func (p *Coroutines) hasEligiblePeerJobLocked(me Thread, frame int64, levelTime float64) bool {
	return p.currentJobs.any(func(job *waitJob) bool {
		if job.kind == waitMainThread {
			return true
		}
		if job.thread == me || job.thread.canceled() {
			return false
		}
		switch job.kind {
		case waitYield, waitLoop, waitNextRound:
			return true
		case waitFrame:
			return job.frame < frame
		case waitTime:
			return job.frame < frame && job.deadline <= levelTime
		default:
			return false
		}
	})
}

// YieldToNextRoundFor waits for the next frame or a round admitted by a loop continuation.
func (p *Coroutines) YieldToNextRoundFor(me Thread) {
	p.yieldAtFrame(me, waitNextRound)
}

// RequestRedraw ends additional script rounds after the current round finishes.
// 内联循环也必须检查这个标记，否则脚本请求重绘后仍可能在同一帧继续
// 扩大逻辑工作量，破坏“本轮结束后立即回到渲染”的语义。
func (p *Coroutines) RequestRedraw() {
	p.redrawFrame.Store(time.Frame())
}

func (p *Coroutines) yieldAtFrame(me Thread, kind waitKind) {
	p.enqueueAndYield(&waitJob{thread: me, kind: kind, frame: time.Frame()})
}

// Admit a new round only after all runnable scripts have yielded.
// 即使某个脚本通过 canInlineLoop 在本协程内连续推进，scriptRound 仍会
// 递增；当它最终走到这里时，roundJobs 依旧按原有规则统一转成 waitYield，
// 保证下一轮只在所有可运行脚本完成让出后开始。
func (p *Coroutines) queueNextRound(state *updateState) bool {
	if p.redrawFrame.Load() == state.frame ||
		!p.loopNow().Before(state.workDeadline) || !p.hasLoopContinuation() {
		return false
	}
	// A script round does not advance the frame clock.
	p.scriptRound.Add(1)
	for n := p.roundJobs.head; n != nil; n = n.next {
		n.value.kind = waitYield
	}
	p.schedulerMu.Lock()
	p.currentJobs.appendFrom(p.roundJobs)
	p.schedulerMu.Unlock()
	return true
}

func (p *Coroutines) hasLoopContinuation() bool {
	return p.roundJobs.any(func(job *waitJob) bool {
		return job.kind == waitLoop && !job.thread.canceled()
	})
}
