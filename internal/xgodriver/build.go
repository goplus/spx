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
	"fmt"

	"github.com/goplus/spx/v3/internal/launchpack"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func buildLauncher(ctx context.Context, cfg Config, snapshot projectpolicy.PortableConfigSnapshot, streams IO) error {
	if err := verifyLauncherPackage(ctx, cfg, streams.Env); err != nil {
		return err
	}
	verifier, err := newGraphVerifier(ctx, cfg, streams.Env)
	if err != nil {
		return fmt.Errorf("xgodriver: snapshot launcher graph: %w", err)
	}
	packCfg := cfg.launchpackConfig(snapshot, streams)
	packCfg.VerifyGraph = verifier.verify
	if _, err := launchpack.BuildLauncher(ctx, packCfg); err != nil {
		return fmt.Errorf("xgodriver: build launcher: %w", err)
	}
	if err := verifyBuiltSPXOrigin(ctx, cfg.Output, cfg.DriverOrigin, moduleDependency, cfg, streams.Env); err != nil {
		return fmt.Errorf("xgodriver: verify generated launcher provenance: %w", err)
	}
	return nil
}
