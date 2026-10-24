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

package assets

import (
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestToBitmapResolution(t *testing.T) {
	if got := ToBitmapResolution(0); got != DefaultBitmapResolution {
		t.Fatalf("ToBitmapResolution(0) = %d, want %d", got, DefaultBitmapResolution)
	}
	if got := ToBitmapResolution(2); got != 2 {
		t.Fatalf("ToBitmapResolution(2) = %d, want 2", got)
	}
}

func TestDefaultAtlasUV(t *testing.T) {
	if got := DefaultAtlasUV(); got != mathf.NewVec4(0, 0, FullUVRange, FullUVRange) {
		t.Fatalf("DefaultAtlasUV() = %v, want full UV range", got)
	}
}

func TestCalculateAtlasUV(t *testing.T) {
	got := CalculateAtlasUV(10, 20, 30, 40, mathf.NewVec2(100, 200))
	want := mathf.NewVec4(0.1, 0.1, 0.3, 0.2)
	if got != want {
		t.Fatalf("CalculateAtlasUV() = %v, want %v", got, want)
	}
}

func TestResolveImageSize(t *testing.T) {
	got := ResolveImageSize(32, 16, "ignored", func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called when config size is present")
		return mathf.Vec2{}
	})
	if got != (mathf.Vec2{X: 32, Y: 16}) {
		t.Fatalf("ResolveImageSize configured = %v, want {32 16}", got)
	}

	fallbackCalled := false
	got = ResolveImageSize(0, 0, "path", func(path string) mathf.Vec2 {
		fallbackCalled = true
		if path != "path" {
			t.Fatalf("fallback path = %q, want %q", path, "path")
		}
		return mathf.NewVec2(8, 4)
	})
	if !fallbackCalled {
		t.Fatal("fallback was not called")
	}
	if got != mathf.NewVec2(8, 4) {
		t.Fatalf("ResolveImageSize fallback = %v, want {8 4}", got)
	}
}

func TestNewSizedFrame(t *testing.T) {
	got := NewSizedFrame(10, 20)
	if got.Width != 10 || got.Height != 20 {
		t.Fatalf("NewSizedFrame size = (%d, %d), want (10, 20)", got.Width, got.Height)
	}
	if got.Center != mathf.NewVec2(5, 10) {
		t.Fatalf("NewSizedFrame center = %v, want {5 10}", got.Center)
	}
}

func TestNewAtlasFrame(t *testing.T) {
	tests := []struct {
		name       string
		nx, index  int
		rect       [4]float64
		width      int
		height     int
		posX, posY int
	}{
		{"full image", 5, 2, [4]float64{}, 20, 20, 40, 0},
		{"zero columns", 0, 0, [4]float64{}, 100, 20, 0, 0},
		{"negative columns", -2, 0, [4]float64{}, 100, 20, 0, 0},
		{"fractional region", 3, 1, [4]float64{7.5, 3.5, 31.5, 11.5}, 10, 11, 17, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NewAtlasFrame(100, 20, "path", test.rect[0], test.rect[1], test.rect[2], test.rect[3], test.nx, test.index, 2, func(string) mathf.Vec2 {
				t.Fatal("fallback should not be called")
				return mathf.Vec2{}
			})
			want := FrameDescriptor{
				Width: test.width, Height: test.height, BitmapResolution: 2,
				ImageSize: mathf.NewVec2(100, 20),
				Center:    mathf.NewVec2(float64(test.width)/2, float64(test.height)/2),
				PosX:      test.posX, PosY: test.posY,
				AtlasUVRect: mathf.NewVec4(float64(test.posX)/100, float64(test.posY)/20, float64(test.width)/100, float64(test.height)/20),
			}
			if got != want {
				t.Fatalf("NewAtlasFrame() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNewStandaloneFrame(t *testing.T) {
	got := NewStandaloneFrame(32, 16, "path", 0, func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called")
		return mathf.Vec2{}
	})
	if got.Width != 32 || got.Height != 16 {
		t.Fatalf("NewStandaloneFrame size = (%d, %d), want (32, 16)", got.Width, got.Height)
	}
	if got.BitmapResolution != DefaultBitmapResolution {
		t.Fatalf("NewStandaloneFrame bitmap resolution = %d, want %d", got.BitmapResolution, DefaultBitmapResolution)
	}
	if got.Center != mathf.NewVec2(16, 8) {
		t.Fatalf("NewStandaloneFrame center = %v, want {16 8}", got.Center)
	}
}
