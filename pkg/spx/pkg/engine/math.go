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
	"math"

	mathf "github.com/goplus/spbase/mathf"
)

func Abs(x float64) float64 {
	return float64(math.Abs(float64(x)))
}
func Sign(x float64) int64 {
	if x < 0 {
		return -1
	} else if x > 0 {
		return 1
	}
	return 0
}

func DegToRad(degrees float64) float64 {
	return degrees * (math.Pi / 180.0)
}

func RadToDeg(radians float64) float64 {
	return radians * (180.0 / math.Pi)
}

func AngleToPoint(v mathf.Vec2, v2 mathf.Vec2) float64 {
	return Angle(v.Sub(v2))
}

// Angle returns the angle of v in radians, measured from the positive X axis.
func Angle(v mathf.Vec2) float64 {
	return float64(mathf.Atan2(float64(v.Y), float64(v.X)))
}
