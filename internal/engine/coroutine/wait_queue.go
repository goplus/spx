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
	next     *waitJob // Owned by the waitQueue; nil when detached.
	thread   Thread   // Waiting coroutine.
	kind     waitKind
	deadline float64 // Level-time deadline for a time job.
	frame    int64   // Issuing frame for frame-gated jobs.
}

// waitQueue is a FIFO whose caller owns synchronization. Its zero value is ready
// for use. A job may belong to only one queue at a time; a nonempty queue must not
// be copied.
type waitQueue struct {
	head  *waitJob
	tail  *waitJob
	count int
}

func (q *waitQueue) len() int {
	return q.count
}

// pushBack appends a non-nil job that is not already in a queue.
func (q *waitQueue) pushBack(job *waitJob) {
	if q.count == 0 {
		q.head = job
	} else {
		q.tail.next = job
	}
	q.tail = job
	q.count++
}

// popFront detaches and returns the front job. It panics if the queue is empty.
func (q *waitQueue) popFront() *waitJob {
	if q.count == 0 {
		panic("queue is empty")
	}

	job := q.head
	q.head = job.next
	job.next = nil
	q.count--

	if q.count == 0 {
		q.tail = nil
	}

	return job
}

// appendFrom appends all jobs from src in O(1) and empties src.
func (q *waitQueue) appendFrom(src *waitQueue) {
	if q == src || src.count == 0 {
		return
	}

	if q.count == 0 {
		q.head = src.head
	} else {
		q.tail.next = src.head
	}
	q.tail = src.tail
	q.count += src.count

	*src = waitQueue{}
}
