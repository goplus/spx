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

package runtimecmd

import "github.com/goplus/spx/v3/internal/cmd/buildctl/shared"

type ExportWebConfig struct {
	Mode string
}

type BuildWasmConfig struct {
	Opt bool
}

func Run(args []string) error {
	return runRuntime(args)
}

func ParseRuntimeBuildWasmArgs(args []string) (BuildWasmConfig, error) {
	cfg, err := parseRuntimeBuildWasmArgs(args)
	return BuildWasmConfig{Opt: cfg.opt}, err
}

func ParseRuntimeCompressWasmArgs(args []string) error {
	return parseRuntimeCompressWasmArgs(args)
}

func ParseRuntimeExportPackArgs(args []string) error {
	return parseRuntimeExportPackArgs(args)
}

func ParseRuntimeExportWebArgs(args []string) (ExportWebConfig, error) {
	cfg, err := parseRuntimeExportWebArgs(args)
	return ExportWebConfig{Mode: cfg.mode}, err
}

func BuildWasmRuntime(cfg BuildWasmConfig, runner shared.ScriptRunner) error {
	return buildWasmRuntime(runtimeBuildWasmConfig{opt: cfg.Opt}, sharedScriptRunnerAdapter{inner: runner})
}

func CompressWasmArtifacts() error {
	return compressWasmArtifacts()
}

func ExportPackRuntime(runner shared.ScriptRunner) error {
	return exportPackRuntime(sharedScriptRunnerAdapter{inner: runner})
}

func ExportWebRuntime(cfg ExportWebConfig, runner shared.ScriptRunner) error {
	return exportWebRuntime(runtimeExportWebConfig{mode: cfg.Mode}, sharedScriptRunnerAdapter{inner: runner})
}

func ExportWebTemplateRuntime(mode string, runner shared.ScriptRunner) error {
	return exportWebTemplateRuntime(mode, sharedScriptRunnerAdapter{inner: runner})
}
