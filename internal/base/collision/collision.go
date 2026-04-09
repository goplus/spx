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

package collision

import spxlog "github.com/goplus/spx/v2/internal/log"

const (
	ColliderNone    int64 = 0x00
	ColliderAuto    int64 = 0x01
	ColliderCircle  int64 = 0x02
	ColliderRect    int64 = 0x03
	ColliderCapsule int64 = 0x04
	ColliderPolygon int64 = 0x05
)

const (
	PixelPrecisionHigh   int64 = 1
	PixelPrecisionMedium int64 = 2
	PixelPrecisionLow    int64 = 4
)

func ParseColliderShapeType(typeName string, defaultValue int64) int64 {
	switch typeName {
	case "none":
		return ColliderNone
	case "auto":
		return ColliderAuto
	case "circle":
		return ColliderCircle
	case "rect":
		return ColliderRect
	case "capsule":
		return ColliderCapsule
	case "polygon":
		return ColliderPolygon
	case "":
		return defaultValue
	default:
		spxlog.Warn("Invalid colliderShapeType value '%s', using default", typeName)
		return defaultValue
	}
}

func ParsePixelCollisionPrecision(precision *string) int64 {
	if precision == nil {
		return PixelPrecisionLow
	}
	switch *precision {
	case "high":
		return PixelPrecisionHigh
	case "medium":
		return PixelPrecisionMedium
	case "low":
		return PixelPrecisionLow
	default:
		spxlog.Warn("Invalid pixelCollisionPrecision value '%s', using default 'low'", *precision)
		return PixelPrecisionLow
	}
}
