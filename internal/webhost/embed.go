package webhost

import (
	"embed"
	"io/fs"
)

var (
	//go:embed static/*
	embeddedAssets embed.FS

	// Assets contains the web host runtime assets used to bridge the
	// engine runtime and the ispx wasm runtime in web exports.
	Assets = mustSubFS(embeddedAssets, "static")
)

// mustSubFS returns the subtree rooted at dir and panics if it cannot be created.
func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
