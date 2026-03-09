package spx

import "testing"

func TestNormalizeEffectValue(t *testing.T) {
	if got := normalizeEffectValue(BrightnessEffect, 200); got != 1 {
		t.Fatalf("normalizeEffectValue brightness = %v, want 1", got)
	}
	if got := normalizeEffectValue(GhostEffect, 50); got != 0.5 {
		t.Fatalf("normalizeEffectValue ghost = %v, want 0.5", got)
	}
	if got := normalizeEffectValue(MosaicEffect, 15); got != 2 {
		t.Fatalf("normalizeEffectValue mosaic = %v, want 2", got)
	}
	if got := normalizeEffectValue(ColorEffect, -100); got != 0.5 {
		t.Fatalf("normalizeEffectValue color = %v, want 0.5", got)
	}
}
