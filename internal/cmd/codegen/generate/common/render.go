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

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/internal/licenseheader"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"
)

const (
	NativeRelDir       = "../../gdengine/binding/native"
	GdengineImplRelDir = "../../gdengine/impl"
	EnginewrapRelDir   = "../../enginewrap"
	EnginePkgRelDir    = "../../../pkg/spx/pkg/engine"
)

func Add(a int, b int) int {
	return a + b
}

func Sub(a int, b int) int {
	return a - b
}

func GoEnumValue(v clang.EnumValue, index int) string {
	if v.IntValue != nil {
		return strconv.Itoa(*v.IntValue)
	} else if v.ConstRefValue != nil {
		return *v.ConstRefValue
	} else if index == 0 {
		return "iota"
	} else {
		return ""
	}
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

func WriteGeneratedFile(dstPath string, output []byte, mode os.FileMode) error {
	if err := os.WriteFile(dstPath, output, mode); err != nil {
		return fmt.Errorf("write generated file %q: %w", dstPath, err)
	}
	spxlog.Info("Generated file: %s", dstPath)
	return nil
}

func GenerateFile(funcs template.FuncMap, name string, text string, data any, dstPath string) error {
	output, err := RenderTemplate(funcs, name, text, data)
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
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create generated file directory %q: %w", filepath.Dir(dstPath), err)
	}
	return WriteGeneratedFile(dstPath, output, 0o644)
}
