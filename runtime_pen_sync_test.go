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
	"reflect"
	"testing"

	internalengine "github.com/goplus/spx/v3/internal/engine"
)

func TestPenComponentQueuesOneOrderedBatch(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	sprite.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
	sprite.transform().x = 12
	sprite.transform().y = 34

	sprite.pen().SetPenColor(HSB(20, 80, 90))
	sprite.pen().PenDown()
	sprite.pen().PenUp()
	sprite.g.flushPenCommands()

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", spy.batchCalls)
	}
	if spy.moveCalls != 0 || spy.penDownCalls != 0 || spy.penUpCalls != 0 || spy.setColorCalls != 0 || spy.setSizeCalls != 0 {
		t.Fatalf("individual calls = move:%d down:%d up:%d color:%d size:%d, want all zero",
			spy.moveCalls, spy.penDownCalls, spy.penUpCalls, spy.setColorCalls, spy.setSizeCalls)
	}

	batch := spy.batches[0]
	if got, want := int(batch[0]), 6; got != want {
		t.Fatalf("command count = %d, want %d", got, want)
	}
	wantOps := []int{
		internalengine.PenBatchColor,
		internalengine.PenBatchSetSize,
		internalengine.PenBatchColor,
		internalengine.PenBatchMove,
		internalengine.PenBatchDown,
		internalengine.PenBatchUp,
	}
	gotOps := penBatchOperations(batch)
	if !reflect.DeepEqual(gotOps, wantOps) {
		t.Fatalf("command operations = %v, want %v", gotOps, wantOps)
	}

	move := batch[1+3*internalengine.PenBatchFields : 1+4*internalengine.PenBatchFields]
	if got, want := move[3], float32(12); got != want {
		t.Fatalf("move x = %v, want %v", got, want)
	}
	if got, want := move[4], float32(34); got != want {
		t.Fatalf("move y = %v, want logical SPX y %v", got, want)
	}
}

func TestPenBatchBarriersPreserveOrder(t *testing.T) {
	tests := []struct {
		name   string
		action func(*penTestSprite)
		last   string
	}{
		{
			name: "stamp",
			action: func(sprite *penTestSprite) {
				configurePenRenderOffsetSprite(sprite)
				sprite.pen().Stamp()
			},
			last: "stamp",
		},
		{
			name: "erase all",
			action: func(sprite *penTestSprite) {
				sprite.g.EraseAll()
			},
			last: "erase",
		},
		{
			name: "destroy",
			action: func(sprite *penTestSprite) {
				sprite.pen().onDestroy()
			},
			last: "destroy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := setupSpyPenMgr(t)
			sprite := newPenTestSprite()
			sprite.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
			sprite.pen().PenDown()

			tt.action(sprite)

			if got, want := spy.events, []string{"batch", tt.last}; !reflect.DeepEqual(got, want) {
				t.Fatalf("events = %v, want %v", got, want)
			}
		})
	}
}

func TestOnEngineRenderFlushesPendingPenCommandsOnEarlyReturn(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.queuePenUp(7)

	game.OnEngineRender(0)

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", spy.batchCalls)
	}
	if got, want := penBatchOperations(spy.batches[0]), []int{internalengine.PenBatchUp}; !reflect.DeepEqual(got, want) {
		t.Fatalf("command operations = %v, want %v", got, want)
	}
}

func TestOnEngineDestroyDiscardsPendingPenCommands(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.queuePenUp(7)

	game.OnEngineDestroy()
	game.flushPenCommands()

	if spy.batchCalls != 0 {
		t.Fatalf("batch calls = %d, want 0", spy.batchCalls)
	}
}

func TestPenBatchFlushesAtCommandLimit(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	for i := 0; i < 1025; i++ {
		game.queuePenUp(internalengine.Object(i + 1))
	}

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls at limit = %d, want 1", spy.batchCalls)
	}
	if got, want := int(spy.batches[0][0]), 1024; got != want {
		t.Fatalf("first batch command count = %d, want %d", got, want)
	}
	game.flushPenCommands()
	if spy.batchCalls != 2 {
		t.Fatalf("batch calls after final flush = %d, want 2", spy.batchCalls)
	}
	if got, want := int(spy.batches[1][0]), 1; got != want {
		t.Fatalf("second batch command count = %d, want %d", got, want)
	}
}

func penBatchOperations(batch []float32) []int {
	count := int(batch[0])
	operations := make([]int, count)
	for i := range operations {
		operations[i] = int(batch[1+i*internalengine.PenBatchFields])
	}
	return operations
}
