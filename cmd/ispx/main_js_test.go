//go:build js && wasm

package main

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/spx/v3/cmd/ispx/internal/ispxai"
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
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse generated Go: %v", err)
	}
	for _, spec := range file.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == ispxai.ModulePath {
			return
		}
	}
	t.Fatalf("generated Go is missing the ai import:\n%s", source)
}
