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
	"math/rand/v2"
	"testing"
)

func TestWaitQueuePushPopPreservesIdentityAndClearsLinks(t *testing.T) {
	var q, reused waitQueue
	jobs := []*waitJob{{}, {}, {}}
	for _, job := range jobs {
		q.pushBack(job)
	}
	assertWaitQueue(t, &q, jobs)
	for i, want := range jobs {
		job := q.popFront()
		if job != want || job.next != nil {
			t.Fatalf("pop %d = %p, want %p with nil next", i, job, want)
		}
		reused.pushBack(job)
		assertWaitQueue(t, &q, jobs[i+1:])
		assertWaitQueue(t, &reused, jobs[:i+1])
	}
}

func TestWaitQueuePopFrontPanicsWhenEmpty(t *testing.T) {
	var q waitQueue
	defer func() {
		if recovered := recover(); recovered != "queue is empty" {
			t.Fatalf("empty PopFront panic = %v, want queue is empty", recovered)
		}
	}()
	q.popFront()
}

func TestWaitQueueAppendFrom(t *testing.T) {
	for _, tc := range []struct {
		name           string
		dstLen, srcLen int
	}{
		{"both empty", 0, 0},
		{"empty destination", 0, 2},
		{"empty source", 2, 0},
		{"both nonempty", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var dst, src waitQueue
			var expected []*waitJob
			for range tc.dstLen {
				job := &waitJob{}
				dst.pushBack(job)
				expected = append(expected, job)
			}
			for range tc.srcLen {
				job := &waitJob{}
				src.pushBack(job)
				expected = append(expected, job)
			}
			dst.appendFrom(&src)
			assertWaitQueue(t, &dst, expected)
			assertWaitQueue(t, &src, nil)

			job := &waitJob{}
			src.pushBack(job)
			assertWaitQueue(t, &src, []*waitJob{job})
			assertWaitQueue(t, &dst, expected)
			dst.appendFrom(&src)
			assertWaitQueue(t, &dst, append(expected, job))
			assertWaitQueue(t, &src, nil)
		})
	}
}

func TestWaitQueueAppendToSelfIsNoOp(t *testing.T) {
	var q waitQueue
	q.appendFrom(&q)
	assertWaitQueue(t, &q, nil)
	jobs := []*waitJob{{}, {}}
	for _, job := range jobs {
		q.pushBack(job)
	}
	q.appendFrom(&q)
	assertWaitQueue(t, &q, jobs)
}

func TestWaitQueueRandomOperations(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	var queues [2]waitQueue
	var expected [2][]*waitJob
	var reusable []*waitJob
	for step := range 2000 {
		i := random.IntN(len(queues))
		q := &queues[i]
		switch random.IntN(4) {
		case 0:
			var job *waitJob
			if len(reusable) > 0 {
				job = reusable[len(reusable)-1]
				reusable = reusable[:len(reusable)-1]
			} else {
				job = &waitJob{}
			}
			q.pushBack(job)
			expected[i] = append(expected[i], job)
		case 1:
			if len(expected[i]) > 0 {
				job := q.popFront()
				if job != expected[i][0] || job.next != nil {
					t.Fatalf("step %d: PopFront = %p, want %p with nil next", step, job, expected[i][0])
				}
				expected[i] = expected[i][1:]
				reusable = append(reusable, job)
			}
		case 2:
			other := 1 - i
			q.appendFrom(&queues[other])
			expected[i] = append(expected[i], expected[other]...)
			expected[other] = nil
		case 3:
			q.appendFrom(q)
		}
		for j := range queues {
			assertWaitQueue(t, &queues[j], expected[j])
		}
	}
	for i := range queues {
		for _, want := range expected[i] {
			if got := queues[i].popFront(); got != want || got.next != nil {
				t.Fatalf("queue %d final PopFront = %p, want %p with nil next", i, got, want)
			}
		}
		assertWaitQueue(t, &queues[i], nil)
	}
}

func assertWaitQueue(t *testing.T, q *waitQueue, expected []*waitJob) {
	t.Helper()
	if got := q.len(); got != len(expected) {
		t.Fatalf("queue length = %d, want %d", got, len(expected))
	}
	if len(expected) == 0 {
		if q.head != nil || q.tail != nil {
			t.Fatalf("empty queue head/tail = %p/%p, want nil/nil", q.head, q.tail)
		}
		return
	}
	if q.head != expected[0] || q.tail != expected[len(expected)-1] {
		t.Fatalf("queue head/tail = %p/%p, want %p/%p", q.head, q.tail, expected[0], expected[len(expected)-1])
	}
	job := q.head
	for i, want := range expected {
		if job != want {
			t.Fatalf("queue item %d = %p, want %p", i, job, want)
		}
		job = job.next
	}
	if job != nil {
		t.Fatalf("queue has unexpected trailing job %p", job)
	}
}
