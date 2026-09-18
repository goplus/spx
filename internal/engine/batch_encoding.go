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

import (
	"fmt"
	"math"
)

func boolArg(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func encodeLegacyBatchObjectID(id int64) float32 {
	value := float32(id)
	// Check the upper bound before converting back: MaxInt64 rounds up to 2^63.
	if id < 0 || value >= 0x1p63 || int64(value) != id {
		panic(fmt.Errorf("invalid sprite ID %d for legacy float32 batch encoding", id))
	}
	return value
}

func splitInt64BitsToFloat32(value int64) (low, high float32) {
	u := uint64(value)
	return uint32BitsToFloat32(uint32(u)), uint32BitsToFloat32(uint32(u >> 32))
}

func uint32BitsToFloat32(value uint32) float32 {
	return math.Float32frombits(value)
}

func ensureFloat32BufferSize(buffer []float32, size int) []float32 {
	if cap(buffer) < size {
		newCap := size * 2
		if newCap < size || newCap < 16 {
			newCap = size
			if newCap < 16 {
				newCap = 16
			}
		}
		return make([]float32, size, newCap)
	}
	return buffer[:size]
}
