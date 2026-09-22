//go:build !js && !pure_engine

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
	"sync/atomic"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type animationRegistrationResMgr struct {
	pkgengine.IResMgr
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (m *animationRegistrationResMgr) CreateAnimation(string, string, string, int64, bool) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
	}
	<-m.release
}

func TestClonedAnimationEntryRegistersOnceConcurrently(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	resources := &animationRegistrationResMgr{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	originalResMgr := pkgengine.ResMgr
	pkgengine.ResMgr = resources
	t.Cleanup(func() { pkgengine.ResMgr = originalResMgr })

	frame := newCostumeWithSize(1, 1)
	frame.bitmapResolution = 4
	frame.path = "walk.png"
	sprite := &SpriteImpl{name: "TestSprite"}
	sprite.costumes = []*costume{frame}
	original := &animationComponent{sprite: sprite}
	original.initFromConfig(&coreproject.SpriteConfig{
		FAnimations: map[string]*coreproject.AniConfig{
			"walk": {FrameFrom: 0, FrameTo: 0},
		},
	})
	clone := original.cloneFor(&SpriteImpl{name: sprite.name})

	originalEntry := original.shared.animations["walk"]
	cloneEntry := clone.shared.animations["walk"]
	if originalEntry != cloneEntry {
		t.Fatal("clone did not share the original animation entry")
	}

	start := make(chan struct{})
	started := make(chan struct{}, 2)
	results := make(chan int, 2)
	for _, entry := range []*animationEntry{originalEntry, cloneEntry} {
		go func() {
			<-start
			started <- struct{}{}
			results <- entry.ensureRegistered()
		}()
	}
	close(start)
	<-started
	<-started
	<-resources.entered
	close(resources.release)

	for range 2 {
		if got := <-results; got != 4 {
			t.Errorf("bitmap resolution = %d, want 4", got)
		}
	}
	if got := resources.calls.Load(); got != 1 {
		t.Fatalf("animation registrations = %d, want 1", got)
	}
}
