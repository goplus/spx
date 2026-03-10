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
	got := NewAtlasFrame(100, 20, "path", 0, 0, 0, 0, 5, 2, 1, func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called")
		return mathf.Vec2{}
	})
	if got.Width != 20 || got.Height != 20 {
		t.Fatalf("NewAtlasFrame size = (%d, %d), want (20, 20)", got.Width, got.Height)
	}
	if got.PosX != 40 || got.PosY != 0 {
		t.Fatalf("NewAtlasFrame position = (%d, %d), want (40, 0)", got.PosX, got.PosY)
	}
}

func TestNewAtlasFrameDefaultsZeroNxToOne(t *testing.T) {
	got := NewAtlasFrame(100, 20, "path", 0, 0, 0, 0, 0, 0, 1, func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called")
		return mathf.Vec2{}
	})
	if got.Width != 100 || got.Height != 20 {
		t.Fatalf("NewAtlasFrame size = (%d, %d), want (100, 20)", got.Width, got.Height)
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
