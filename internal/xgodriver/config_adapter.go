/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package xgodriver

import (
	"context"
	"runtime"

	"github.com/goplus/spx/v3/internal/launchpack"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func (cfg Config) launchpackConfig(snapshot projectpolicy.PortableConfigSnapshot, streams IO) launchpack.Config {
	origin := cfg.DriverOrigin
	effective := origin.Effective()
	packCfg := launchpack.Config{
		ProjectDir:      cfg.ProjectDir,
		ProjectFile:     cfg.ProjectFile,
		ProjectExt:      cfg.Project.Extension,
		PackDir:         cfg.Project.PackDirectory,
		PackIndex:       cfg.Project.PackIndexFile,
		PortableConfig:  snapshot,
		RuntimeIdentity: launchpack.RuntimeIdentity{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Source: launchpack.SourceIdentity{
			SelectedPath:     origin.Selected.Path,
			SelectedVersion:  origin.Selected.Version,
			EffectivePath:    effective.Path,
			EffectiveVersion: effective.Version,
			Main:             origin.Main,
			SourceMode:       origin.IsLocal(),
		},
		GoCommand:  cfg.GoCommand,
		WorkDir:    cfg.GraphWorkDir,
		GoWork:     cfg.goWorkForCommand(),
		GraphFlags: append([]string(nil), cfg.graphFlagsForCommand()...),
		BuildFlags: append([]string(nil), cfg.BuildFlags...),
		Output:     cfg.Output,
		IO:         launchpack.IO{Stdin: streams.Stdin, Stdout: streams.Stdout, Stderr: streams.Stderr, Env: streams.Env},
	}
	if origin.IsLocal() {
		packCfg.RuntimeSourceRoot = effective.Dir
		packCfg.BridgePackage = origin.Selected.Path + "/cmd/ispxnative"
		packCfg.VerifyBridge = cfg.verifyBridge(streams)
	}
	return packCfg
}

func (cfg Config) verifyBridge(streams IO) func(context.Context, string) error {
	return func(ctx context.Context, path string) error {
		return verifyBuiltSPXOrigin(ctx, path, cfg.DriverOrigin, moduleMain, cfg, streams.Env)
	}
}
