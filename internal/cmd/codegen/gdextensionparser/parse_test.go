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

func TestPreprocessorParseHeaderFile(t *testing.T) {
	content := `/*******
* test *
********/

#ifndef MYFILE_H
#define MYFILE_H
#include <test.h>

#ifndef __cplusplus
typedef uint32_t char32_t;
typedef uint16_t char16_t;
#endif

#ifdef __cplusplus
extern "C" {
#endif

typedef void *GDExtensionVariantPtr;

#ifdef __cplusplus
}
#endif
#endif // MYFILE_H
`

	ast, err := parsePreprocessor(content)

	require.NoError(t, err)
	require.Len(t, ast.Directives, 1)
	require.NotNil(t, ast.Directives[0].Ifndef)
	require.Equal(t, "MYFILE_H", ast.Directives[0].Ifndef.Name)
	require.Len(t, ast.Directives[0].Ifndef.Directives, 6)
	require.NotNil(t, ast.Directives[0].Ifndef.Directives[0].Define)
	require.Equal(t, "MYFILE_H", ast.Directives[0].Ifndef.Directives[0].Define.Name)
	require.NotNil(t, ast.Directives[0].Ifndef.Directives[1].Include)
	require.NotNil(t, ast.Directives[0].Ifndef.Directives[2].Ifndef)
	require.Equal(t, "__cplusplus", ast.Directives[0].Ifndef.Directives[2].Ifndef.Name)
	require.NotNil(t, ast.Directives[0].Ifndef.Directives[3].Ifdef)
	require.Equal(t, "__cplusplus", ast.Directives[0].Ifndef.Directives[3].Ifdef.Name)
	require.Len(t, ast.Directives[0].Ifndef.Directives[3].Ifdef.Directives, 1)
	require.Equal(t, "extern \"C\" {\n", ast.Directives[0].Ifndef.Directives[3].Ifdef.Directives[0].Source)
	require.Equal(t, "typedef void *GDExtensionVariantPtr;\n\n", ast.Directives[0].Ifndef.Directives[4].Source)
	require.NotNil(t, ast.Directives[0].Ifndef.Directives[5].Ifdef)
	require.Len(t, ast.Directives[0].Ifndef.Directives[5].Ifdef.Directives, 1)
	require.Equal(t, "__cplusplus", ast.Directives[0].Ifndef.Directives[5].Ifdef.Name)
	require.Equal(t, "}\n", ast.Directives[0].Ifndef.Directives[5].Ifdef.Directives[0].Source)
	require.Equal(t, "\n\ntypedef uint32_t char32_t;\ntypedef uint16_t char16_t;\n\n\n\ntypedef void *GDExtensionVariantPtr;\n\n\n\n\n", ast.eval(false))
}

func TestParseCommentRegression(t *testing.T) {
	content := `/*******
	* test *
	********/

	#ifndef REGRESSION_H
	/* misc types */
	const i int = 5;
	const j int = 6;
	const k int = 7;

	/* typed arrays */
	const x int = 1;
	const y int = 2;

	#endif // REGRESSION_H
`
	ast, err := parsePreprocessor(content)

	require.NoError(t, err)

	require.Len(t, ast.Directives, 1)
	require.NotNil(t, ast.Directives[0].Ifndef)
	require.Equal(t, "REGRESSION_H", ast.Directives[0].Ifndef.Name)
	require.Len(t, ast.Directives[0].Ifndef.Directives, 1)
	require.Equal(t, "const i int = 5;\n\tconst j int = 6;\n\tconst k int = 7;\n\n\t/* typed arrays */\n\tconst x int = 1;\n\tconst y int = 2;\n\n\t", ast.Directives[0].Ifndef.Directives[0].Source)
	require.Equal(t, "const i int = 5;\n\tconst j int = 6;\n\tconst k int = 7;\n\n\t/* typed arrays */\n\tconst x int = 1;\n\tconst y int = 2;\n\n\t\n\n", ast.eval(false))
}

