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

package spx

import (
	"reflect"
	"strings"
	"syscall"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/ui"
)

// monitorBinding resolves reads and optional writes in the same variable scope.
type monitorBinding struct {
	read  func() ui.MonitorValue
	write func(float64) bool
}

// -----------------------------------------------------------------------------
// Evaluation
// -----------------------------------------------------------------------------
func getTarget(g reflect.Value, target string) (reflect.Value, int) {
	if target == "" {
		return g, 1 // spx.Game
	}
	if val := coreproject.FindFieldPtr(g, target, 0); val != nil {
		if _, ok := val.(Shape); ok {
			return reflect.ValueOf(val).Elem(), 2 // (spx.Sprite, *Game)
		}
	}
	return reflect.Value{}, -1
}

func bindMonitor(g reflect.Value, targetName, val string, appearance ui.MonitorAppearance) (monitorBinding, error) {
	target, from := getTarget(g, targetName)
	if from < 0 {
		return monitorBinding{}, syscall.ENOENT
	}
	binding := monitorBinding{read: buildMonitorEval(target, from, val, appearance)}
	if binding.read == nil {
		return monitorBinding{}, syscall.ENOENT
	}
	if appearance == ui.MonitorAppearanceSlider {
		binding.write = coreproject.ResolveMemberNumberSetter(target, strings.TrimPrefix(val, getVarPrefix), from)
		if binding.write == nil {
			return monitorBinding{}, syscall.EINVAL
		}
	}
	return binding, nil
}

func buildMonitorEval(target reflect.Value, from int, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	name := strings.TrimPrefix(val, getVarPrefix)
	if appearance == ui.MonitorAppearanceList {
		if name == "" {
			return nil
		}
		if eval := coreproject.ResolveMemberValueEval(target, name, from); eval != nil {
			return func() ui.MonitorValue { return ui.MonitorValue{Items: listMonitorItems(eval())} }
		}
		return nil
	}
	if val == getVarPrefix {
		spxlog.Error("Bind monitor error: name is empty")
		return nil
	}
	if eval := coreproject.ResolveMemberStringEval(target, name, from); eval != nil {
		return func() ui.MonitorValue { return ui.MonitorValue{Text: eval()} }
	}
	spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	return nil
}
