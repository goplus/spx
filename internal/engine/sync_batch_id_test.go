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

package engine_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/engine"
)

func TestLegacyBatchObjectIDs(t *testing.T) {
	serializers := []struct {
		name      string
		serialize func(int64) []float32
		idIndex   int
	}{
		{
			name: "transform",
			serialize: func(id int64) []float32 {
				buffer := engine.NewSpriteSyncBuffer(2)
				buffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
				buffer.Add(id, 2, 3, 4, 5, 6, 7, 8, true)
				buffer.AddDelete(2)
				return buffer.Serialize()
			},
			idIndex: 2 + engine.SyncFieldsPerSprite,
		},
		{
			name: "delete",
			serialize: func(id int64) []float32 {
				buffer := engine.NewSpriteSyncBuffer(1)
				buffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
				buffer.AddDelete(2)
				buffer.AddDelete(id)
				return buffer.Serialize()
			},
			idIndex: 2 + engine.SyncFieldsPerSprite + 1,
		},
		{
			name: "visual",
			serialize: func(id int64) []float32 {
				buffer := engine.NewVisualSyncBuffer(2)
				buffer.AddRenderScale(1, 2)
				buffer.AddFull(id, 2, 3, true, [4]float64{4, 5, 6, 7}, true)
				return buffer.Serialize()
			},
			idIndex: 1 + engine.VisualFieldsPerSprite,
		},
	}
	cases := []struct {
		id    int64
		valid bool
	}{
		{0, true},
		{1, true},
		{1<<24 - 1, true},
		{1 << 24, true},
		{1<<24 + 1, false},
		{1<<24 + 2, true},
		{1 << 53, true},
		{1<<53 + 1, false},
		{1 << 60, true},
		{1<<60 + 1, false},
		{math.MaxInt64 - (1 << 39) + 1, true}, // Largest float32 below 2^63.
		{math.MaxInt64, false},
		{-1, false},
		{math.MinInt64, false},
	}
	for _, serializer := range serializers {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%d", serializer.name, tc.id), func(t *testing.T) {
				flushed := false
				defer func() {
					recovered := recover()
					if tc.valid {
						if recovered != nil {
							t.Fatalf("exactly representable ID caused panic: %v", recovered)
						}
						if !flushed {
							t.Fatal("valid packet was not flushed")
						}
						return
					}
					if recovered == nil {
						t.Fatal("invalid ID did not cause a panic")
					}
					if flushed {
						t.Fatal("packet containing an invalid ID was partially submitted")
					}
					if !strings.Contains(fmt.Sprint(recovered), fmt.Sprint(tc.id)) {
						t.Fatalf("panic %q does not identify original ID %d", recovered, tc.id)
					}
				}()
				coreruntime.FlushSerializedBuffer(1, 0,
					func() []float32 { return serializer.serialize(tc.id) },
					func(packet []float32) {
						flushed = true
						if tc.valid && int64(packet[serializer.idIndex]) != tc.id {
							t.Errorf("serialized ID %v does not round-trip to %d", packet[serializer.idIndex], tc.id)
						}
					},
				)
			})
		}
	}
}
