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

package audio

import (
	"math"
	"testing"

	"github.com/goplus/spx/v2/internal/engine"
)

type fakeBackend struct {
	nextID      int64
	plays       []playCall
	pauses      []int64
	resumes     []int64
	stops       []int64
	restarts    []int64
	loops       []loopCall
	volumes     []float64
	destroys    []engine.Object
	lastDestroy engine.Object
	pan         float64
	pitch       float64
	volume      float64
	playing     map[int64]bool
}

type playCall struct {
	path       string
	owner      engine.Object
	attenation float64
	maxDist    float64
}

type loopCall struct {
	id   int64
	loop bool
}

func (f *fakeBackend) CreateAudio() engine.Object {
	return 77
}

func (f *fakeBackend) DestroyAudio(obj engine.Object) {
	f.lastDestroy = obj
	f.destroys = append(f.destroys, obj)
}

func (f *fakeBackend) SetPitch(obj engine.Object, pitch float64) {
	f.pitch = pitch
}

func (f *fakeBackend) GetPitch(obj engine.Object) float64 {
	return f.pitch
}

func (f *fakeBackend) SetPan(obj engine.Object, pan float64) {
	f.pan = pan
}

func (f *fakeBackend) GetPan(obj engine.Object) float64 {
	return f.pan
}

func (f *fakeBackend) SetVolume(obj engine.Object, volume float64) {
	f.volume = volume
	f.volumes = append(f.volumes, volume)
}

func (f *fakeBackend) GetVolume(obj engine.Object) float64 {
	return f.volume
}

func (f *fakeBackend) PlayWithAttenuation(obj engine.Object, path string, ownerID engine.Object, attenuation, maxDistance float64) int64 {
	f.nextID++
	if f.playing == nil {
		f.playing = make(map[int64]bool)
	}
	f.playing[f.nextID] = true
	f.plays = append(f.plays, playCall{
		path:       path,
		owner:      ownerID,
		attenation: attenuation,
		maxDist:    maxDistance,
	})
	return f.nextID
}

func (f *fakeBackend) Pause(aid int64) {
	f.pauses = append(f.pauses, aid)
}

func (f *fakeBackend) Resume(aid int64) {
	f.resumes = append(f.resumes, aid)
}

func (f *fakeBackend) Stop(aid int64) {
	if f.playing != nil {
		f.playing[aid] = false
	}
	f.stops = append(f.stops, aid)
}

func (f *fakeBackend) Restart(aid int64) bool {
	if f.playing == nil || !f.playing[aid] {
		return false
	}
	f.restarts = append(f.restarts, aid)
	return true
}

func (f *fakeBackend) SetLoop(aid int64, loop bool) {
	f.loops = append(f.loops, loopCall{id: aid, loop: loop})
}

func (f *fakeBackend) IsPlaying(aid int64) bool {
	return f.playing[aid]
}

func (f *fakeBackend) StopAll() {
	f.playing = make(map[int64]bool)
}

func TestManagerPlayPauseResumeStop(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id1 := mgr.Play(1, "sounds/a.wav", false, false, 9, 0, 100)
	id2 := mgr.Play(1, "sounds/a.wav", true, false, 9, 12, 100)

	if id1 != 1 || id2 != 2 {
		t.Fatalf("Play ids = %d, %d; want 1, 2", id1, id2)
	}
	if backend.plays[0].path != engine.ToAssetPath("sounds/a.wav") {
		t.Fatalf("Play path = %q, want %q", backend.plays[0].path, engine.ToAssetPath("sounds/a.wav"))
	}
	if backend.plays[0].owner != 0 {
		t.Fatalf("owner with zero attenuation = %d, want 0", backend.plays[0].owner)
	}
	if backend.plays[1].owner != 9 {
		t.Fatalf("owner with attenuation = %d, want 9", backend.plays[1].owner)
	}

	mgr.Pause("sounds/a.wav")
	mgr.Resume("sounds/a.wav")
	mgr.Stop("sounds/a.wav")

	if len(backend.pauses) != 2 || len(backend.resumes) != 2 || len(backend.stops) != 2 {
		t.Fatalf("pause/resume/stop lens = %d/%d/%d, want 2/2/2", len(backend.pauses), len(backend.resumes), len(backend.stops))
	}
	if len(backend.loops) != 1 || backend.loops[0].id != 2 {
		t.Fatalf("SetLoop calls = %+v, want loop only on latest id", backend.loops)
	}
}

