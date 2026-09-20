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
	"os"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *Game) applyRuntimeConfig(conf *Config, proj *coreproject.ProjectConfig) {
	if conf != nil {
		p.runtimeConfigInput = *conf
	} else {
		p.runtimeConfigInput = Config{}
	}
	p.applyStoredRuntimeConfig(proj)
}

func (p *Game) applyStoredRuntimeConfig(proj *coreproject.ProjectConfig) {
	runtimeCfg := resolveGameRuntimeConfig(p.runtimeConfigInput, proj)
	p.runtimeConfigInput.Title = runtimeCfg.Title
	proj.FullScreen = runtimeCfg.FullScreen
	p.physicsEnabled = runtimeCfg.PhysicsEnabled
	p.eventQueueState.EventQueuePolicy = coreevent.ParsePolicy(runtimeCfg.EventQueuePolicy)
	p.displayState.WindowHeight = runtimeCfg.WindowHeight
	p.displayState.WindowWidth = runtimeCfg.WindowWidth

	if runtimeCfg.ScreenshotKey == "" {
		return
	}
	if err := os.Setenv("SPX_SCREENSHOT_KEY", runtimeCfg.ScreenshotKey); err != nil {
		engine.Panic(err)
	}
}

func resolveGameRuntimeConfig(conf Config, proj *coreproject.ProjectConfig) coreproject.RuntimeConfig {
	cwd, _ := os.Getwd()
	return coreproject.ResolveRuntimeConfig(&conf, proj, cwd, os.Getenv("SPX_SCREENSHOT_KEY"))
}
