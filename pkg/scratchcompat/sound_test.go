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

package scratchcompat_test

import (
	"slices"
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

type soundCall struct {
	name  string
	kind  spx.SoundEffectKind
	value float64
}

type soundTarget struct{ calls []soundCall }

func (s *soundTarget) SetVolume(volume float64) {
	s.calls = append(s.calls, soundCall{name: "set volume", value: volume})
}

func (s *soundTarget) ChangeVolume(delta float64) {
	s.calls = append(s.calls, soundCall{name: "change volume", value: delta})
}

func (s *soundTarget) SetSoundEffect(kind spx.SoundEffectKind, value float64) {
	s.calls = append(s.calls, soundCall{name: "set effect", kind: kind, value: value})
}

func (s *soundTarget) ChangeSoundEffect(kind spx.SoundEffectKind, delta float64) {
	s.calls = append(s.calls, soundCall{name: "change effect", kind: kind, value: delta})
}

var soundOperations = []struct {
	name string
	call func(*soundTarget)
	want soundCall
}{
	{"set volume", func(s *soundTarget) { scratchcompat.SetVolume(s, 100) }, soundCall{name: "set volume", value: 100}},
	{"change volume", func(s *soundTarget) { scratchcompat.ChangeVolume(s, 0) }, soundCall{name: "change volume", value: 0}},
	{"set pan", func(s *soundTarget) { scratchcompat.SetSoundEffect(s, spx.SoundPanEffect, -25) }, soundCall{name: "set effect", kind: spx.SoundPanEffect, value: -25}},
	{"set pitch", func(s *soundTarget) { scratchcompat.SetSoundEffect(s, spx.SoundPitchEffect, 240) }, soundCall{name: "set effect", kind: spx.SoundPitchEffect, value: 240}},
	{"change pan", func(s *soundTarget) { scratchcompat.ChangeSoundEffect(s, spx.SoundPanEffect, 0) }, soundCall{name: "change effect", kind: spx.SoundPanEffect, value: 0}},
	{"change pitch", func(s *soundTarget) { scratchcompat.ChangeSoundEffect(s, spx.SoundPitchEffect, -30) }, soundCall{name: "change effect", kind: spx.SoundPitchEffect, value: -30}},
}

func TestSoundControlsYieldBeforeFollowingStatement(t *testing.T) {
	for _, operation := range soundOperations {
		for _, warp := range []bool{false, true} {
			t.Run(operation.name+"/"+map[bool]string{false: "normal", true: "warp"}[warp], func(t *testing.T) {
				scheduler := newSoundScheduler(t)
				target := new(soundTarget)
				var order []string
				var frames []int64
				music := scheduler.Create("music", func(thread coroutine.Thread) {
					thread.SetWarp(warp)
					// A plain Go loop has no generated loop wait. Each sound
					// operation must suspend before the following statement.
					for range 2 {
						operation.call(target)
						order = append(order, "music")
						frames = append(frames, clock.Frame())
					}
				})
				scheduler.JoinFirstSlice(music)
				if want := []soundCall{operation.want}; !slices.Equal(target.calls, want) || len(order) != 0 {
					t.Fatalf("first slice: calls = %v, following statements = %v; want %v before yielding", target.calls, order, want)
				}

				telescope := scheduler.Create("telescope", func(coroutine.Thread) {
					order = append(order, "telescope")
				})
				scheduler.Join(telescope)
				scheduler.Update()
				if want := []string{"telescope"}; !slices.Equal(order, want) {
					t.Fatalf("same-frame order = %v, want %v", order, want)
				}

				for frame := int64(1); frame <= 2; frame++ {
					clock.Update(1.0/30, 30)
					scheduler.Update()
					if len(frames) != int(frame) || frames[len(frames)-1] != frame {
						t.Fatalf("frame %d: subsequent statements ran at %v", frame, frames)
					}
				}
				scheduler.Join(music)
				if want := []string{"telescope", "music", "music"}; !slices.Equal(order, want) {
					t.Fatalf("script order = %v, want %v", order, want)
				}
				if want := []soundCall{operation.want, operation.want}; !slices.Equal(target.calls, want) {
					t.Fatalf("sound calls = %v, want %v", target.calls, want)
				}
			})
		}
	}
}

func TestSoundControlsOutsideCoroutine(t *testing.T) {
	for _, scheduler := range []bool{false, true} {
		t.Run(map[bool]string{false: "no scheduler", true: "bound scheduler"}[scheduler], func(t *testing.T) {
			if scheduler {
				newSoundScheduler(t)
			}
			for _, operation := range soundOperations {
				t.Run(operation.name, func(t *testing.T) {
					target := new(soundTarget)
					operation.call(target)
					if want := []soundCall{operation.want}; !slices.Equal(target.calls, want) {
						t.Fatalf("calls = %v, want %v", target.calls, want)
					}
				})
			}
		})
	}
}

func newSoundScheduler(t *testing.T) *coroutine.Scheduler {
	t.Helper()
	clock.Start()
	scheduler := coroutine.New(func(report coroutine.PanicReport) {
		t.Errorf("unexpected coroutine panic: %v", report.Value)
	})
	engine.SetScheduler(scheduler)
	t.Cleanup(func() {
		scheduler.RunAfterStop(nil)
		engine.SetScheduler(nil)
		clock.Start()
	})
	return scheduler
}