func TestManagerPlayPrunesFinishedIDs(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	first := mgr.Play(1, "sounds/a.wav", false, false, 0, 0, 0)
	backend.playing[first] = false

	second := mgr.Play(1, "sounds/a.wav", false, false, 0, 0, 0)
	mgr.Pause("sounds/a.wav")

	if len(mgr.path2ids["sounds/a.wav"]) != 1 || mgr.path2ids["sounds/a.wav"][0] != second {
		t.Fatalf("path2ids = %+v, want only latest live id", mgr.path2ids["sounds/a.wav"])
	}
	if len(backend.pauses) != 1 || backend.pauses[0] != second {
		t.Fatalf("Pause calls = %+v, want [%d]", backend.pauses, second)
	}
}

func TestManagerStopIDOnlyStopsRequestedPlayback(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	first := mgr.Play(1, "sounds/a.wav", true, false, 0, 0, 0)
	second := mgr.Play(1, "sounds/a.wav", true, false, 0, 0, 0)

	mgr.StopID(first)
	mgr.Pause("sounds/a.wav")

	if len(backend.stops) != 1 || backend.stops[0] != first {
		t.Fatalf("Stop calls = %+v, want [%d]", backend.stops, first)
	}
	if len(mgr.path2ids["sounds/a.wav"]) != 1 || mgr.path2ids["sounds/a.wav"][0] != second {
		t.Fatalf("path2ids = %+v, want only second playback %d", mgr.path2ids["sounds/a.wav"], second)
	}
	if len(backend.pauses) != 1 || backend.pauses[0] != second {
		t.Fatalf("Pause calls = %+v, want [%d]", backend.pauses, second)
	}
}

func TestManagerPruneStoppedIDsRemovesStalePathEntries(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	first := mgr.Play(1, "sounds/a.wav", false, false, 0, 0, 0)
	second := mgr.Play(1, "sounds/a.wav", false, false, 0, 0, 0)
	backend.playing[first] = false

	live := mgr.PruneStoppedIDs([]int64{first, second})

	if len(live) != 1 || live[0] != second {
		t.Fatalf("live = %+v, want [%d]", live, second)
	}
	if len(mgr.path2ids["sounds/a.wav"]) != 1 || mgr.path2ids["sounds/a.wav"][0] != second {
		t.Fatalf("path2ids = %+v, want only second playback %d", mgr.path2ids["sounds/a.wav"], second)
	}
}

func TestManagerRestartIDKeepsTrackedPlayback(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(1, "sounds/a.wav", true, false, 0, 0, 0)

	if !mgr.RestartID(id) {
		t.Fatalf("RestartID(%d) = false, want true", id)
	}
	if len(backend.restarts) != 1 || backend.restarts[0] != id {
		t.Fatalf("restart calls = %+v, want [%d]", backend.restarts, id)
	}
	if len(mgr.path2ids["sounds/a.wav"]) != 1 || mgr.path2ids["sounds/a.wav"][0] != id {
		t.Fatalf("path2ids = %+v, want [%d]", mgr.path2ids["sounds/a.wav"], id)
	}
}

func TestManagerRestartIDPrunesStalePlayback(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(1, "sounds/a.wav", true, false, 0, 0, 0)
	backend.playing[id] = false

	if mgr.RestartID(id) {
		t.Fatalf("RestartID(%d) = true, want false for stale playback", id)
	}
	if _, ok := mgr.path2ids["sounds/a.wav"]; ok {
		t.Fatalf("path2ids still contains stale playback %d", id)
	}
}

func TestManagerVolumeAndEffects(t *testing.T) {
	backend := &fakeBackend{pan: 0.25, pitch: 2, volume: 0.8}
	var mgr Manager
	mgr.Init(backend)

	if got := mgr.GetPan(1); got != 25 {
		t.Fatalf("GetPan = %v, want 25", got)
	}
	if got := mgr.GetPitch(1); !almostEqual(got, 120) {
		t.Fatalf("GetPitch = %v, want 120", got)
	}
	if got := mgr.GetVolume(1); got != 80 {
		t.Fatalf("GetVolume = %v, want 80", got)
	}

	mgr.SetPan(1, 40)
	mgr.SetPitch(1, 120)
	mgr.SetVolume(1, -5)

	if backend.pan != 0.4 {
		t.Fatalf("SetPan backend = %v, want 0.4", backend.pan)
	}
	if !almostEqual(backend.pitch, 2) {
		t.Fatalf("SetPitch backend = %v, want 2", backend.pitch)
	}
	if got := backend.volumes[len(backend.volumes)-1]; got != 0.01 {
		t.Fatalf("SetVolume backend = %v, want 0.01", got)
	}
}

func TestManagerPitchEffectUsesScratchScale(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	mgr.SetPitch(1, 0)
	if !almostEqual(backend.pitch, 1) {
		t.Fatalf("SetPitch(0) backend = %v, want 1", backend.pitch)
	}

	mgr.SetPitch(1, 10)
	want := math.Pow(2, 1.0/12)
	if !almostEqual(backend.pitch, want) {
		t.Fatalf("SetPitch(10) backend = %v, want %v", backend.pitch, want)
	}

	backend.pitch = want
	if got := mgr.GetPitch(1); !almostEqual(got, 10) {
		t.Fatalf("GetPitch = %v, want 10", got)
	}

	backend.pitch = 0
	if got := mgr.GetPitch(1); got != 0 {
		t.Fatalf("GetPitch with non-positive backend scale = %v, want 0", got)
	}
}

