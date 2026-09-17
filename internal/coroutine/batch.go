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

// StartBatch registers tasks before running them in order; wait modes require a
// managed caller. Setup may cancel work but must not wait for its task.
func (p *Coroutines) StartBatch(tasks []Task, mode BatchMode) []Thread {
	if mode != BatchAsync && mode != BatchWaitFirstSlice && mode != BatchWaitDone {
		panic("coroutine: invalid batch mode")
	}
	if len(tasks) == 0 {
		return nil
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
	progress := newLatchSet(p, len(tasks)+1)
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
		if setup := task.Setup; setup != nil {
			// Publish the slot before invoking the callback, preserving the
			// registration order visible to callbacks.
			task.Setup = func(thread Thread) func() {
				threads[i] = thread
				return setup(thread)
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

func newLatchSet(p *Coroutines, size int) []*Latch {
	latches := make([]*Latch, size)
	for i := range latches {
		latches[i] = p.NewLatch()
	}
	return latches
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
