/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gdext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestCallbackDefaultsFollowABIFieldsAndTypes(t *testing.T) {
	ast, err := clang.ParseCString(`
typedef void (*GDExtensionSpxCallbackReady)();
typedef void (*GDExtensionSpxCallbackChanged)(GdObj obj, GdString value);
typedef struct {
 GDExtensionSpxCallbackChanged custom_field;
 GDExtensionSpxCallbackReady func_on_ready;
} SpxCallbackInfo;`)
	require.NoError(t, err)
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	dir := t.TempDir()
	require.NoError(t, generation.renderCallbacks(dir))
	require.NoError(t, generation.Output.Commit())
	output, err := os.ReadFile(filepath.Join(dir, "spx_callback_defaults.gen.h"))
	require.NoError(t, err)
	require.Contains(t, string(output), "SpxCallbackInfo callbacks = {};")
	require.Contains(t, string(output), "callbacks.custom_field = [](GdObj, GdString) {};")
	require.Contains(t, string(output), "callbacks.func_on_ready = []() {};")
}

// Structural ABI validation is covered by common; this checks the generator boundary.
func TestCallbacksPropagateABIError(t *testing.T) {
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}
	require.EqualError(t, generation.renderCallbacks(t.TempDir()), "generate callbacks: missing SpxCallbackInfo")
}

func TestWebCallbacksFollowNewABIFieldsAndSignatures(t *testing.T) {
	// A callback absent from the real ABI must need no additional name list.
	// Its field name deliberately differs from its function type name.
	abi := `
typedef void (*GDExtensionSpxCallbackOnUnused)(GdInt key);
typedef void (*GDExtensionSpxCallbackOnAlternate)(GdObj obj, GdString text, GdInt key, GdFloat delta, GdBool enabled);
typedef void (*GDExtensionSpxCallbackOnAddedForTest)(GdObj obj, GdString text, GdInt key, GdFloat delta, GdBool enabled);
typedef void (*GDExtensionSpxCallbackOnReady)();
typedef struct {
 GDExtensionSpxCallbackOnAddedForTest custom_field;
 GDExtensionSpxCallbackOnReady func_on_ready;
 GDExtensionSpxCallbackOnAlternate alternate_field;
} SpxCallbackInfo;`
	dir := t.TempDir()
	for _, deltaType := range []string{"GdFloat", "GdInt"} {
		t.Run(deltaType, func(t *testing.T) {
			source := strings.ReplaceAll(abi, "GdFloat delta", deltaType+" delta")
			ast, err := clang.ParseCString(source)
			require.NoError(t, err)
			generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
			require.NoError(t, generation.renderCallbacks(dir))
			require.NoError(t, generation.Output.Commit())
			declarations, err := os.ReadFile(filepath.Join(dir, "web", "godot_js_spx_callbacks.gen.h"))
			require.NoError(t, err)
			require.Contains(t, string(declarations), "extern void godot_js_spx_on_added_for_test(GdObj obj, GdString text, GdInt key, "+deltaType+" delta, GdBool enabled);")
			require.Contains(t, string(declarations), "extern void godot_js_spx_on_ready(void);")
			require.NotContains(t, string(declarations), "on_unused")
			table, err := os.ReadFile(filepath.Join(dir, "web", "spx_web_callbacks.gen.h"))
			require.NoError(t, err)
			require.Contains(t, string(table), "callbacks.custom_field = &godot_js_spx_on_added_for_test;")
			require.Contains(t, string(table), "callbacks.func_on_ready = &godot_js_spx_on_ready;")

			// Compile the generated table against the original typedefs: this
			// detects drift in argument types, order and empty parameter lists.
			types := "#pragma once\n#include <stdint.h>\ntypedef int64_t GdObj;\ntypedef const void *GdString;\ntypedef int64_t GdInt;\ntypedef float GdFloat;\ntypedef uint8_t GdBool;\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gdextension_spx_ext.h"), []byte(types+source), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "web", "godot_js_spx.h"), []byte(types+"#include \"godot_js_spx_callbacks.gen.h\"\n"), 0o600))
			compileCallbackHeader(t, "c", dir, "#include \"web/godot_js_spx.h\"\n")
			compileCallbackHeader(t, "c++", dir, "#include \"web/spx_web_callbacks.gen.h\"\n")
			// Execute two same-signature entries so swapped registrations cannot
			// pass on type checking alone; also exercise the fallback table.
			program := strings.ReplaceAll(`
#include <cassert>
#include "web/spx_web_callbacks.gen.h"
#include "spx_callback_defaults.gen.h"
static int added = 0, alternate = 0, ready = 0;
static const char text[] = "callback payload";
extern "C" void godot_js_spx_on_added_for_test(GdObj obj, GdString value, GdInt key, DELTA_TYPE delta, GdBool enabled) {
 assert(obj == INT64_MAX && value == text && key == INT64_MIN && delta == 3 && enabled == 1);
 ++added;
}
extern "C" void godot_js_spx_on_alternate(GdObj obj, GdString value, GdInt key, DELTA_TYPE delta, GdBool enabled) {
 assert(obj == INT64_MIN && value == text && key == INT64_MAX && delta == -2 && enabled == 0);
 ++alternate;
}
extern "C" void godot_js_spx_on_ready(void) { ++ready; }
int main() {
 auto defaults = get_default_spx_callbacks();
 defaults.custom_field(INT64_MAX, text, INT64_MIN, 3, 1);
 defaults.alternate_field(INT64_MIN, text, INT64_MAX, -2, 0);
 defaults.func_on_ready();
 assert(added == 0 && alternate == 0 && ready == 0);
 auto callbacks = get_web_spx_callbacks();
 callbacks.custom_field(INT64_MAX, text, INT64_MIN, 3, 1);
 callbacks.alternate_field(INT64_MIN, text, INT64_MAX, -2, 0);
 callbacks.func_on_ready();
 assert(added == 1 && alternate == 1 && ready == 1);
}
`, "DELTA_TYPE", deltaType)
			executable := filepath.Join(dir, "callbacks_test")
			command := exec.Command("c++", "-x", "c++", "-std=c++17", "-Wall", "-Wextra", "-Werror", "-I", dir, "-o", executable, "-")
			command.Stdin = strings.NewReader(program)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
			output, err = exec.Command(executable).CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func TestWebCallbackHeadersCompileAgainstPublishedABI(t *testing.T) {
	module := filepath.Join("..", "..", "..", "..", "..", "godot_modules", "spx")
	compileCallbackHeader(t, "c", module, "#include \"web/godot_js_spx.h\"\n")
	compileCallbackHeader(t, "c++", module, "#include \"web/tests/stubs/godot_types.h\"\n#include \"web/spx_web_callbacks.gen.h\"\n", filepath.Join(module, "web", "tests", "stubs"))
}

func compileCallbackHeader(t *testing.T, language, includeDir, source string, extraIncludes ...string) {
	t.Helper()
	compiler, standard := "cc", "c11"
	if language == "c++" {
		compiler, standard = "c++", "c++17"
	}
	path, err := exec.LookPath(compiler)
	if err != nil {
		t.Skipf("%s unavailable for callback header compile check: %v", compiler, err)
	}
	args := []string{"-x", language, "-std=" + standard, "-Wall", "-Wextra", "-Werror", "-fsyntax-only", "-I", includeDir, "-"}
	for _, include := range extraIncludes {
		args = append(args, "-I", include)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = strings.NewReader(source)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}
