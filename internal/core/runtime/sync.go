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

package runtime

import (
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

func SyncOnce(start *sync.Once, fire func()) {
	start.Do(fire)
}

func SyncMousePos(pos mathf.Vec2, setMousePos func(mathf.Vec2)) {
	setMousePos(pos)
}

func SyncBatchPositions[T any](
	items []T,
	shouldSync func(T) bool,
	idOf func(T) int64,
	fetchPositions func([]int64) []float32,
	applyPosition func(T, float64, float64),
) {
	spriteIDs := make([]int64, 0, len(items))
	targets := make([]T, 0, len(items))
	for _, item := range items {
		if !shouldSync(item) {
			continue
		}
		spriteIDs = append(spriteIDs, idOf(item))
		targets = append(targets, item)
	}
	if len(spriteIDs) == 0 {
		return
	}

	positions := fetchPositions(spriteIDs)
	for i, target := range targets {
		applyPosition(target, float64(positions[i*2]), float64(positions[i*2+1]))
	}
}

func FlushSerializedBuffer[T any](
	updateCount int,
	deleteCount int,
	serialize func() T,
	flush func(T),
) {
	if updateCount > 0 || deleteCount > 0 {
		flush(serialize())
	}
}

func ProcessTriggerPairs[T any](
	pairs []engine.TriggerEvent,
	resolve func(any) (T, bool),
	isTouchable func(T) bool,
	onTouch func(T, T),
	onInvalid func(),
) {
	for _, pair := range pairs {
		if pair.Src == nil || pair.Dst == nil {
			onInvalid()
			continue
		}

		src, ok1 := resolve(pair.Src.Target)
		dst, ok2 := resolve(pair.Dst.Target)
		if !ok1 || !ok2 {
			onInvalid()
			continue
		}
		if !isTouchable(src) || !isTouchable(dst) {
			continue
		}
		onTouch(src, dst)
	}
}
