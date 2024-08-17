/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"io"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// -------------------------------------------------------------------------------------

type readSeekCloser struct {
	io.ReadCloser
}

type readCloser struct {
	io.ReadSeeker
	io.Closer
}

func (p *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	panic("can't seek")
}

func newReadSeeker(source io.ReadCloser) io.ReadSeeker {
	if r, ok := source.(io.ReadSeeker); ok {
		return r
	}
	return &readSeekCloser{source}
}

// -------------------------------------------------------------------------------------

type playerState byte

const (
	playerPlay playerState = iota
	playerClosed
	playerPaused
)

type PlayAction int

const (
	PlayRewind PlayAction = iota
	PlayContinue
	PlayPause
	PlayResume
	PlayStop
)

type PlayOptions struct {
	Action PlayAction
	Wait   bool
	Loop   bool
}

type soundPlayer struct {
	*audio.Player
	media Sound
	state playerState
	loop  bool
}

type soundMgr struct {
	g            *Game
	audioContext *audio.Context
	players      map[*soundPlayer]chan bool
	playersM     sync.Mutex
	audios       map[string]Sound
}

const (
	defaultSampleRate = 44100
	defaultRatio      = 100.0
)

func (p *soundMgr) addPlayer(sp *soundPlayer, done chan bool) {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	p.players[sp] = done
}

func (p *soundMgr) init(g *Game) {
	audioContext := audio.NewContext(defaultSampleRate)
	p.audioContext = audioContext
	p.players = make(map[*soundPlayer]chan bool)
	p.g = g
	p.audios = make(map[string]Sound)

	// media formats supported
	formats = make([]format, 4)
	formats[0].name = "mp3"
	formats[0].magic = "ID3"
	formats[1].name = "mp3"
	formats[1].magic = "\xff\xfb"
	formats[2].name = "ogg"
	formats[2].magic = "OggS"
	formats[3].name = "wav"
	formats[3].magic = "RIFF????WAVE"
}

func (p *soundMgr) update() {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	var closed []*soundPlayer
	for sp, done := range p.players {
		if !sp.IsPlaying() && sp.state != playerPaused {
			if sp.loop {
				sp.Rewind()
				sp.Play()
				continue
			}
			sp.Close()
			if done != nil {
				done <- true
			}
			closed = append(closed, sp)
		}
	}
	for _, sp := range closed {
		delete(p.players, sp)
	}
}

func (p *soundMgr) stopAll() {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	closed := make([]*soundPlayer, 0, len(p.players))
	for sp, done := range p.players {
		sp.Close()
		if done != nil {
			done <- true
		}
		sp.state = playerClosed
		closed = append(closed, sp)
	}
	for _, sp := range closed {
		delete(p.players, sp)
	}
}

func (p *soundMgr) playAction(media Sound, opts *PlayOptions) (err error) {
	switch opts.Action {
	case PlayRewind:
		err = p.play(media, opts.Wait, opts.Loop)
	case PlayContinue:
		err = p.playContinue(media, opts.Wait, opts.Loop)
	case PlayStop:
		p.stop(media)
	case PlayResume:
		p.resume(media)
	case PlayPause:
		p.pause(media)
	}
	return
}

func (p *soundMgr) playContinue(media Sound, wait, loop bool) (err error) {
	p.playersM.Lock()
	found := false
	for sp := range p.players {
		if sp.media.Path == media.Path {
			sp.loop = loop
			found = true
		}
	}
	p.playersM.Unlock()

	if !found {
		err = p.play(media, wait, loop)
	}
	return
}

type format struct {
	name, magic string
}

var formats []format

func (p *soundMgr) play(media Sound, wait, loop bool) (err error) {
	source, err := p.g.fs.Open(media.Path)
	if err != nil {
		panic(err)
	}

	audioContext := p.audioContext
	sp := &soundPlayer{media: media, loop: loop}
	parts := strings.Split(media.Path, ".")
	l := len(parts)
	ext := ""
	if l >= 1 {
		ext = parts[l-1]
	}

	switch ext {
	case "mp3":
		var ms *mp3.Stream
		ms, err = mp3.DecodeF32(source)
		if err != nil {
			return err
		}
		sp.Player, err = audioContext.NewPlayerF32(ms)
	case "ogg":
		var vs *vorbis.Stream
		vs, err = vorbis.DecodeF32(source)
		if err != nil {
			return err
		}
		sp.Player, err = audioContext.NewPlayerF32(vs)
	case "wav":
		var ws *wav.Stream
		ws, err = wav.DecodeF32(source)
		if err != nil {
			return err
		}
		sp.Player, err = audioContext.NewPlayerF32(ws)
	}

	if err != nil {
		source.Close()
		return err
	}

	var done chan bool
	if wait {
		done = make(chan bool, 1)
	}
	p.addPlayer(sp, done)
	sp.Play()
	if wait {
		waitForChan(done)
	}
	return
}

func (p *soundMgr) stop(media Sound) {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	closed := make([]*soundPlayer, 0, len(p.players))
	for sp, done := range p.players {
		if sp.media.Path == media.Path {
			sp.Close()
			if done != nil {
				done <- true
			}
			sp.state = playerClosed
			closed = append(closed, sp)
		}
	}
	for _, sp := range closed {
		delete(p.players, sp)
	}
}

func (p *soundMgr) pause(media Sound) {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	for sp := range p.players {
		if sp.media.Path == media.Path {
			sp.Pause()
			sp.state = playerPaused

		}

	}
}

func (p *soundMgr) resume(media Sound) {
	p.playersM.Lock()
	defer p.playersM.Unlock()
	for sp := range p.players {
		if sp.media.Path == media.Path {
			sp.Play()
			sp.state = playerPlay

		}

	}
}

func (p *soundMgr) volume() float64 {
	for sp := range p.players {
		return sp.Volume() * defaultRatio
	}
	return 0
}

func (p *soundMgr) SetVolume(volume float64) {
	for sp := range p.players {
		sp.SetVolume(volume / defaultRatio)
	}
}

func (p *soundMgr) ChangeVolume(delta float64) {
	v := p.volume()
	for sp := range p.players {
		sp.SetVolume((v + delta) / defaultRatio)
	}
}

// -------------------------------------------------------------------------------------
