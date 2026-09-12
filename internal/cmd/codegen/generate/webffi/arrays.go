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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

var (
	//go:embed arrays.go.tmpl
	goArraysTemplate string

	//go:embed arrays.js.tmpl
	jsArraysTemplate string
)

var arrayTemplateFuncs = template.FuncMap{
	"arrayTag": func() string { return common.ArrayTag },
}

func writeGoArrays(projectPath string) error {
	return common.GenerateFile(arrayTemplateFuncs, "arrays.gen.go", goArraysTemplate, common.ArrayTypes(),
		filepath.Join(projectPath, WebRelDir, "arrays.gen.go"))
}

func writeJSArrays(spxModulePath string) error {
	path := filepath.Join(spxModulePath, "web", "js", "engine", "gdspx.util.js")
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	const start = "// BEGIN GENERATED ARRAY TYPES\n"
	const end = "// END GENERATED ARRAY TYPES\n"
	before, rest, ok := strings.Cut(string(source), start)
	if !ok {
		return fmt.Errorf("missing array types start marker in %s", path)
	}
	_, after, ok := strings.Cut(rest, end)
	if !ok {
		return fmt.Errorf("missing array types end marker in %s", path)
	}
	output, err := common.RenderTemplate(arrayTemplateFuncs, "arrays.js", jsArraysTemplate, common.ArrayTypes())
	if err != nil {
		return err
	}
	return common.WriteGeneratedFile(path, []byte(before+start+string(output)+end+after), 0o644)
}
