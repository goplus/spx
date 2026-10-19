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

	spxfs "github.com/goplus/spx/v3/fs"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

// -----------------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------------
func (p *Game) loadGame(resource any, generation uint64) (err error) {
	opened, err := coreproject.OpenBuilderResources(resource, nil)
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
	if err := validateProjectConfig(proj); err != nil {
		return fmt.Errorf("project config: %w", err)
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
	gamer := reflect.ValueOf(p.gamer).Elem()
	p.startLoad(opened.FS)
	if err := p.loadSpriteFields(gamer); err != nil {
		return err
	}
	p.tilemapMgr.init(p, opened.FS, proj.TilemapPath)
	if err := p.loadStage(gamer, proj, generation); err != nil {
		return err
	}
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

func (p *Game) loadSpriteFields(gamer reflect.Value) error {
	for i := range gamer.NumField() {
		name, sprite := p.allocSpriteField(gamer, i)
		if sprite == nil || p.typs[name] == nil {
			continue
		}
		if err := p.loadSprite(sprite, name, gamer); err != nil {
			return fmt.Errorf("load sprite %q: %w", name, err)
		}
	}
	return nil
}

// Allocate before filtering so registered non-sprite interface fields are initialized too.
func (g *Game) allocSpriteField(v reflect.Value, i int) (string, Sprite) {
	name, val := coreproject.FieldPtrOrAlloc(v, i, coreproject.FieldAllocConfig{
		IsPointerSpriteType: func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		ResolveInterfaceSpriteType: func(fieldName string) (reflect.Type, bool) {
			typ, ok := g.typs[fieldName]
			return typ, ok
		},
	})
	sprite, _ := val.(Sprite)
	return name, sprite
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

func validateProjectConfig(project *coreproject.ProjectConfig) error {
	for i, backdrop := range project.Backdrops {
		if backdrop == nil {
			return fmt.Errorf("backdrops[%d] is null", i)
		}
	}
	settings := coreproject.ResolveSystemSettings(project)
	if settings.AutoSetCollisionLayer == project.Physics {
		return fmt.Errorf("autoSetCollisionLayer and physics must have different enabled states")
	}
	return nil
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
