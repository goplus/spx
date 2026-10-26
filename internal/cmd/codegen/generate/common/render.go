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
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/goplus/spx/v4/internal/base/licenseheader"
	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

var codegenLogger = spxlog.NewWithOutputs("SPX-CODEGEN", spxlog.LevelInfo, os.Stdout, os.Stderr)

const (
	NativeRelDir       = "../../gdengine/binding/native"
	GDEngineImplRelDir = "../../gdengine/impl"
	EngineWrapRelDir   = "../../enginewrap"
	EnginePkgRelDir    = "../../../pkg/spx/pkg/engine"
)

// OutputPlan defers publication until every generator has completed. Its zero
// value is ready to use; only Commit writes files.
type OutputPlan struct {
	files []generatedFile
}

type generatedFile struct {
	path string
	data []byte
}

// RenderTemplate completes template execution before any output file is opened.
func RenderTemplate(funcs template.FuncMap, name, text string, data any) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(funcs).Parse(text)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// GoEnumValue preserves C's increment rule after explicit values and aliases.
func GoEnumValue(values []clang.EnumValue, index int) string {
	v := values[index]
	if v.IntValue != nil {
		return strconv.Itoa(*v.IntValue)
	} else if v.ConstRefValue != nil {
		return *v.ConstRefValue
	} else if index == 0 {
		return "0"
	}
	return values[index-1].Name + " + 1"
}

func Add(a int, b int) int {
	return a + b
}

func Sub(a int, b int) int {
	return a - b
}

// RenderFile renders and formats an output before adding it to the plan.
func (p *OutputPlan) RenderFile(funcs template.FuncMap, text string, data any, dstPath string) error {
	output, err := RenderTemplate(funcs, filepath.Base(dstPath), text, data)
	if err != nil {
		return err
	}
	if filepath.Ext(dstPath) == ".go" {
		output = licenseheader.AddToGoSource(output)
		output, err = format.Source(output)
		if err != nil {
			return fmt.Errorf("format generated Go file %q: %w", dstPath, err)
		}
	}
	return p.AddFile(dstPath, output)
}

// AddFile snapshots an output and rejects conflicting ownership of its path.
func (p *OutputPlan) AddFile(path string, data []byte) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve generated file path: %w", err)
	}
	for _, file := range p.files {
		if file.path == path {
			return fmt.Errorf("duplicate generated file %q", path)
		}
	}
	p.files = append(p.files, generatedFile{path, bytes.Clone(data)})
	return nil
}

// Commit writes the completed outputs in generation order. Rendering failures
// cannot affect existing files; filesystem failures during Commit may leave a
// partially published set, and are returned to the caller.
func (p *OutputPlan) Commit() error {
	for _, file := range p.files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			return fmt.Errorf("create generated file directory %q: %w", filepath.Dir(file.path), err)
		}
		if err := os.WriteFile(file.path, file.data, 0o644); err != nil {
			return fmt.Errorf("write generated file %q: %w", file.path, err)
		}
		codegenLogger.Info("Generated file: %s", file.path)
	}
	p.files = nil
	return nil
}
