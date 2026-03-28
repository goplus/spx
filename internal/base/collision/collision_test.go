package collision

import "testing"

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
	if got := ParsePixelCollisionPrecision(nil); got != PixelPrecisionLow {
		t.Fatalf("ParsePixelCollisionPrecision(nil) = %d, want %d", got, PixelPrecisionLow)
	}
	tests := []struct {
		input string
		want  int64
	}{
		{input: "high", want: PixelPrecisionHigh},
		{input: "medium", want: PixelPrecisionMedium},
		{input: "low", want: PixelPrecisionLow},
		{input: "bad", want: PixelPrecisionLow},
	}
	for _, tt := range tests {
		input := tt.input
		if got := ParsePixelCollisionPrecision(&input); got != tt.want {
			t.Fatalf("ParsePixelCollisionPrecision(%q) = %d, want %d", input, got, tt.want)
		}
	}
}
