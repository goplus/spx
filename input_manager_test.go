package spx

import "testing"

func TestKeyFromStringRecognizesExclam(t *testing.T) {
	if got := KeyFromString("!"); got != KeyExclam {
		t.Fatalf("KeyFromString(\"!\") = %v, want %v", got, KeyExclam)
	}
}
