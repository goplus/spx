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

	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

// applyRuntimeConfig applies startup settings and returns the window title.
func (p *Game) applyRuntimeConfig(conf *Config, proj *coreproject.ProjectConfig) string {
	if conf == nil {
		conf = &Config{}
	}
	// Desktop sessions run from a temporary directory, separate from the project.
	projectDir := os.Getenv("SPX_PROJECT_DIR")
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	runtimeCfg := coreproject.ResolveRuntimeConfig(conf, proj, projectDir, os.Getenv("SPX_SCREENSHOT_KEY"))
	proj.FullScreen = runtimeCfg.FullScreen
	p.physicsEnabled = runtimeCfg.PhysicsEnabled
	p.displayState.WindowHeight = runtimeCfg.WindowHeight
	p.displayState.WindowWidth = runtimeCfg.WindowWidth

	if runtimeCfg.ScreenshotKey != "" {
		if err := os.Setenv("SPX_SCREENSHOT_KEY", runtimeCfg.ScreenshotKey); err != nil {
			engine.Panic(err)
		}
	}
	return runtimeCfg.Title
}
