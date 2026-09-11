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

package common

import (
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateFileFormatsGoSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "generated.go")
	templateText := "package generated\n\nfunc Answer( )int { return 42 }\n"
	require.NoError(t, GenerateFile(nil, "generated.go", templateText, nil, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	formatted, err := format.Source(got)
	require.NoError(t, err)
	require.Equal(t, got, formatted)
	require.Contains(t, string(got), "func Answer() int")
}

func TestGenerateFileRejectsInvalidGoBeforeWriting(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "nested", "generated.go")
	err := GenerateFile(nil, "generated.go", "package generated\nfunc {", nil, dst)
	require.ErrorContains(t, err, "format generated Go file")
	_, statErr := os.Stat(dst)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
