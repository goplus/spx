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

package input

import (
	"sync"
	"time"
)

// ClickGate suppresses repeated click handling within a minimum interval.
type ClickGate struct {
	mu            sync.Mutex
	minIntervalMs int64
	lastClickMs   map[int64]int64
	now           func() time.Time
}

// A nil clock falls back to the system clock.
func (g *ClickGate) InitWithClock(minInterval time.Duration, now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.minIntervalMs = int64(minInterval / time.Millisecond)
	g.lastClickMs = make(map[int64]int64)
	g.now = now
}

func (g *ClickGate) Allow(id int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	currentTimeMs := g.nowMs()
	if g.minIntervalMs <= 0 {
		return true
	}

	g.pruneLocked(currentTimeMs - g.minIntervalMs)
	if lastTimeMs, ok := g.lastClickMs[id]; ok && currentTimeMs-lastTimeMs < g.minIntervalMs {
		return false
	}
	g.lastClickMs[id] = currentTimeMs
	return true
}

func (g *ClickGate) Remove(id int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.lastClickMs, id)
}

func (g *ClickGate) pruneLocked(expireBefore int64) {
	for id, lastTimeMs := range g.lastClickMs {
		if lastTimeMs <= expireBefore {
			delete(g.lastClickMs, id)
		}
	}
}

func (g *ClickGate) nowMs() int64 {
	now := g.now
	if now == nil {
		now = time.Now
	}
	return now().UnixNano() / int64(time.Millisecond)
}
