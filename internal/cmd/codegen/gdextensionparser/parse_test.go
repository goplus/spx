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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestGenerateGDExtensionInterfaceAST(t *testing.T) {
	root := t.TempDir()
	nativeDir := filepath.Join(root, "internal", "gdengine", "binding", "native")
	require.NoError(t, os.MkdirAll(nativeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nativeDir, "gdextension_spx_codegen_header.h"), []byte("#include \"api.h\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(nativeDir, "api.h"), []byte(`
#ifndef SPX_TEST_API_H
#define SPX_TEST_API_H
typedef void (*GDExtensionSpxExampleRun)(int value);
#endif
`), 0o600))
	debugPath := filepath.Join(root, "parsed.json")
	parsed, err := GenerateGDExtensionInterfaceAST(filepath.Join(root, "internal", "cmd", "codegen"), debugPath)
	require.NoError(t, err)
	functions := parsed.CollectGDExtensionInterfaceFunctions()
	require.Len(t, functions, 1)
	require.Equal(t, "GDExtensionSpxExampleRun", functions[0].Name)
	require.Equal(t, "int", functions[0].Arguments[0].Type.Primitive.Name)
	expanded, err := os.ReadFile(filepath.Join(nativeDir, "_temp_output.h"))
	require.NoError(t, err)
	require.Contains(t, string(expanded), "GDExtensionSpxExampleRun")
	debug, err := os.ReadFile(debugPath)
	require.NoError(t, err)
	var saved clang.CHeaderFileAST
	require.NoError(t, json.Unmarshal(debug, &saved))
	require.Equal(t, parsed, saved)
}
