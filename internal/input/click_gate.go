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

	engine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

// ClickGate suppresses repeated click handling within a minimum interval.
type ClickGate struct {
	mu            sync.Mutex
	minIntervalMs int64
	lastClickMs   map[engine.Object]int64
	now           func() time.Time
}

func (g *ClickGate) Init(minInterval time.Duration) {
	g.InitWithClock(minInterval, nil)
}

// InitWithClock initializes the click gate with a caller-provided clock.
// A nil clock falls back to the system clock.
func (g *ClickGate) InitWithClock(minInterval time.Duration, now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.minIntervalMs = int64(minInterval / time.Millisecond)
	g.lastClickMs = make(map[engine.Object]int64)
	g.now = now
}

func (g *ClickGate) Allow(id engine.Object) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	currentTimeMs := g.nowMs()
	if g.minIntervalMs <= 0 {
		return true
	}

	if g.lastClickMs == nil {
		g.lastClickMs = make(map[engine.Object]int64)
	}
	g.pruneLocked(currentTimeMs - g.minIntervalMs)
	if lastTimeMs, ok := g.lastClickMs[id]; ok {
		if currentTimeMs-lastTimeMs < g.minIntervalMs {
			return false
		}
	}
	g.lastClickMs[id] = currentTimeMs
	return true
}

func (g *ClickGate) Remove(id engine.Object) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.lastClickMs, id)
}

func (g *ClickGate) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	clear(g.lastClickMs)
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
