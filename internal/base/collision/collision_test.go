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

import (
	"testing"
)

func TestParseColliderShapeType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		def   int64
		want  int64
	}{
		{name: "none", input: "none", def: ColliderAuto, want: ColliderNone},
		{name: "auto", input: "auto", def: ColliderNone, want: ColliderAuto},
		{name: "circle", input: "circle", def: ColliderNone, want: ColliderCircle},
		{name: "rect", input: "rect", def: ColliderNone, want: ColliderRect},
		{name: "capsule", input: "capsule", def: ColliderNone, want: ColliderCapsule},
		{name: "polygon", input: "polygon", def: ColliderNone, want: ColliderPolygon},
		{name: "empty default", input: "", def: ColliderRect, want: ColliderRect},
		{name: "unknown default", input: "bad", def: ColliderCapsule, want: ColliderCapsule},
	}

	for _, tt := range tests {
		if got := ParseColliderShapeType(tt.input, tt.def); got != tt.want {
			t.Fatalf("%s: ParseColliderShapeType(%q, %d) = %d, want %d", tt.name, tt.input, tt.def, got, tt.want)
		}
	}
}

func TestParsePixelCollisionPrecision(t *testing.T) {
	if got := ParsePixelCollisionPrecision(nil); got != PixelPrecisionHigh {
		t.Fatalf("ParsePixelCollisionPrecision(nil) = %d, want %d", got, PixelPrecisionHigh)
	}
	tests := []struct {
		input string
		want  int64
	}{
		{input: "high", want: PixelPrecisionHigh},
		{input: "medium", want: PixelPrecisionMedium},
		{input: "low", want: PixelPrecisionLow},
		{input: "bad", want: PixelPrecisionHigh},
	}
	for _, tt := range tests {
		input := tt.input
		if got := ParsePixelCollisionPrecision(&input); got != tt.want {
			t.Fatalf("ParsePixelCollisionPrecision(%q) = %d, want %d", input, got, tt.want)
		}
	}
}

func TestAABBIntersects(t *testing.T) {
	a := AABB{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}
	b := AABB{MinX: 5, MinY: 5, MaxX: 15, MaxY: 15}
	c := AABB{MinX: 11, MinY: 11, MaxX: 20, MaxY: 20}

	if !a.Intersects(b) {
		t.Fatal("expected overlapping boxes to intersect")
	}
	if a.Intersects(c) {
		t.Fatal("expected separated boxes not to intersect")
	}
}
