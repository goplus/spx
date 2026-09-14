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
	"regexp"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

type bindingOptions struct {
	Web common.WebBindingMode
}

var reBinding = regexp.MustCompile(`^SPX_BINDING\s*\(([^()]*)\)`)

// parseBinding consumes an optional annotation and leaves the method declaration.
func parseBinding(line string) (*bindingOptions, string) {
	if !strings.HasPrefix(line, "SPX_BINDING") {
		return nil, line
	}
	match := reBinding.FindStringSubmatch(line)
	if match == nil {
		panic("invalid SPX_BINDING annotation: " + line)
	}
	var options bindingOptions
	seen := make(map[string]bool)
	for entry := range strings.SplitSeq(match[1], ",") {
		key, value, ok := strings.Cut(entry, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			panic("SPX_BINDING requires key=value: " + entry)
		}
		if seen[key] {
			panic("duplicate SPX_BINDING option: " + key)
		}
		seen[key] = true
		switch key {
		case "web":
			options.Web = common.WebBindingMode(value)
		default:
			panic("unknown SPX_BINDING option: " + key)
		}
	}
	declaration := strings.TrimSpace(line[len(match[0]):])
	if strings.HasPrefix(declaration, "SPX_BINDING") {
		panic("duplicate SPX_BINDING annotation: " + line)
	}
	return &options, declaration
}

func (o bindingOptions) validate(method classMethodDecl, arrayBridge bool) {
	if o.Web == common.WebBindingDefault {
		return
	}
	if arrayBridge {
		panic("SPX_BINDING web cannot override an array bridge: " + method.MethodName)
	}
	switch o.Web {
	case common.WebBindingNoop:
		if method.ReturnType != "void" {
			panic("SPX_BINDING web=noop requires a void method: " + method.MethodName)
		}
	case common.WebBindingReuseResult:
		switch method.ReturnType {
		case "GdVec2", "GdVec3", "GdVec4", "GdColor", "GdRect2":
		default:
			panic("SPX_BINDING web=reuse_result requires a structured value result: " + method.MethodName)
		}
	default:
		panic("unknown SPX_BINDING web mode: " + string(o.Web))
	}
}
