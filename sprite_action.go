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

import "math"

// switchAction represents the direction for switching costumes.
type switchAction int

const (
	Prev switchAction = -1
	Next switchAction = 1
)

// layerAction represents the direction for layer changes.
type layerAction int

const (
	Front layerAction = -1
	Back  layerAction = 1
)

// dirAction represents forward or backward direction.
type dirAction int

const (
	Forward  dirAction = -1
	Backward dirAction = 1
)

// toRadian converts degrees to radians.
func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
}

// normalizeDirection normalizes a direction angle to the range (-180, 180].
func normalizeDirection(dir float64) float64 {
	if dir <= -180 {
		dir += 360
	} else if dir > 180 {
		dir -= 360
	}
	return dir
}
