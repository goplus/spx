package spx

import "testing"

func TestParseLayerMaskValue(t *testing.T) {
	if got := parseLayerMaskValue(nil); got != 1 {
		t.Fatalf("parseLayerMaskValue(nil) = %d, want 1", got)
	}

	val := int64(8)
	if got := parseLayerMaskValue(&val); got != 8 {
		t.Fatalf("parseLayerMaskValue(&8) = %d, want 8", got)
	}
}

func TestToRotationStyle(t *testing.T) {
	tests := []struct {
		input string
		want  RotationStyle
	}{
		{input: "left-right", want: LeftRight},
		{input: "none", want: None},
		{input: "normal", want: Normal},
		{input: "bad", want: Normal},
	}

	for _, tt := range tests {
		if got := toRotationStyle(tt.input); got != tt.want {
			t.Fatalf("toRotationStyle(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
