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

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	itime "github.com/goplus/spx/v2/internal/time"
)

type frameCapture struct {
	name  string
	check bool
}

type frameTask struct {
	call    func()
	capture frameCapture
}

type decoratorContext struct {
	frame    int64
	capture  frameCapture
	consumed bool
}

var decoratorContextState struct {
	sync.Mutex
	stack []*decoratorContext
}

// Frame schedules a decorated SPX event body to run on the requested engine
// frame. XGo resolves @frame to this function from the spx framework package.
func Frame(i int, fn func()) {
	if fn == nil {
		return
	}
	ctx := &decoratorContext{frame: int64(i)}
	withDecoratorContext(ctx, fn)
}

// Capture runs the decorated event body and asks the browser-side E2E bridge to
// save the rendered frame under name.
func Capture(name string, fn func()) {
	capture(name, false, fn)
}

// CaptureAndCheck runs the decorated event body and asks the browser-side E2E
// bridge to compare the rendered frame with the named baseline.
func CaptureAndCheck(name string, fn func()) {
	capture(name, true, fn)
}

func capture(name string, check bool, fn func()) {
	if fn == nil {
		return
	}
	if ctx := currentDecoratorContext(); ctx != nil {
		prev := ctx.capture
		ctx.capture = frameCapture{name: name, check: check}
		fn()
		if !ctx.consumed {
			ctx.capture = prev
		}
		return
	}
	fn()
	mustCaptureFrame(frameCapture{name: name, check: check})
}

func withDecoratorContext(ctx *decoratorContext, fn func()) {
	decoratorContextState.Lock()
	decoratorContextState.stack = append(decoratorContextState.stack, ctx)
	decoratorContextState.Unlock()

	defer func() {
		decoratorContextState.Lock()
		stack := decoratorContextState.stack
		if len(stack) > 0 {
			decoratorContextState.stack = stack[:len(stack)-1]
		}
		decoratorContextState.Unlock()
	}()
	fn()
}

func currentDecoratorContext() *decoratorContext {
	decoratorContextState.Lock()
	defer decoratorContextState.Unlock()
	stack := decoratorContextState.stack
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}

func consumeFrameDecoratorContext(call func()) bool {
	ctx := currentDecoratorContext()
	if ctx == nil || ctx.frame == 0 || call == nil {
		return false
	}
	ctx.consumed = true
	scheduleDecoratedFrameTask(ctx.frame, frameTask{
		call:    call,
		capture: ctx.capture,
	})
	return true
}

func scheduleDecoratedFrameTask(frame int64, task frameTask) {
	if task.call == nil {
		return
	}
	if frame <= itime.Frame() {
		task.call()
		if task.capture.name != "" {
			mustCaptureFrame(task.capture)
		}
		return
	}
	if g := activeGame(); g != nil {
		g.scheduleFrameTask(frame, task)
		return
	}
	gco.CreateAndStart(false, "frame", func(coroutine.Thread) int {
		for itime.Frame() < frame {
			engine.WaitNextFrame()
		}
		task.call()
		if task.capture.name != "" {
			mustCaptureFrame(task.capture)
		}
		return 0
	})
}

func (p *Game) scheduleFrameTask(frame int64, task frameTask) {
	p.frameTasksMu.Lock()
	if p.frameTasks == nil {
		p.frameTasks = make(map[int64][]frameTask)
	}
	p.frameTasks[frame] = append(p.frameTasks[frame], task)
	p.frameTasksMu.Unlock()
}

func (p *Game) runFrameTasks() {
	frame := itime.Frame()
	p.frameTasksMu.Lock()
	var tasks []frameTask
	for target, targetTasks := range p.frameTasks {
		if target <= frame {
			tasks = append(tasks, targetTasks...)
			delete(p.frameTasks, target)
		}
	}
	p.frameTasksMu.Unlock()

	for _, task := range tasks {
		task.call()
		if task.capture.name != "" {
			p.pendingFrameCapture = append(p.pendingFrameCapture, task.capture)
		}
	}
}

func (p *Game) runPendingFrameCaptures() {
	captures := p.pendingFrameCapture
	p.pendingFrameCapture = nil
	for _, capture := range captures {
		mustCaptureFrame(capture)
	}
}

func (p *Game) resetFrameTasks() {
	p.frameTasksMu.Lock()
	p.frameTasks = nil
	p.frameTasksMu.Unlock()
	p.pendingFrameCapture = nil
}

func mustCaptureFrame(capture frameCapture) {
	if err := captureFramePlatform(capture.name, capture.check); err != nil {
		engine.Panic(err)
	}
}
