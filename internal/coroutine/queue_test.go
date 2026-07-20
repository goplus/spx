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

import "testing"

func TestQueuePushPopPreservesOrder(t *testing.T) {
	q := NewQueue[int]()
	q.PushBack(1)
	q.PushBack(2)
	q.PushFront(0)

	if got := q.PopFront(); got != 0 {
		t.Fatalf("first PopFront = %d, want 0", got)
	}
	if got := q.PopFront(); got != 1 {
		t.Fatalf("second PopFront = %d, want 1", got)
	}
	if got := q.PopBack(); got != 2 {
		t.Fatalf("PopBack = %d, want 2", got)
	}
}

func TestQueueSupportsZeroValueWithPooling(t *testing.T) {
	var q Queue[int]

	q.PushBack(7)
	if got := q.PopFront(); got != 7 {
		t.Fatalf("PopFront = %d, want 7", got)
	}

	q.PushFront(9)
	if got := q.PopBack(); got != 9 {
		t.Fatalf("PopBack = %d, want 9", got)
	}
}

func TestQueueMoveToSelfIsNoOp(t *testing.T) {
	q := NewQueue[int]()
	q.PushBack(1)
	q.PushBack(2)

	q.Move(q)

	if got := q.Count(); got != 2 {
		t.Fatalf("Count after self move = %d, want 2", got)
	}
	if got := q.PopFront(); got != 1 {
		t.Fatalf("first PopFront after self move = %d, want 1", got)
	}
	if got := q.PopFront(); got != 2 {
		t.Fatalf("second PopFront after self move = %d, want 2", got)
	}
}
