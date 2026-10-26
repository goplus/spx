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

func TestRenderFileFormatsGoSourceAndDefersWriting(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "generated.go")
	templateText := "package generated\n\nfunc Answer( )int { return 42 }\n"
	var output OutputPlan
	require.NoError(t, output.RenderFile(nil, templateText, nil, dst))
	_, err := os.Stat(dst)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, output.Commit())

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	formatted, err := format.Source(got)
	require.NoError(t, err)
	require.Equal(t, got, formatted)
	require.Contains(t, string(got), "func Answer() int")
	// Committing releases path ownership so the plan can be reused.
	require.NoError(t, output.RenderFile(nil, templateText, nil, dst))
}

func TestRenderFileRejectsInvalidOutputBeforeWriting(t *testing.T) {
	for _, test := range []struct{ name, text, want string }{
		{"Go syntax", "package generated\nfunc {", "format generated Go file"},
		{"template syntax", "{{if}}", "missing value for if"},
		{"template execution", "{{.Missing}}", "can't evaluate field Missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "generated.go")
			require.NoError(t, os.WriteFile(dst, []byte("previous generation"), 0o600))
			var output OutputPlan
			require.ErrorContains(t, output.RenderFile(nil, test.text, struct{}{}, dst), test.want)
			require.NoError(t, output.Commit())
			got, err := os.ReadFile(dst)
			require.NoError(t, err)
			require.Equal(t, "previous generation", string(got))
		})
	}
}

func TestOutputPlanRejectsDuplicatePathsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.h")
	var output OutputPlan
	require.NoError(t, output.AddFile(path, []byte("first")))
	for _, duplicate := range []string{path, dir + "/nested/../generated.h"} {
		err := output.AddFile(duplicate, []byte("second"))
		require.ErrorContains(t, err, "duplicate generated file")
	}
	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestOutputPlanSnapshotsContentsAndPreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.js")
	require.NoError(t, os.WriteFile(existing, []byte("previous"), 0o600))
	before, err := os.Stat(existing)
	require.NoError(t, err)
	var output OutputPlan
	data := []byte("snapshot")
	require.NoError(t, output.AddFile(existing, data))
	data[0] = 'X'
	got, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.Equal(t, "previous", string(got))
	require.NoError(t, output.Commit())
	got, err = os.ReadFile(existing)
	require.NoError(t, err)
	require.Equal(t, "snapshot", string(got))
	after, err := os.Stat(existing)
	require.NoError(t, err)
	require.Equal(t, before.Mode().Perm(), after.Mode().Perm())
	require.Empty(t, output.files)
	require.NoError(t, output.Commit())
}

func TestOutputPlanCreatesFilesWithUniformPermissions(t *testing.T) {
	dir := t.TempDir()
	// Compare with a reference file so the check respects the process umask
	// and the platform's permission semantics without changing global state.
	reference := filepath.Join(dir, "reference")
	require.NoError(t, os.WriteFile(reference, nil, 0o644))
	want, err := os.Stat(reference)
	require.NoError(t, err)
	var output OutputPlan
	for _, name := range []string{"generated.h", "generated.js", "generated.go"} {
		require.NoError(t, output.AddFile(filepath.Join(dir, "nested", name), []byte("contents")))
	}
	require.NoError(t, output.Commit())
	for _, name := range []string{"generated.h", "generated.js", "generated.go"} {
		got, err := os.Stat(filepath.Join(dir, "nested", name))
		require.NoError(t, err)
		require.Equal(t, want.Mode().Perm(), got.Mode().Perm(), name)
	}
}
