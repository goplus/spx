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
	"flag"
	"fmt"
	"os"
	"reflect"

	spxfs "github.com/goplus/spx/v3/fs"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *Game) loadGame(resource any, generation uint64) error {
	opened, err := coreproject.OpenBuilderResources(resource, nil)
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

	conf, proj := &opened.Config, &opened.Project
	parseCommandLineFlags(conf)
	p.applyRuntimeConfig(conf, proj)
	setupGameSystems(p, proj)
	gamer := reflect.ValueOf(p.gamer).Elem()
	loadGameSprites(p, gamer, opened.FS, proj)
	p.loadStage(gamer, proj, generation, p.loadSprite)

	platform := &engine.Managers().PlatformMgr
	if !conf.DontRunOnUnfocused {
		platform.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	platform.SetWindowTitle(p.runtimeConfigInput.Title)
	return nil
}

func (p *Game) startLoad(fs spxfs.Dir) {
	p.soundMgr.Init(&engine.Managers().AudioMgr)
	p.sounds = make(map[string]sound)
	p.inputMgr.init(p)
	p.events = make(chan event, eventBufferSize)
	p.eventQueueState.EventQueueStats.Reset()
	p.fs = fs
}

// -----------------------------------------------------------------------------
// Setup
// -----------------------------------------------------------------------------
func setupGameSystems(g *Game, proj *coreproject.ProjectConfig) {
	settings := coreproject.ResolveSystemSettings(proj)
	if settings.AutoSetCollisionLayer == g.physicsEnabled {
		engine.Panic("invalid configuration: autoSetCollisionLayer and physics enabled state must not be the same")
	}
	engine.SetLayerSortMode(settings.LayerSortMode)
	g.applyPathFinderSettings(settings)
	g.applyAudioSettings(settings)
	g.applyPhysicsSettings(settings)
}

// -----------------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------------
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *coreproject.ProjectConfig) {
	g.startLoad(fs)
	err := coreproject.WalkFields(v, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, v, fieldIndex)
	}, func(name string, val any) error {
		fld, ok := val.(Sprite)
		if !ok || g.typs[name] == nil {
			return nil
		}
		return g.loadSprite(fld, name, v)
	})
	if err != nil {
		engine.Panic(err)
	}
	g.tilemapMgr.init(g, fs, proj.TilemapPath)
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

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val any) {
	return coreproject.FieldPtrOrAlloc(v, i, coreproject.FieldAllocConfig{
		IsPointerSpriteType: func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		ResolveInterfaceSpriteType: func(fieldName string) (reflect.Type, bool) {
			typ, ok := g.typs[fieldName]
			return typ, ok
		},
	})
}
