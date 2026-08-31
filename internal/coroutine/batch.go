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

// BatchTask describes one member of an ordered coroutine batch.
type BatchTask struct {
	Owner  ThreadObj
	Before func(Thread)
	Run    func(Thread)
}

// StartBatch registers all tasks, then admits Run in slice order.
// Before runs pre-admission; wait modes require a managed caller.
func (p *Coroutines) StartBatch(tasks []BatchTask, mode BatchMode) []Thread {
	if mode != BatchAsync && mode != BatchWaitFirstSlice && mode != BatchWaitDone {
		panic("coroutine: invalid batch mode")
	}
	if len(tasks) == 0 {
		return nil
	}

	progress := newLatchSet(p, len(tasks)+1)
	threads := make([]Thread, len(tasks))
	for i, task := range tasks {
		current, next := progress[i], progress[i+1]
		threads[i] = p.Create(task.Owner, func(thread Thread) int {
			defer next.Open()
			if task.Before != nil {
				task.Before(thread)
			}
			current.Wait()
			next.Open()
			task.Run(thread)
			return 0
		})
	}

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

func newLatchSet(p *Coroutines, size int) []*Latch {
	latches := make([]*Latch, size)
	for i := range latches {
		latches[i] = p.NewLatch()
	}
	return latches
}

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
