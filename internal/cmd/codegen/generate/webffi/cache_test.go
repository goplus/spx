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

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestCacheFollowsFunctions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(`package webffi
 func CachedExampleReadValue(value string, fallback func() bool) bool { return fallback() }
 `), 0o600))
	caches, err := scanCaches(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleReadValue)(GdString renamed, GdBool ret_value);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{}), caches: caches}
	functions := ast.CollectGDExtensionInterfaceFunctions()
	body := generation.managerBody(&functions[0])
	require.Contains(t, body, "CachedExampleReadValue(renamed, func() bool")
	require.Contains(t, body, "API.SpxExampleReadValue.Invoke(arg0)")

	// API names alone do not enable caching.
	ast, err = clang.ParseCString("typedef void (*GDExtensionSpxInputGetKey)(GdInt key, GdBool ret_value);")
	require.NoError(t, err)
	generation.GenerationContext = common.NewGenerationContext(ast, common.GenerationMetadata{})
	functions = ast.CollectGDExtensionInterfaceFunctions()
	body = generation.managerBody(&functions[0])
	require.NotContains(t, body, "CachedInput")
	require.Contains(t, body, "API.SpxInputGetKey.Invoke(")
}

func TestCacheRejectsInvalidFallback(t *testing.T) {
	for _, signature := range []string{
		"Read() bool", "Read(value bool) bool", "Read(fallback func()) bool",
		"Read(fallback func(int) bool) bool", "Read(fallback func() bool)",
	} {
		t.Run(signature, func(t *testing.T) {
			dir := t.TempDir()
			source := "package webffi; func CachedExample" + signature + " {}"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(source), 0o600))
			_, err := scanCaches(dir)
			require.Error(t, err)
		})
	}
}

func TestCacheRejectsMissingAPI(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	dir := filepath.Join(projectPath, WebRelDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	source := "package webffi; func CachedExampleRemoved(fallback func() bool) bool { return fallback() }"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "example_cache.go"), []byte(source), 0o600))
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}
	require.ErrorContains(t, generation.writeManager(projectPath), "cache function has no matching API: GDExtensionSpxExampleRemoved")
}
