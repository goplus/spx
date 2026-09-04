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
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/goplus/spx/v3/internal/envutil"
)

type listedPackage struct {
	ImportPath string        `json:"ImportPath"`
	Module     *listedModule `json:"Module"`
}

type listedModule struct {
	Path    string        `json:"Path"`
	Version string        `json:"Version"`
	Dir     string        `json:"Dir"`
	GoMod   string        `json:"GoMod"`
	Main    bool          `json:"Main"`
	Replace *listedModule `json:"Replace"`
}

func verifyLauncherPackage(ctx context.Context, cfg Config, baseEnv []string) error {
	return verifyPackageOrigin(ctx, cfg, baseEnv, "github.com/goplus/spx/v3/x/xgolauncher", "launcher")
}

func verifyDriverPackage(ctx context.Context, cfg Config, baseEnv []string) error {
	return verifyPackageOrigin(ctx, cfg, baseEnv, cfg.DriverPackage, "driver")
}

func verifyPackageOrigin(ctx context.Context, cfg Config, baseEnv []string, importPath, label string) error {
	if err := validatePackageGraphPolicy(cfg); err != nil {
		return requestError(err)
	}
	args := append([]string{"list"}, cfg.graphFlagsForCommand()...)
	args = append(args, "-json", importPath)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = hostGoEnv(cfg, baseEnv)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("xgodriver: validate %s package: %w", label, err)
	}
	var listed listedPackage
	if err := json.Unmarshal(output, &listed); err != nil {
		return fmt.Errorf("xgodriver: decode %s package identity: %w", label, err)
	}
	if listed.ImportPath != importPath || listed.Module == nil {
		return requestError(fmt.Errorf("xgodriver: %s package resolved to unexpected identity %q", label, listed.ImportPath))
	}
	got, err := normalizeCommandOrigin(listed.Module, cfg)
	if err != nil {
		return requestError(fmt.Errorf("xgodriver: invalid %s package origin: %w", label, err))
	}
	if !got.Equal(cfg.DriverOrigin) {
		return requestError(fmt.Errorf("xgodriver: %s package origin does not match resolved driver provenance", label))
	}
	return nil
}

func normalizeCommandOrigin(module *listedModule, cfg Config) (ModuleOrigin, error) {
	privateModFile := cfg.privateModFileForCommand()
	want := cfg.DriverOrigin
	if privateModFile != "" && want.Main && module.Main &&
		module.Path == want.Selected.Path && module.Dir == want.Selected.Dir &&
		normalizeGraphPath(module.GoMod) == normalizeGraphPath(privateModFile) {
		copy := *module
		copy.GoMod = want.Selected.GoMod
		module = &copy
	}
	return normalizeListedOrigin(module)
}

func validatePackageGraphPolicy(cfg Config) error {
	if err := cfg.validateGraphInputs(); err != nil {
		return err
	}
	for _, flag := range cfg.GraphFlags {
		if flag == "-mod=vendor" {
			return fmt.Errorf("xgodriver: project driver does not support vendor mode; use -mod=readonly or -mod=mod")
		}
	}
	return nil
}

func normalizeListedOrigin(module *listedModule) (ModuleOrigin, error) {
	if module == nil {
		return ModuleOrigin{}, fmt.Errorf("missing module")
	}
	origin := ModuleOrigin{Selected: ModuleRef{Path: module.Path, Version: module.Version}, Main: module.Main}
	if module.Replace == nil {
		origin.Selected.Dir, origin.Selected.GoMod = module.Dir, module.GoMod
	} else {
		replacement := ModuleRef{
			Path: module.Replace.Path, Version: module.Replace.Version,
			Dir: module.Replace.Dir, GoMod: module.Replace.GoMod,
		}
		if replacement.Version == "" {
			replacement.Path = replacement.Dir
		}
		origin.Replace = &replacement
	}
	if err := origin.Validate(); err != nil {
		return ModuleOrigin{}, err
	}
	return origin, nil
}

func hostGoEnv(cfg Config, base []string) []string {
	return envutil.HostGoEnvironment(base, cfg.goWorkForCommand(), false, activeEnvironment)
}
