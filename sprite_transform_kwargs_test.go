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

import "testing"

func TestMotionOptions(t *testing.T) {
	tests := []struct {
		name          string
		opts          *MotionOptions
		wantSpeed     Speed
		wantAnimation SpriteAnimationName
	}{
		{
			name:      "nil uses defaults",
			opts:      nil,
			wantSpeed: 1,
		},
		{
			name: "zero speed keeps default and animation",
			opts: &MotionOptions{
				Animation: "walk",
			},
			wantSpeed:     1,
			wantAnimation: "walk",
		},
		{
			name: "positive speed overrides default",
			opts: &MotionOptions{
				Speed:     2.5,
				Animation: "run",
			},
			wantSpeed:     2.5,
			wantAnimation: "run",
		},
		{
			name: "negative speed falls back to default",
			opts: &MotionOptions{
				Speed:     -3,
				Animation: "run",
			},
			wantSpeed:     1,
			wantAnimation: "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpeed, gotAnimation := motionOptions(tt.opts)
			if gotSpeed != tt.wantSpeed {
				t.Fatalf("speed = %v, want %v", gotSpeed, tt.wantSpeed)
			}
			if gotAnimation != tt.wantAnimation {
				t.Fatalf("animation = %q, want %q", gotAnimation, tt.wantAnimation)
			}
		})
	}
}
