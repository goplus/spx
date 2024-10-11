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
	"sync"

	"github.com/qiniu/audio/convert"
	_ "github.com/qiniu/audio/mp3"       // support mp3
	_ "github.com/qiniu/audio/wav"       // support wav/pcm
	_ "github.com/qiniu/audio/wav/adpcm" // support wav/adpcm

	"github.com/hajimehoshi/ebiten/v2/audio"

	qaudio "github.com/qiniu/audio"
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

func (p *soundMgr) playAction(sound Sound, opts *PlayOptions, wait bool) (err error) {
	switch opts.Action {
	case PlayRewind:
		err = p.play(sound, wait, opts.Loop)
	case PlayContinue:
		err = p.playContinue(sound, wait, opts.Loop)
	case PlayStop:
		p.stop(sound)
	case PlayResume:
		p.resume(sound)
	case PlayPause:
		p.pause(sound)
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

func (p *soundMgr) play(sound Sound, wait, loop bool) (err error) {
	source, err := p.g.fs.Open(sound.Path)
	if err != nil {
		panic(err)
	}

	audioContext := p.audioContext
	d, _, err := qaudio.Decode(newReadSeeker(source))
	if err != nil {
		source.Close()
		return
	}

	d = convert.ToStereo16(d)
	d = convert.Resample(d, audioContext.SampleRate())

	sp := &soundPlayer{media: sound, loop: loop}
	sp.Player, err = audioContext.NewPlayer(&readCloser{d, source})
	if err != nil {
		source.Close()
		return
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
