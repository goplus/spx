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
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/x/xgolauncher"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const activeEnvironment = "SPX_XGO_DRIVER_ACTIVE"

// IO contains inherited streams and the caller's complete environment.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

// Execute runs one parsed driver request.
func Execute(ctx context.Context, cfg Config, streams IO) (xgolauncher.ProcessStatus, error) {
	if ctx == nil {
		return xgolauncher.ProcessStatus{}, errors.New("xgodriver: nil context")
	}
	if err := validateSPXRequest(cfg); err != nil {
		return xgolauncher.ProcessStatus{}, requestError(err)
	}
	if err := validateDriverOrigin(cfg.DriverOrigin); err != nil {
		return xgolauncher.ProcessStatus{}, requestError(err)
	}
	if envutil.HasNonEmpty(streams.Env, activeEnvironment) {
		return xgolauncher.ProcessStatus{}, requestError(errors.New("xgodriver: recursive project-driver invocation rejected"))
	}
	if err := VerifyDeclaration(cfg.Declaration); err != nil {
		return xgolauncher.ProcessStatus{}, requestError(err)
	}
	if err := VerifyTargetModFile(cfg.TargetModFile); err != nil {
		return xgolauncher.ProcessStatus{}, requestError(err)
	}
	snapshot, err := projectpolicy.SnapshotPortableConfig(cfg.ProjectDir)
	if err != nil {
		return xgolauncher.ProcessStatus{}, requestError(fmt.Errorf("xgodriver: %w", err))
	}
	if err := validatePackageGraphPolicy(cfg); err != nil {
		return xgolauncher.ProcessStatus{}, requestError(err)
	}
	isolated, cleanupGraph, err := isolateGoGraph(ctx, cfg, streams.Env)
	if err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	defer cleanupGraph()
	cfg = isolated
	if err := verifyDriverPackage(ctx, cfg, streams.Env); err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	switch cfg.Action {
	case ActionRun:
		return runProject(ctx, cfg, snapshot, streams)
	case ActionBuild:
		if err := buildLauncher(ctx, cfg, snapshot, streams); err != nil {
			return xgolauncher.ProcessStatus{}, err
		}
		return xgolauncher.ProcessStatus{}, nil
	default:
		return xgolauncher.ProcessStatus{}, requestError(fmt.Errorf("xgodriver: unsupported action %q", cfg.Action))
	}
}

func validateDriverOrigin(origin ModuleOrigin) error {
	if origin.Selected.Path != driverbundle.SPXModulePath {
		return fmt.Errorf("xgodriver: driver module %q is not supported", origin.Selected.Path)
	}
	if origin.IsLocal() {
		return nil
	}
	if origin.Main || origin.Replace != nil {
		return errors.New("xgodriver: published mode requires an unreplaced dependency")
	}
	version := origin.Selected.Version
	if !semver.IsValid(version) || semver.Canonical(version) != version || module.IsPseudoVersion(version) {
		return fmt.Errorf("xgodriver: published driver requires an exact canonical release version, got %q", version)
	}
	return nil
}

// validateSPXRequest applies SPX-only requirements not shared by drivers.
func validateSPXRequest(cfg Config) error {
	if cfg.Project.Extension != ".spx" || cfg.Project.FullExtension != "main.spx" {
		return fmt.Errorf("xgodriver: unsupported SPX project shape %q/%q", cfg.Project.Extension, cfg.Project.FullExtension)
	}
	if filepath.Base(cfg.ProjectFile) != cfg.Project.FullExtension {
		return fmt.Errorf("xgodriver: SPX project file must be %q", cfg.Project.FullExtension)
	}
	if cfg.Project.PackDirectory == "" || cfg.Project.PackIndexFile == "" {
		return errors.New("xgodriver: SPX driver requires pack metadata")
	}
	if cfg.Project.PackDirectory == "." {
		return errors.New("xgodriver: SPX driver requires a dedicated pack directory below the project root")
	}
	return nil
}
