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

package spx

import (
	"math/rand"
	"sync"
	"time"
)

type randomScope uint8

const (
	randomScopeShared randomScope = iota
	randomScopePerCoroutine
)

type randomState struct {
	mu            sync.Mutex
	scope         randomScope
	seed          int64
	global        *rand.Rand
	coroutineBase int64
	coroutines    map[int64]*rand.Rand
}

var scriptRandom = newRandomState()

// SetRandomSeed resets SPX's script-level random source to a deterministic seed.
func SetRandomSeed(seed int64) {
	scriptRandom.setSeed(seed, randomScopeShared)
}

func setDeterministicRandomSeed(seed int64) {
	scriptRandom.setSeed(seed, randomScopePerCoroutine)
}

// ResetRandomSeed resets SPX's script-level random source to a non-deterministic seed.
func ResetRandomSeed() {
	SetRandomSeed(time.Now().UnixNano())
}

func randomIntn(n int) int {
	return scriptRandom.intn(n)
}

func randomInt31n(n int32) int32 {
	return scriptRandom.int31n(n)
}

func randomFloat64() float64 {
	return scriptRandom.float64()
}

func newRandomState() *randomState {
	return &randomState{
		global:     newRandomSource(time.Now().UnixNano()),
		coroutines: make(map[int64]*rand.Rand),
	}
}

func newRandomSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func (r *randomState) setSeed(seed int64, scope randomScope) {
	r.mu.Lock()
	r.seed = seed
	r.scope = scope
	r.global = newRandomSource(seed)
	// Capture the current coroutine high-water mark so a deterministic reload
	// in the same process keeps newly created coroutine streams stable.
	r.coroutineBase = lastCoroutineRandomID()
	clear(r.coroutines)
	r.mu.Unlock()
}

func (r *randomState) intn(n int) int {
	r.mu.Lock()
	v := r.streamLocked().Intn(n)
	r.mu.Unlock()
	return v
}

func (r *randomState) int31n(n int32) int32 {
	r.mu.Lock()
	v := r.streamLocked().Int31n(n)
	r.mu.Unlock()
	return v
}

func (r *randomState) float64() float64 {
	r.mu.Lock()
	v := r.streamLocked().Float64()
	r.mu.Unlock()
	return v
}

func (r *randomState) streamLocked() *rand.Rand {
	if r.scope != randomScopePerCoroutine {
		return r.global
	}

	streamID, ok := r.currentCoroutineStreamIDLocked()
	if !ok {
		return r.global
	}
	if src, exists := r.coroutines[streamID]; exists {
		return src
	}

	src := newRandomSource(mixRandomSeed(r.seed, streamID))
	r.coroutines[streamID] = src
	return src
}

func (r *randomState) currentCoroutineStreamIDLocked() (int64, bool) {
	id := currentCoroutineRandomID()
	if id <= 0 {
		return 0, false
	}
	return id - r.coroutineBase, true
}

func currentCoroutineRandomID() int64 {
	if gco == nil {
		return 0
	}
	if !gco.IsInCoroutine() {
		return 0
	}
	if th := gco.Current(); th != nil {
		return th.ID()
	}
	return 0
}

func lastCoroutineRandomID() int64 {
	if gco == nil {
		return 0
	}
	return gco.LastThreadID()
}

func mixRandomSeed(baseSeed, streamID int64) int64 {
	const increment = uint64(0x9e3779b97f4a7c15)

	x := uint64(baseSeed) + increment + uint64(streamID)*increment
	x = (x ^ (x >> 30)) * uint64(0xbf58476d1ce4e5b9)
	x = (x ^ (x >> 27)) * uint64(0x94d049bb133111eb)
	x ^= x >> 31
	return int64(x)
}
