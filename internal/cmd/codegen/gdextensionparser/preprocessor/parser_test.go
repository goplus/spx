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

package preprocessor

import (
	"testing"

	_ "embed"

	"github.com/stretchr/testify/require"
)

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

	ast, err := ParsePreprocessorString(content)

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
	require.Equal(t, "\n\ntypedef uint32_t char32_t;\ntypedef uint16_t char16_t;\n\n\n\ntypedef void *GDExtensionVariantPtr;\n\n\n\n\n", ast.Eval(false))
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
	ast, err := ParsePreprocessorString(content)

	require.NoError(t, err)

	require.Len(t, ast.Directives, 1)
	require.NotNil(t, ast.Directives[0].Ifndef)
	require.Equal(t, "REGRESSION_H", ast.Directives[0].Ifndef.Name)
	require.Len(t, ast.Directives[0].Ifndef.Directives, 1)
	require.Equal(t, "const i int = 5;\n\tconst j int = 6;\n\tconst k int = 7;\n\n\t/* typed arrays */\n\tconst x int = 1;\n\tconst y int = 2;\n\n\t", ast.Directives[0].Ifndef.Directives[0].Source)
	require.Equal(t, "const i int = 5;\n\tconst j int = 6;\n\tconst k int = 7;\n\n\t/* typed arrays */\n\tconst x int = 1;\n\tconst y int = 2;\n\n\t\n\n", ast.Eval(false))
}

func TestConditionalDirectiveEvaluation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive Directive
		vars      PreprocVars
		want      string
		wantVars  PreprocVars
	}{
		{
			name: "nested definitions affect later siblings",
			directive: Directive{Ifndef: &IfndefDirective{Name: "ROOT", Directives: []Directive{
				{Define: &DefineDirective{Name: "READY"}},
				{Ifdef: &IfdefDirective{Name: "READY", Directives: []Directive{{Source: "ready"}}}},
				{Ifndef: &IfndefDirective{Name: "READY", Directives: []Directive{{Source: "hidden"}}}},
			}}},
			vars: PreprocVars{}, want: "\nready\n\n\n", wantVars: PreprocVars{"READY": {}},
		},
		{
			name: "disabled ifdef skips definitions and invalid children",
			directive: Directive{Ifdef: &IfdefDirective{Name: "MISSING", Directives: []Directive{
				{Define: &DefineDirective{Name: "HIDDEN"}},
				{Ifndef: &IfndefDirective{}},
			}}},
			vars: PreprocVars{}, wantVars: PreprocVars{},
		},
		{
			name: "disabled ifndef skips definitions",
			directive: Directive{Ifndef: &IfndefDirective{Name: "READY", Directives: []Directive{
				{Define: &DefineDirective{Name: "HIDDEN"}},
			}}},
			vars: PreprocVars{"READY": {}}, wantVars: PreprocVars{"READY": {}},
		},
		{
			name:      "repeated definitions preserve existing variables",
			directive: Directive{Define: &DefineDirective{Name: "READY"}},
			vars:      PreprocVars{"READY": {}, "OTHER": {}}, wantVars: PreprocVars{"READY": {}, "OTHER": {}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.directive.Eval(tt.vars))
			require.Equal(t, tt.wantVars, tt.vars)
		})
	}
}

func TestPreprocessorEvalInitialVariables(t *testing.T) {
	ast := PreprocessorHeaderFileAST{Directives: []Directive{
		{Ifndef: &IfndefDirective{Name: "SEEN", Directives: []Directive{
			{Source: "first"},
			{Define: &DefineDirective{Name: "SEEN"}},
		}}},
		{Ifdef: &IfdefDirective{Name: "__cplusplus", Directives: []Directive{{Source: "cpp"}}}},
	}}
	// Definitions from either language mode must not survive the next evaluation.
	require.Equal(t, "first\n\n\n\n", ast.Eval(false))
	require.Equal(t, "first\n\n\ncpp\n\n", ast.Eval(true))
	require.Equal(t, "first\n\n\n\n", ast.Eval(false))
}

func TestPreprocessorEvalEmptyDirectives(t *testing.T) {
	require.Empty(t, (PreprocessorHeaderFileAST{}).Eval(false))
	ast := PreprocessorHeaderFileAST{Directives: []Directive{
		{}, {Include: &IncludeDirective{Name: "test.h"}}, {Source: "source\n"},
	}}
	require.Equal(t, "\n\nsource\n\n", ast.Eval(false))
}

func TestDirectiveMissingNames(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive Directive
	}{
		{"ifndef", Directive{Ifndef: &IfndefDirective{}}},
		{"ifdef", Directive{Ifdef: &IfdefDirective{}}},
		{"define", Directive{Define: &DefineDirective{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, "#"+tt.name+" missing variable", func() { tt.directive.Eval(PreprocVars{}) })
		})
	}
}
