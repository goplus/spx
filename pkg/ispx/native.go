//go:build !js || !wasm

package ispx

// defaultIXGoContextLookup is the default [ixgo.Context.Lookup] for native
// platforms. It always returns not found.
func defaultIXGoContextLookup(root, path string) (dir string, found bool) { return }
