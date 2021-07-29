/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"io"
	"sync"

	"github.com/qiniu/audio/convert"
	_ "github.com/qiniu/audio/mp3"       // support mp3
	_ "github.com/qiniu/audio/wav"       // support wav/pcm
	_ "github.com/qiniu/audio/wav/adpcm" // support wav/adpcm

	"github.com/hajimehoshi/ebiten/audio"

	qaudio "github.com/qiniu/audio"
)

// -------------------------------------------------------------------------------------

type readSeekCloser struct {
	io.ReadCloser
}

type readCloser struct {
	io.Reader
	io.Closer
}

func (p *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	panic("can't seek")
}

// -------------------------------------------------------------------------------------

type soundMgr struct {
	audioContext *audio.Context
	players      map[*audio.Player]chan bool
	playersM     sync.Mutex
}

const (
	defaultSampleRate = 44100
)

func (p *soundMgr) addPlayer(sp *audio.Player, done chan bool) {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	p.players[sp] = done
}

func (p *soundMgr) init() {
	audioContext, err := audio.NewContext(defaultSampleRate)
	if err != nil {
		panic(err)
	}
	p.audioContext = audioContext
	p.players = make(map[*audio.Player]chan bool)
}

func (p *soundMgr) update() {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	var closed []*audio.Player
	for sp, done := range p.players {
		if !sp.IsPlaying() {
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

	closed := make([]*audio.Player, 0, len(p.players))
	for sp, done := range p.players {
		sp.Close()
		if done != nil {
			done <- true
		}
		closed = append(closed, sp)
	}
	for _, sp := range closed {
		delete(p.players, sp)
	}
}

func (p *soundMgr) play(source io.ReadCloser, wait ...bool) (err error) {
	audioContext := p.audioContext
	d, _, err := qaudio.Decode(&readSeekCloser{source})
	if err != nil {
		source.Close()
		return
	}

	d = convert.ToStereo16(d)
	d = convert.Resample(d, audioContext.SampleRate())
	sp, err := audio.NewPlayer(audioContext, &readCloser{d, source})
	if err != nil {
		source.Close()
		return
	}

	var waitDone = (wait != nil)
	var done chan bool
	if waitDone {
		done = make(chan bool, 1)
	}
	p.addPlayer(sp, done)
	err = sp.Play()
	if waitDone {
		waitForChan(done)
	}
	return
}

// -------------------------------------------------------------------------------------
