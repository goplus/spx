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
	"testing"

	mathf "github.com/goplus/spbase/mathf"
)

func TestHeadingToPoint(t *testing.T) {
	from := mathf.NewVec2(0, 0)

	tests := []struct {
		name string
		to   mathf.Vec2
		want float64
	}{
		{name: "right", to: mathf.NewVec2(10, 0), want: 90},
		{name: "up", to: mathf.NewVec2(0, 10), want: 0},
		{name: "left", to: mathf.NewVec2(-10, 0), want: -90},
		{name: "down", to: mathf.NewVec2(0, -10), want: 180},
		{name: "bottom left keeps raw heading", to: mathf.NewVec2(-10, -10), want: 225},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HeadingToPoint(from, tt.to); Abs(got-tt.want) > 1e-9 {
				t.Fatalf("HeadingToPoint(%v, %v) = %v, want %v", from, tt.to, got, tt.want)
			}
		})
	}
}
