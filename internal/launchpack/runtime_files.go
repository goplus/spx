/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package launchpack

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/internal/release"
)

// RuntimeNames contains file names for the Engine, its PCK, and interpreter bridge.
// Source builds may place the bridge in a separate build directory; installed
// runtimes must supply all three files from the same installation directory.
type RuntimeNames struct {
	Engine string
	Pack   string
	Bridge string
}

// RuntimeNamesFor preserves locally named development versions as well
// as published versions. It describes file names, not release provenance.
func RuntimeNamesFor(version, goos, goarch string) (RuntimeNames, error) {
	if version == "" || strings.ContainsAny(version, `/\\`) {
		return RuntimeNames{}, fmt.Errorf("launchpack: invalid runtime version %q", version)
	}
	bridge, err := bridgeFileName(goos, goarch)
	if err != nil {
		return RuntimeNames{}, err
	}
	names := RuntimeNames{
		Engine: release.RuntimeTag + version,
		Pack:   release.RuntimeTag + version + ".pck",
		Bridge: bridge,
	}
	if goos == "windows" {
		names.Engine += ".exe"
	}
	return names, names.validate()
}

// ResolveInstalledRuntime selects the first complete installation, never
// combining files from different directories. An incomplete earlier directory
// may fall through to a complete later one. This is the explicit legacy
// compatibility path: historical installations have no bridge-version
// manifest, so only their co-location and selected file names can be asserted.
func ResolveInstalledRuntime(directories []string, names RuntimeNames) (Assets, error) {
	if err := names.validate(); err != nil {
		return Assets{}, err
	}
	var failures []error
	for _, directory := range directories {
		assets := names.inDirectory(directory)
		if err := assets.validateFiles(names); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", directory, err))
			continue
		}
		return assets, nil
	}
	if len(failures) == 0 {
		return Assets{}, fmt.Errorf("launchpack: no runtime installation directories configured")
	}
	return Assets{}, fmt.Errorf("launchpack: no complete runtime installation: %w", errors.Join(failures...))
}

// ValidateFiles checks a complete runtime before it is run or packaged.
// Acquisition still owns content hashes, release identity and use leases;
// this shared check never substitutes file names for those guarantees.
func (assets Assets) ValidateFiles(names RuntimeNames) error {
	if err := names.validate(); err != nil {
		return err
	}
	return assets.validateFiles(names)
}

func (assets Assets) validateFiles(names RuntimeNames) error {
	if err := validateNamedRuntimeFile("Engine", assets.EnginePath, names.Engine); err != nil {
		return err
	}
	if err := validateNamedRuntimeFile("runtime pack", assets.PackPath, names.Pack); err != nil {
		return err
	}
	if filepath.Dir(assets.EnginePath) != filepath.Dir(assets.PackPath) {
		return fmt.Errorf("launchpack: Engine and runtime pack must share one directory")
	}
	return validateNamedRuntimeFile("interpreter bridge", assets.BridgePath, names.Bridge)
}

func (names RuntimeNames) validate() error {
	for _, name := range []string{names.Engine, names.Pack, names.Bridge} {
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
			return fmt.Errorf("launchpack: invalid runtime file name %q", name)
		}
	}
	// A Unix version can itself end in .exe; Windows adds one executable
	// suffix after the version. Accept either spelling without stripping a
	// suffix that belongs to the version.
	if !strings.HasPrefix(names.Engine, release.RuntimeTag) ||
		(names.Pack != names.Engine+".pck" && names.Pack != strings.TrimSuffix(names.Engine, ".exe")+".pck") {
		return fmt.Errorf("launchpack: Engine and runtime pack file names must identify the same version")
	}
	return nil
}

func (names RuntimeNames) inDirectory(directory string) Assets {
	return Assets{
		EnginePath: filepath.Join(directory, names.Engine),
		PackPath:   filepath.Join(directory, names.Pack),
		BridgePath: filepath.Join(directory, names.Bridge),
	}
}

func validateNamedRuntimeFile(label, path, name string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("launchpack: %s path must be absolute and clean: %q", label, path)
	}
	if filepath.Base(path) != name {
		return fmt.Errorf("launchpack: %s file %q does not match selected runtime %q", label, path, name)
	}
	return validateRuntimeFile(path, label)
}
