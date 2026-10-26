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

package gdextensionparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGDExtensionInterfaceUsesSourceHeaderWithoutWriting(t *testing.T) {
	root := t.TempDir()
	nativeDir := filepath.Join(root, "internal", "gdengine", "binding", "native")
	require.NoError(t, os.MkdirAll(nativeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nativeDir, "gdextension_spx_codegen_header.h"), []byte("#include \"gdextension_spx_ext.h\"\n"), 0o600))
	const published = "published lowered ABI must remain unchanged\n"
	header := filepath.Join(nativeDir, "gdextension_spx_ext.h")
	require.NoError(t, os.WriteFile(header, []byte(published), 0o600))
	parsed, err := ParseGDExtensionInterface(nativeDir, `
#ifndef SPX_TEST_API_H
#define SPX_TEST_API_H
typedef void (*GDExtensionSpxExampleRun)(int value);
#endif
`)
	require.NoError(t, err)
	functions := parsed.CollectGDExtensionInterfaceFunctions()
	require.Len(t, functions, 1)
	require.Equal(t, "GDExtensionSpxExampleRun", functions[0].Name)
	require.Equal(t, "int", functions[0].Arguments[0].Type.Primitive.Name)
	got, err := os.ReadFile(header)
	require.NoError(t, err)
	require.Equal(t, published, string(got))
	entries, err := os.ReadDir(nativeDir)
	require.NoError(t, err)
	require.Len(t, entries, 2, "parsing must not write scratch headers or diagnostics")
	// A wrong header directory must fail instead of finding an ancestor's
	// project and reading inputs from a different root than generated outputs.
	_, err = ParseGDExtensionInterface(filepath.Join(root, "nested"), "")
	require.ErrorIs(t, err, os.ErrNotExist)
}
