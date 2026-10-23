//go:build js && wasm

package main

import (
	"testing"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/spx/v4/cmd/ispx/ai/internal/ispxai"
)

func TestWebInterpreterAutoImportsAI(t *testing.T) {
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp | xgobuild.StaticLoad)
	if err := ispxai.RegisterPatch(ctx); err != nil {
		t.Fatal(err)
	}
	// The Web entry point adds the ai import to the default SPX project.
	// Compiling a game without an explicit import exercises that registration.
	source, err := xgobuild.BuildFile(ctx, "main.spx", "var player ai.Player\n")
	if err != nil {
		t.Fatalf("compile game using the implicit ai import: %v", err)
	}
	if _, err := ctx.LoadFile("main.go", source); err != nil {
		t.Fatalf("load generated Go using the implicit ai import: %v", err)
	}
}
