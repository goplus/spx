/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

import "container/heap"

// readyQueue partitions runnable scripts around its cursor. Each
// heap is ordered by the original script position, including handler restarts.
// The current heap holds positions after the cursor; the other waits for wrap.
// Caller holds schedulerMu; the queue must not be copied after its first use.
type readyQueue struct {
	heaps   [2]readyHeap
	current int
	cursor  int64 // Last selected live script position.
}

func (q *readyQueue) add(th Thread) {
	index := q.current
	if th.resumeOrder <= q.cursor {
		index ^= 1
	}
	heap.Push(&q.heaps[index], th)
}

func (q *readyQueue) remove(th Thread) {
	if th.readyHeap != nil {
		heap.Remove(th.readyHeap, th.readyIndex)
	}
}

func (q *readyQueue) pop() Thread {
	for {
		if len(q.heaps[q.current]) == 0 {
			q.current ^= 1
			if len(q.heaps[q.current]) == 0 {
				return nil
			}
		}
		th := heap.Pop(&q.heaps[q.current]).(Thread)
		if !th.canceled() {
			q.cursor = th.resumeOrder
			return th
		}
	}
}

type readyHeap []Thread

func (h readyHeap) Len() int { return len(h) }

func (h readyHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	if a.resumeOrder != b.resumeOrder {
		return a.resumeOrder < b.resumeOrder
	}
	return a.id < b.id
}

func (h readyHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].readyIndex, h[j].readyIndex = i, j
}

func (h *readyHeap) Push(value any) {
	th := value.(Thread)
	th.readyHeap, th.readyIndex = h, len(*h)
	*h = append(*h, th)
}

func (h *readyHeap) Pop() any {
	last := len(*h) - 1
	th := (*h)[last]
	(*h)[last] = nil
	*h = (*h)[:last]
	th.readyHeap = nil
	return th
}
