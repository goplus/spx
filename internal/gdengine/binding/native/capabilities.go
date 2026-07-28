//go:build !js && !wasm

package ffi

// HasResApplyProjectFonts reports whether the loaded Godot engine exports the
// atomic project-font API. A missing symbol must not be treated as an empty
// success string by the generated return-value bridge.
func HasResApplyProjectFonts() bool {
	return api.SpxResApplyProjectFonts != nil
}
