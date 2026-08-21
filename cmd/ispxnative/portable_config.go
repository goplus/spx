//go:build !js || !wasm

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
	"fmt"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

type portableConfigOverlay struct {
	present bool
	data    []byte
}

func loadPortableConfigOverlay(env []string, roots interpruntime.Roots) (*portableConfigOverlay, error) {
	configDir, expectedIdentity, found, err := interpruntime.PortableConfigFromEnv(env)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if err := validatePortableConfigDir(roots.SessionDir, configDir); err != nil {
		return nil, err
	}
	configRoot, err := openPortableConfigRoot(roots.SessionDir, configDir)
	if err != nil {
		return nil, err
	}
	defer configRoot.Close()
	snapshot, err := projectpolicy.SnapshotPortableConfigRoot(configRoot)
	if err != nil {
		return nil, fmt.Errorf("ispxnative: load portable config snapshot: %w", err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		return nil, fmt.Errorf("ispxnative: identify portable config snapshot: %w", err)
	}
	if identity != expectedIdentity {
		return nil, fmt.Errorf("ispxnative: portable config identity mismatch")
	}
	return &portableConfigOverlay{present: snapshot.Present(), data: snapshot.Bytes()}, nil
}
