package main

//go:generate qexp -outdir pkg github.com/goplus/spx
//go:generate qexp -outdir pkg github.com/hajimehoshi/ebiten/v2

import (
	"archive/zip"
	"bytes"
	"log"
	"syscall/js"

	_ "github.com/goplus/builder/ispx/pkg/github.com/goplus/spx"
	_ "github.com/goplus/builder/ispx/pkg/github.com/hajimehoshi/ebiten/v2"
	"github.com/goplus/builder/ispx/zipfs"
	"github.com/goplus/igop"
	"github.com/goplus/igop/gopbuild"
	_ "github.com/goplus/igop/pkg/fmt"
	_ "github.com/goplus/igop/pkg/math"
	_ "github.com/goplus/reflectx/icall/icall8192"
	spxfs "github.com/goplus/spx/fs"
)

var dataChannel = make(chan []byte)

func loadData(this js.Value, args []js.Value) interface{} {
	inputArray := args[0]

	// Convert Uint8Array to Go byte slice
	length := inputArray.Get("length").Int()
	goBytes := make([]byte, length)
	js.CopyBytesToGo(goBytes, inputArray)

	dataChannel <- goBytes
	return nil
}

func main() {
	js.Global().Set("goLoadData", js.FuncOf(loadData))

	zipData := <-dataChannel

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		log.Fatalln("Failed to read zip data:", err)
	}
	fs := zipfs.NewZipFsFromReader(zipReader)
	// Configure spx to load project files from zip-based file system.
	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) {
		return fs.Chrooted(path), nil
	})

	var mode igop.Mode
	ctx := igop.NewContext(mode)

	// NOTE(everyone): Keep sync with the config in spx [gop.mod](https://github.com/goplus/spx/blob/main/gop.mod)
	gopbuild.RegisterClassFileType(".spx", "Game", []*gopbuild.Class{{Ext: ".spx", Class: "SpriteImpl"}}, "github.com/goplus/spx")

	// Register patch for spx to support functions with generic type like `Gopt_Game_Gopx_GetWidget`.
	// See details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805
	err = gopbuild.RegisterPackagePatch(ctx, "github.com/goplus/spx", `
package spx

import (
	. "github.com/goplus/spx"
)

func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(interface{}).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
`)
	if err != nil {
		log.Fatalln("Failed to register package patch:", err)
	}

	source, err := gopbuild.BuildFSDir(ctx, fs, "")
	if err != nil {
		log.Fatalln("Failed to build Go+ source:", err)
	}
	println("code", string(source))
	code, err := ctx.RunFile("main.go", source, nil)
	if err != nil {
		log.Fatalln("Failed to run Go+ source:", err, " Code:", code)
	}
}
