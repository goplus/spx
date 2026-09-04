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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/envutil"
)

func resolveRuntimeCacheRoot(env []string, defaultRoot func() string) (string, error) {
	value, found, duplicate := envutil.Lookup(env, runtimeCacheEnv)
	if duplicate {
		return "", fmt.Errorf("launchpack: duplicate %s", runtimeCacheEnv)
	}
	if !found || value == "" {
		value = filepath.Clean(defaultRoot())
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeCacheEnv)
	}
	return value, nil
}

func runtimeOffline(env []string) (bool, error) {
	value, found, duplicate := envutil.Lookup(env, runtimeOfflineEnv)
	if duplicate {
		return false, fmt.Errorf("launchpack: duplicate %s", runtimeOfflineEnv)
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if !found || value == "" {
		return false, nil
	}
	switch value {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("launchpack: invalid %s value %q", runtimeOfflineEnv, value)
	}
}

func runtimeEnvironment(cfg Config, base []string) []string {
	return environmentWithNonEmpty(base,
		envutil.Assignment{Key: runtimeLocalManifestEnv, Value: cfg.RuntimeManifestPath},
		envutil.Assignment{Key: runtimeAssetDirEnv, Value: cfg.RuntimeAssetDir},
		envutil.Assignment{Key: runtimeCacheEnv, Value: cfg.RuntimeCacheRoot},
	)
}

func environmentWithNonEmpty(base []string, values ...envutil.Assignment) []string {
	assignments := make([]envutil.Assignment, 0, len(values))
	for _, value := range values {
		if value.Value == "" {
			continue
		}
		assignments = append(assignments, value)
	}
	return envutil.SetMany(base, assignments...)
}
