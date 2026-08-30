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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/driverbundle"
)

func runVerifyRelease(args []string) error {
	flags := flag.NewFlagSet("driverbundle verify-release", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	directory, expectedVersion := ".", ""
	lockPath := "internal/release/runtime.lock.json"
	flags.StringVar(&lockPath, "lock", lockPath, "runtime lock JSON")
	flags.StringVar(&directory, "directory", directory, "download directory")
	flags.StringVar(&expectedVersion, "spx-version", "", "expected SPX release version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(expectedVersion) == "" {
		return fmt.Errorf("--spx-version is required")
	}
	manifestPath := filepath.Join(directory, driverbundle.ManifestName)
	data, err := readRegularFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read SPX driver manifest: %w", err)
	}
	lock, err := loadDriverLock(lockPath)
	if err != nil {
		return err
	}
	manifest, err := driverbundle.ParseForVersions(data, expectedVersion, lock.RuntimeVersion)
	if err != nil {
		return fmt.Errorf("parse SPX driver manifest: %w", err)
	}

	for _, bundle := range manifest.Bundles {
		zipPath := filepath.Join(directory, bundle.Name)
		if err := verifyBundleFileForRuntime(zipPath, bundle, lock.RuntimeVersion); err != nil {
			return fmt.Errorf("verify SPX driver %s/%s: %w", bundle.GOOS, bundle.GOARCH, err)
		}
	}
	return nil
}
