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

package gdext

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
	"github.com/iancoleman/strcase"
)

var (
	//go:embed callback_defaults.h.tmpl
	callbackDefaultsTemplate string

	//go:embed web_callback_declarations.h.tmpl
	webCallbackDeclarationsTemplate string

	//go:embed web_callbacks.h.tmpl
	webCallbacksTemplate string
)

// Callback outputs share the ABI field list. JavaScript forwarding and
// lifecycle behavior remain hand-written.
func (g *Generator) renderCallbacks(spxModulePath string) error {
	callbacks, err := common.CollectCallbacks(g.AST())
	if err != nil {
		return fmt.Errorf("generate callbacks: %w", err)
	}
	funcs := template.FuncMap{
		"webCallbackName": func(name string) string {
			return "godot_js_spx_" + strcase.ToSnake(strings.TrimPrefix(name, "GDExtensionSpxCallback"))
		},
	}
	for _, output := range []struct{ path, source string }{
		{"spx_callback_defaults.gen.h", callbackDefaultsTemplate},
		{"web/godot_js_spx_callbacks.gen.h", webCallbackDeclarationsTemplate},
		{"web/spx_web_callbacks.gen.h", webCallbacksTemplate},
	} {
		if err := g.Output.RenderFile(funcs, output.source, callbacks, filepath.Join(spxModulePath, filepath.FromSlash(output.path))); err != nil {
			return err
		}
	}
	return nil
}
