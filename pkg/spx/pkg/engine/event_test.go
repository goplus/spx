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

package engine

import "testing"

func TestEvent0TriggerClearsTempActions(t *testing.T) {
	ev := NewEvent0()
	ev.Subscribe(func() {})
	ev.Subscribe(func() {})

	ev.Trigger()

	for i, action := range ev.tempActions {
		if action != nil {
			t.Fatalf("tempActions[%d] should be cleared after trigger", i)
		}
	}
	if len(ev.tempIds) != 0 {
		t.Fatalf("tempIds should be cleared after trigger, got %d", len(ev.tempIds))
	}
}

func TestEvent2TriggerClearsShrunkTempActions(t *testing.T) {
	ev := NewEvent2[int, int]()
	firstID := ev.Subscribe(func(int, int) {})
	ev.Subscribe(func(int, int) {})

	ev.Trigger(1, 2)
	ev.Unsubscribe(firstID)
	ev.Trigger(3, 4)

	for i, action := range ev.tempActions {
		if action != nil {
			t.Fatalf("tempActions[%d] should be cleared after trigger", i)
		}
	}
	if len(ev.tempIds) != 0 {
		t.Fatalf("tempIds should be cleared after trigger, got %d", len(ev.tempIds))
	}
}
