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

package launchpack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func validatePublishedSource(source SourceIdentity) error {
	if source.SourceMode {
		return errors.New("launchpack: published driver cannot use source mode")
	}
	if source.Main {
		return errors.New("launchpack: published mode requires an unreplaced SPX dependency")
	}
	if source.SelectedPath != driverbundle.SPXModulePath || source.EffectivePath != driverbundle.SPXModulePath {
		return fmt.Errorf("launchpack: published driver module must be %q", driverbundle.SPXModulePath)
	}
	version := source.SelectedVersion
	if !semver.IsValid(version) || semver.Canonical(version) != version || module.IsPseudoVersion(version) {
		return fmt.Errorf("launchpack: published driver requires an exact canonical release version, got %q", version)
	}
	if source.EffectiveVersion != version {
		return fmt.Errorf("launchpack: published driver effective version %q does not match selected version %q", source.EffectiveVersion, version)
	}
	return nil
}

func publishedDriverEnvironment(cfg Config, base []string) []string {
	return environmentWithNonEmpty(base,
		envutil.Assignment{Key: driverAssetDirEnv, Value: cfg.DriverAssetDir},
		envutil.Assignment{Key: runtimeCacheEnv, Value: cfg.RuntimeCacheRoot},
	)
}

func acquireDriverFile(ctx context.Context, root, name string, size int64, digest, url, localDir string, offline bool, fetch runtimebundle.FetchFunc) (*runtimebundle.AcquiredFile, error) {
	if localDir != "" {
		source := filepath.Join(localDir, name)
		if err := verifyLocalDriverAsset(source, size, digest); err != nil {
			return nil, err
		}
		return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
			Name: name, URL: source, Size: size, SHA256: digest,
			Fetch: func(ctx context.Context, _ string, dst io.Writer) error {
				return copyLocalRuntimeAsset(ctx, source, dst)
			},
		})
	}
	return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
		Name: name, URL: url, Size: size, SHA256: digest, Offline: offline, Fetch: fetch,
	})
}

func verifyLocalDriverAsset(path string, size int64, digest string) error {
	gotSize, gotDigest, err := hashRuntimeFile(path)
	if err != nil {
		return fmt.Errorf("verify local published driver asset %q: %w", path, err)
	}
	if gotSize != size {
		return fmt.Errorf("local published driver asset %q size = %d, want %d", path, gotSize, size)
	}
	if gotDigest != digest {
		return fmt.Errorf("local published driver asset %q SHA-256 = %s, want %s", path, gotDigest, digest)
	}
	return nil
}