func TestPreprocessorIncludesAcceptDigitLeadingNames(t *testing.T) {
	for _, filename := range []string{"1.h", "1_name.h"} {
		t.Run(filename, func(t *testing.T) {
			ast, err := parsePreprocessor("#include <" + filename + ">\n")
			require.NoError(t, err)
			require.Len(t, ast.Directives, 1)
			require.NotNil(t, ast.Directives[0].Include)
			require.Equal(t, filename, ast.Directives[0].Include.Name)
			require.Equal(t, "\n", ast.eval(false))
		})
	}
}

func TestPreprocessorRejectsMalformedDirectives(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{"unterminated conditional", "#ifdef READY\nint value;\n"},
		{"missing definition name", "#define\n"},
		{"unsupported include form", "#include \"local.h\"\n"},
		{"unsupported numeric include form", "#include \"1.h\"\n"},
		{"unsupported directive", "#undef READY\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := parsePreprocessor(tt.source)
			require.Error(t, err)
			require.Nil(t, ast, "invalid headers must not expose a partially parsed AST")
		})
	}
}

func TestConditionalDirectiveEvaluation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive directive
		vars      preprocessorVars
		want      string
		wantVars  preprocessorVars
	}{
		{
			name: "nested definitions affect later siblings",
			directive: directive{Ifndef: &ifndefDirective{Name: "ROOT", Directives: []directive{
				{Define: &defineDirective{Name: "READY"}},
				{Ifdef: &ifdefDirective{Name: "READY", Directives: []directive{{Source: "ready"}}}},
				{Ifndef: &ifndefDirective{Name: "READY", Directives: []directive{{Source: "hidden"}}}},
			}}},
			vars: preprocessorVars{}, want: "\nready\n\n\n", wantVars: preprocessorVars{"READY": {}},
		},
		{
			name: "disabled ifdef skips definitions and invalid children",
			directive: directive{Ifdef: &ifdefDirective{Name: "MISSING", Directives: []directive{
				{Define: &defineDirective{Name: "HIDDEN"}},
				{Ifndef: &ifndefDirective{}},
			}}},
			vars: preprocessorVars{}, wantVars: preprocessorVars{},
		},
		{
			name: "disabled ifndef skips definitions",
			directive: directive{Ifndef: &ifndefDirective{Name: "READY", Directives: []directive{
				{Define: &defineDirective{Name: "HIDDEN"}},
			}}},
			vars: preprocessorVars{"READY": {}}, wantVars: preprocessorVars{"READY": {}},
		},
		{
			name:      "repeated definitions preserve existing variables",
			directive: directive{Define: &defineDirective{Name: "READY"}},
			vars:      preprocessorVars{"READY": {}, "OTHER": {}}, wantVars: preprocessorVars{"READY": {}, "OTHER": {}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.directive.eval(tt.vars))
			require.Equal(t, tt.wantVars, tt.vars)
		})
	}
}

func TestPreprocessorEvalInitialVariables(t *testing.T) {
	ast := preprocessorFile{Directives: []directive{
		{Ifndef: &ifndefDirective{Name: "SEEN", Directives: []directive{
			{Source: "first"},
			{Define: &defineDirective{Name: "SEEN"}},
		}}},
		{Ifdef: &ifdefDirective{Name: "__cplusplus", Directives: []directive{{Source: "cpp"}}}},
	}}
	// Definitions from either language mode must not survive the next evaluation.
	require.Equal(t, "first\n\n\n\n", ast.eval(false))
	require.Equal(t, "first\n\n\ncpp\n\n", ast.eval(true))
	require.Equal(t, "first\n\n\n\n", ast.eval(false))
}

func TestPreprocessorEvalEmptyDirectives(t *testing.T) {
	require.Empty(t, (preprocessorFile{}).eval(false))
	ast := preprocessorFile{Directives: []directive{
		{}, {Include: &includeDirective{Name: "test.h"}}, {Source: "source\n"},
	}}
	require.Equal(t, "\n\nsource\n\n", ast.eval(false))
}

func TestDirectiveMissingNames(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive directive
	}{
		{"ifndef", directive{Ifndef: &ifndefDirective{}}},
		{"ifdef", directive{Ifdef: &ifdefDirective{}}},
		{"define", directive{Define: &defineDirective{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, "#"+tt.name+" missing variable", func() { tt.directive.eval(preprocessorVars{}) })
		})
	}
}
