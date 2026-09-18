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

func TestTriggerEventsObserveFrameBoundary(t *testing.T) {
	resetTriggerEvents()
	t.Cleanup(resetTriggerEvents)

	firstSrc, firstDst := &Sprite{}, &Sprite{}
	secondSrc, secondDst := &Sprite{}, &Sprite{}
	enqueueTriggerEvent(firstSrc, firstDst)
	if events := GetTriggerEvents(nil); len(events) != 0 {
		t.Fatalf("events before frame boundary = %+v, want none", events)
	}

	cacheTriggerEvents()
	enqueueTriggerEvent(secondSrc, secondDst)
	events := GetTriggerEvents(nil)
	if len(events) != 1 || events[0].Src != firstSrc || events[0].Dst != firstDst {
		t.Fatalf("current-frame events = %+v, want first event", events)
	}
	if events := GetTriggerEvents(nil); len(events) != 0 {
		t.Fatalf("events after drain = %+v, want none", events)
	}

	cacheTriggerEvents()
	events = GetTriggerEvents(nil)
	if len(events) != 1 || events[0].Src != secondSrc || events[0].Dst != secondDst {
		t.Fatalf("next-frame events = %+v, want second event", events)
	}
}
