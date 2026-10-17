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

// StartBatch registers tasks before running them in order.
// BatchWaitFirstSlice requires a managed caller for non-empty batches.
// BatchWaitDone yields managed callers and blocks other callers.
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

	threads, progress := p.registerBatch(tasks)
	relayBatchProgress(threads, progress[1:])
	progress[0].Open()
	switch mode {
	case BatchWaitFirstSlice:
		progress[len(tasks)].Wait()
	case BatchWaitDone:
		p.JoinAll(threads)
	}
	return threads
}

func (p *Coroutines) registerBatch(tasks []Task) ([]Thread, []*Latch) {
	progress := make([]*Latch, len(tasks)+1)
	for i := range progress {
		progress[i] = p.NewLatch()
	}
	threads := make([]Thread, len(tasks))
	batchCreated := false
	defer func() {
		if !batchCreated {
			for _, thread := range threads {
				p.Stop(thread)
			}
		}
	}()
	admission := p.captureAdmission()
	for i, task := range tasks {
		current, next := progress[i], progress[i+1]
		if registerInvocation := task.RegisterInvocation; registerInvocation != nil {
			// Publish each slot before its registration callback.
			task.RegisterInvocation = func(thread Thread) func() {
				threads[i] = thread
				return registerInvocation(thread)
			}
		}
		run := task.Run
		task.Run = func(thread Thread) {
			defer next.Open()
			current.Wait()
			next.Open()
			run(thread)
		}
		threads[i] = p.createThread(admission, task)
	}
	batchCreated = true
	return threads, progress
}

// Advance canceled tasks in order so later tasks cannot bypass an unfinished prefix.
func relayBatchProgress(threads []Thread, progress []*Latch) {
	go func() {
		for i, thread := range threads {
			select {
			case <-progress[i].Done():
			case <-thread.Context().Done():
				progress[i].Open()
			}
		}
	}()
}
