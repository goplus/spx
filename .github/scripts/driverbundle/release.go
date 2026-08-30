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

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
)

type descriptorInputs []string

func (v *descriptorInputs) String() string { return strings.Join(*v, ",") }

func (v *descriptorInputs) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("descriptor path must not be empty")
	}
	*v = append(*v, value)
	return nil
}

func runAssemble(args []string) error {
	flags := flag.NewFlagSet("driverbundle assemble", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	lockPath := "internal/release/runtime.lock.json"
	spxVersion := release.DefaultReleaseMeta().SPXVersion
	manifestPath := ""
	var descriptors descriptorInputs
	flags.StringVar(&lockPath, "lock", lockPath, "runtime lock JSON")
	flags.StringVar(&spxVersion, "spx-version", spxVersion, "SPX release version")
	flags.StringVar(&manifestPath, "manifest", "", "output driver-manifest.json path")
	flags.Var(&descriptors, "descriptor", "bundle descriptor JSON path; repeat exactly four times")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: driverbundle assemble --manifest PATH --spx-version VERSION --descriptor JSON ...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(manifestPath) == "" {
		return errors.New("manifest path is required")
	}
	targets := driverbundle.SupportedTargets()
	if len(descriptors) != len(targets) {
		return fmt.Errorf("descriptor count = %d, want %d", len(descriptors), len(targets))
	}
	lock, err := loadDriverLock(lockPath)
	if err != nil {
		return err
	}

	bundles := make([]driverbundle.Bundle, 0, len(descriptors))
	seen := make(map[string]struct{}, len(descriptors))
	sources := make(map[string]string, len(descriptors))
	for _, descriptorPath := range descriptors {
		data, err := readRegularFile(descriptorPath)
		if err != nil {
			return fmt.Errorf("read descriptor %s: %w", descriptorPath, err)
		}
		bundle, err := driverbundle.ParseBundle(data)
		if err != nil {
			return fmt.Errorf("parse descriptor %s: %w", descriptorPath, err)
		}
		key := bundle.GOOS + "/" + bundle.GOARCH
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate driver target %s/%s", bundle.GOOS, bundle.GOARCH)
		}
		zipPath := filepath.Join(filepath.Dir(descriptorPath), bundle.Name)
		seen[key] = struct{}{}
		sources[key] = zipPath
		bundles = append(bundles, bundle)
	}
	slices.SortFunc(bundles, compareDriverBundles)
	manifest := driverbundle.Manifest{
		Schema:         driverbundle.ManifestSchema,
		SPXVersion:     spxVersion,
		RuntimeVersion: lock.RuntimeVersion,
		Bundles:        bundles,
	}
	manifestData, err := manifest.JSON()
	if err != nil {
		return err
	}
	if err := ensureParent(manifestPath); err != nil {
		return fmt.Errorf("prepare manifest output: %w", err)
	}
	for _, bundle := range bundles {
		key := bundle.GOOS + "/" + bundle.GOARCH
		destination := filepath.Join(filepath.Dir(manifestPath), bundle.Name)
		if err := copyRegularFile(sources[key], destination); err != nil {
			return fmt.Errorf("copy %s/%s bundle: %w", bundle.GOOS, bundle.GOARCH, err)
		}
		if err := verifyBundleFileForRuntime(destination, bundle, lock.RuntimeVersion); err != nil {
			return fmt.Errorf("verify copied %s/%s bundle: %w", bundle.GOOS, bundle.GOARCH, err)
		}
	}
	if err := atomicWrite(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("write driver manifest: %w", err)
	}
	return nil
}

func loadDriverLock(path string) (release.RuntimeLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return release.RuntimeLock{}, fmt.Errorf("read runtime lock: %w", err)
	}
	lock, err := release.ParseRuntimeLock(data)
	if err != nil {
		return release.RuntimeLock{}, err
	}
	return lock, nil
}

func compareDriverBundles(a, b driverbundle.Bundle) int {
	return strings.Compare(a.GOOS+"/"+a.GOARCH, b.GOOS+"/"+b.GOARCH)
}
