package ffi

import (
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

func TestProjectFontManagerChecksNativeCapability(t *testing.T) {
	common.GetManagers(clang.CHeaderFileAST{})
	function := &clang.TypedefFunction{
		Name:       "GDExtensionSpxResApplyProjectFonts",
		ReturnType: clang.PrimativeType{Name: "GdString"},
	}
	body := getManagerFuncBody(function)
	if !strings.Contains(body, "if !HasResApplyProjectFonts()") ||
		!strings.Contains(body, "loaded Godot engine does not support atomic project fonts") {
		t.Fatalf("generated manager body lacks the project-font capability guard:\n%s", body)
	}
}
