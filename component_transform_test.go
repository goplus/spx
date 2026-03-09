package spx

import "testing"

func TestTransformNormalizeAngleRange(t *testing.T) {
	var tr transformComponent
	from, to := tr.normalizeAngleRange(350, 10)
	if from != 350 || to != 370 {
		t.Fatalf("normalizeAngleRange(350, 10) = (%v, %v), want (350, 370)", from, to)
	}
}

func TestTransformCalculateBounceDirection(t *testing.T) {
	var tr transformComponent
	dx, dy := tr.calculateBounceDirection(touchingScreenRight, 0.1, 0.3)
	if dx >= 0 {
		t.Fatalf("calculateBounceDirection right edge dx = %v, want negative", dx)
	}
	if dy != 0.3 {
		t.Fatalf("calculateBounceDirection right edge dy = %v, want 0.3", dy)
	}
	if dx > -minBounceComponent {
		t.Fatalf("calculateBounceDirection right edge dx magnitude = %v, want at least %v", dx, minBounceComponent)
	}
}
