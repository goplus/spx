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
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"

	spxfs "github.com/goplus/spx/v4/fs"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/projectloader"
)

// -----------------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------------
func (p *Game) loadGame(resource any, generation uint64) (err error) {
	opened, err := projectloader.OpenResources(resource, nil)
	if err != nil {
		return err
	}
	previousFS := p.fs
	previousTilemap := p.tilemapMgr
	defer func() {
		if opened.FS == nil {
			return
		}
		// Failed loading must not retain the closed resource directory.
		p.fs = previousFS
		p.tilemapMgr = previousTilemap
		if closeErr := opened.FS.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close project resources: %w", closeErr))
		}
	}()

	conf, proj := &opened.Config, &opened.Project
	parseCommandLineFlags(conf)
	gamer := reflect.ValueOf(p.gamer)
	if gamer.IsValid() {
		gamer = gamer.Elem()
	}
	plan, err := p.prepareProject(opened.FS, proj, gamer)
	if err != nil {
		return err
	}
	if opened.AssetDir != "" {
		engine.SetAssetDir(opened.AssetDir)
	}
	fontPlan := coreproject.ResolveRuntimeFontPlan(opened.Fonts, engine.ToAssetPath)
	if err := applyRuntimeFontPlan(&engine.Managers().ResMgr, fontPlan); err != nil {
		return fmt.Errorf("apply project fonts: %w", err)
	}

	title := p.applyRuntimeConfig(conf, proj)
	setupGameSystems(p, proj)
	p.startLoad(opened.FS)
	p.applySpriteFields(gamer, &plan)
	p.tilemapMgr.g, p.tilemapMgr.fs = p, opened.FS
	p.tilemapMgr.applyLoadedMap(plan.tilemap)
	p.applyPreparedStage(gamer, proj, generation, &plan)
	opened.FS = nil // The game now owns the directory.

	platform := &engine.Managers().PlatformMgr
	if !conf.DontRunOnUnfocused {
		platform.SetRunnableOnUnfocused(true)
	}
	p.initLogicLoop()
	platform.SetWindowTitle(title)
	return nil
}

func (p *Game) startLoad(fs spxfs.Dir) {
	p.soundMgr.Init(&engine.Managers().AudioMgr)
	p.sounds = make(map[string]sound)
	p.inputMgr.init(p)
	p.fs = fs
}

// -----------------------------------------------------------------------------
// Setup
// -----------------------------------------------------------------------------
func setupGameSystems(g *Game, proj *coreproject.ProjectConfig) {
	settings := coreproject.ResolveSystemSettings(proj)
	engine.SetLayerSortMode(settings.LayerSortMode)
	g.applyPathFinderSettings(settings)
	g.applyAudioSettings(settings)
	g.applyPhysicsSettings(settings)
}

func parseCommandLineFlags(conf *Config) {
	f := flag.CommandLine
	effects, err := coreproject.ParseCommandLineFlags(f, os.Args[1:], conf)
	if err != nil {
		engine.Panic(err)
	}

	if effects.ShowHelp {
		fmt.Fprintf(os.Stderr, "Usage: %v [-v -f -h]\n", os.Args[0])
		f.PrintDefaults()
		os.Exit(0)
	}

	if effects.Verbose {
		SetDebug(DbgFlagAll)
	}
}
