/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package spx

import (
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine/platform"
	itime "github.com/goplus/spx/v2/internal/time"
)

const (
	defaultDeterministicSeed        int64 = 1
	defaultDeterministicWebTimestep       = 1.0 / itime.DefaultFPS
)

type resolvedDeterministicConfig struct {
	fixedTimestep float64
	randomSeed    *int64
}

func resolveFixedTimestep(fixedTimestep float64, deterministic bool, isWeb bool) float64 {
	if !isWeb {
		return 0
	}
	if fixedTimestep > 0 {
		return fixedTimestep
	}
	if deterministic {
		return defaultDeterministicWebTimestep
	}
	return 0
}

func resolveDeterministicConfig(cfg coreproject.RuntimeConfig, isWeb bool) resolvedDeterministicConfig {
	randomSeed := cfg.RandomSeed

	if cfg.Deterministic {
		if randomSeed == nil {
			seed := defaultDeterministicSeed
			randomSeed = &seed
		}
	}

	return resolvedDeterministicConfig{
		fixedTimestep: resolveFixedTimestep(cfg.FixedTimestep, cfg.Deterministic, isWeb),
		randomSeed:    randomSeed,
	}
}

func (p *Game) applyDeterministicConfig(cfg coreproject.RuntimeConfig) {
	resolved := resolveDeterministicConfig(cfg, platform.IsWeb())
	p.configuredFixedTimestep = resolved.fixedTimestep

	// Deterministic mode currently scopes to SPX's logical update clock on web.
	// Godot fixed-physics callbacks continue to use their engine-provided delta.
	itime.SetFixedDeltaTime(resolved.fixedTimestep)
	applyRandomSeed(resolved.randomSeed, cfg.Deterministic)
}

func applyRandomSeed(seed *int64, deterministic bool) {
	if seed == nil {
		ResetRandomSeed()
		return
	}
	if deterministic {
		setDeterministicRandomSeed(*seed)
		return
	}
	SetRandomSeed(*seed)
}
