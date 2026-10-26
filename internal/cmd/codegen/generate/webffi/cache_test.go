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

package webffi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestCacheFollowsFunctions(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "binding[one]")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	neighbor := filepath.Join(root, "bindingo")
	require.NoError(t, os.MkdirAll(neighbor, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(neighbor, "other_cache.go"), []byte("invalid neighboring cache"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ordinary.go"), []byte("not a cache source"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(`package webffi
 func CachedExampleReadValue(value string, fallback func() bool) bool { return fallback() }
 `), 0o600))
	caches, err := scanCaches(dir)
	require.NoError(t, err)
	require.Len(t, caches, 1)
	ast, err := clang.ParseCString("typedef GdBool (*GDExtensionSpxExampleReadValue)(GdString renamed);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	functions := ast.CollectGDExtensionInterfaceFunctions()
	body := generation.cachedManagerBody(&functions[0], caches)
	require.Contains(t, body, "CachedExampleReadValue(renamed, func() bool")
	require.Contains(t, body, "API.SpxExampleReadValue.Invoke(arg0)")

	// API names alone do not enable caching.
	ast, err = clang.ParseCString("typedef GdBool (*GDExtensionSpxInputGetKey)(GdInt key);")
	require.NoError(t, err)
	generation.GenerationContext = common.NewGenerationContext(ast, common.GenerationMetadata{})
	functions = ast.CollectGDExtensionInterfaceFunctions()
	body = generation.cachedManagerBody(&functions[0], caches)
	require.NotContains(t, body, "CachedInput")
	require.Contains(t, body, "API.SpxInputGetKey.Invoke(")
}

func TestCacheRejectsInvalidDeclarations(t *testing.T) {
	for _, test := range []struct{ signature, want string }{
		{"Read() bool", "requires arguments"},
		{"Read(value bool) bool", "requires a final func() T fallback"},
		{"Read(fallback func()) bool", "requires a final func() T fallback"},
		{"Read(fallback func(int) bool) bool", "requires a final func() T fallback"},
		{"Read(fallback func() bool)", "requires arguments"},
		{"Read(", "expected"},
		{"Read(fallback func() bool) bool {}\nfunc CachedExampleRead(fallback func() bool) bool", "duplicate cache function"},
	} {
		t.Run(test.signature, func(t *testing.T) {
			dir := t.TempDir()
			source := "package webffi; func CachedExample" + test.signature + " {}"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(source), 0o600))
			_, err := scanCaches(dir)
			require.ErrorContains(t, err, test.want)
		})
	}
	_, err := scanCaches(filepath.Join(t.TempDir(), "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCacheRejectsMissingAPI(t *testing.T) {
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	dir := filepath.Join(codegenDir, WebRelDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	source := "package webffi; func CachedExampleRemoved(fallback func() bool) bool { return fallback() }"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(source), 0o600))
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}
	require.ErrorContains(t, generation.renderManager(codegenDir), "cache function has no matching API: GDExtensionSpxExampleRemoved")
}
