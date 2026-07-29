//go:build cgo && !js && !pure_engine

package impl

import "testing"

func TestApplyProjectFontsRejectsMissingEngineSymbol(t *testing.T) {
	const want = "loaded Godot engine does not support atomic project fonts"
	if got := (&resMgr{}).ApplyProjectFonts("res://font.ttf", []string{}, []string{}, []string{}); got != want {
		t.Fatalf("ApplyProjectFonts() = %q, want %q", got, want)
	}
}
