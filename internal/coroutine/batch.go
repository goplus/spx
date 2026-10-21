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

// BatchMode controls how long StartBatch waits.
type BatchMode uint8

const (
	_ BatchMode = iota
	BatchAsync
	BatchWaitFirstSlice
	BatchWaitDone
)

// StartBatch registers all tasks before publishing them in script order.
// BatchWaitFirstSlice requires a managed caller for non-empty batches.
// BatchWaitDone waits for completion; managed callers resume in script order.
// RegisterInvocation may cancel work but must not wait for its task.
func (p *Coroutines) StartBatch(tasks []Task, mode BatchMode) []Thread {
	if mode != BatchAsync && mode != BatchWaitFirstSlice && mode != BatchWaitDone {
		panic("coroutine: invalid batch mode")
	}
	if len(tasks) == 0 {
		return nil
	}
	if mode == BatchWaitFirstSlice && p.callerThread() == nil {
		panic("coroutine: FirstSlice requires a managed caller")
	}

	threads := p.registerBatch(tasks)
	p.publishThreads(threads)
	switch mode {
	case BatchWaitFirstSlice:
		for _, thread := range threads {
			p.JoinFirstSlice(thread)
		}
	case BatchWaitDone:
		p.JoinAll(threads)
	}
	return threads
}

func (p *Coroutines) registerBatch(tasks []Task) []Thread {
	threads := make([]Thread, len(tasks))
	registered := false
	defer func() {
		if !registered {
			for _, thread := range threads {
				p.Stop(thread)
			}
		}
	}()
	admission := p.captureAdmission()
	for i, task := range tasks {
		if registerInvocation := task.RegisterInvocation; registerInvocation != nil {
			// Record the task before its callback so panic cleanup can cancel it.
			task.RegisterInvocation = func(thread Thread) func() {
				threads[i] = thread
				return registerInvocation(thread)
			}
		}
		threads[i] = p.registerTask(admission, task)
	}
	registered = true
	return threads
}