func TestManagerAllocAndRelease(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	if got := mgr.AllocSound(); got != 77 {
		t.Fatalf("AllocSound = %d, want 77", got)
	}

	mgr.ReleaseSound(13)
	if backend.lastDestroy != 13 {
		t.Fatalf("ReleaseSound destroyed = %d, want 13", backend.lastDestroy)
	}
}

func TestManagerReleaseSoundDefersDestroyUntilPlaybackStops(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(13, "sounds/a.wav", false, false, 0, 0, 0)

	mgr.ReleaseSound(13)
	if len(backend.destroys) != 0 {
		t.Fatalf("DestroyAudio calls = %+v, want none while playback %d is live", backend.destroys, id)
	}

	backend.playing[id] = false
	mgr.Update()

	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after playback stops", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerReleaseSoundDoesNotDestroyPendingSoundTwice(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(13, "sounds/a.wav", false, false, 0, 0, 0)

	mgr.ReleaseSound(13)
	backend.playing[id] = false
	mgr.ReleaseSound(13)
	mgr.Update()

	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want exactly [13]", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerReleaseSoundStopsLoopedPlaybackBeforeDestroy(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(13, "sounds/loop.wav", true, false, 0, 0, 0)

	mgr.ReleaseSound(13)

	if len(backend.stops) != 1 || backend.stops[0] != id {
		t.Fatalf("Stop calls = %+v, want [%d]", backend.stops, id)
	}
	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after loop playback stops", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerReleaseSoundStopsLoopsAndDefersOneShots(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	loopID := mgr.Play(13, "sounds/loop.wav", true, false, 0, 0, 0)
	oneShotID := mgr.Play(13, "sounds/hit.wav", false, false, 0, 0, 0)

	mgr.ReleaseSound(13)

	if len(backend.stops) != 1 || backend.stops[0] != loopID {
		t.Fatalf("Stop calls = %+v, want only loop playback %d stopped", backend.stops, loopID)
	}
	if len(backend.destroys) != 0 {
		t.Fatalf("DestroyAudio calls = %+v, want none while one-shot playback %d is live", backend.destroys, oneShotID)
	}
	if _, ok := mgr.playbacks[loopID]; ok {
		t.Fatalf("loop playback %d still tracked after release", loopID)
	}
	if _, ok := mgr.playbacks[oneShotID]; !ok {
		t.Fatalf("one-shot playback %d not tracked while still playing", oneShotID)
	}

	backend.playing[oneShotID] = false
	mgr.Update()

	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after one-shot playback stops", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerStopPathDestroysPendingReleasedSound(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(13, "sounds/a.wav", false, false, 0, 0, 0)
	mgr.ReleaseSound(13)

	mgr.Stop("sounds/a.wav")

	if len(backend.stops) != 1 || backend.stops[0] != id {
		t.Fatalf("Stop calls = %+v, want [%d]", backend.stops, id)
	}
	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after Stop", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerStopIDDestroysPendingReleasedSound(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	id := mgr.Play(13, "sounds/a.wav", false, false, 0, 0, 0)
	mgr.ReleaseSound(13)

	mgr.StopID(id)

	if len(backend.stops) != 1 || backend.stops[0] != id {
		t.Fatalf("Stop calls = %+v, want [%d]", backend.stops, id)
	}
	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after StopID", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func TestManagerStopAllDestroysPendingReleasedSounds(t *testing.T) {
	backend := &fakeBackend{}
	var mgr Manager
	mgr.Init(backend)

	mgr.Play(13, "sounds/a.wav", false, false, 0, 0, 0)
	mgr.ReleaseSound(13)
	mgr.StopAll()

	if len(backend.destroys) != 1 || backend.destroys[0] != 13 {
		t.Fatalf("DestroyAudio calls = %+v, want [13] after StopAll", backend.destroys)
	}
	assertManagerNoTracking(t, &mgr)
}

func assertManagerNoTracking(t *testing.T, mgr *Manager) {
	t.Helper()
	if len(mgr.path2ids) != 0 || len(mgr.obj2ids) != 0 || len(mgr.playbacks) != 0 || len(mgr.pendingDestroy) != 0 {
		t.Fatalf(
			"manager tracking not cleared: path2ids=%+v obj2ids=%+v playbacks=%+v pendingDestroy=%+v",
			mgr.path2ids,
			mgr.obj2ids,
			mgr.playbacks,
			mgr.pendingDestroy,
		)
	}
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}
