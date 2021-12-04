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
	io.Reader
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

type soundMgr struct {
	audioContext *audio.Context
	players      map[*audio.Player]chan bool
	playersM     sync.Mutex
}

const (
	defaultSampleRate = 44100
	defaultRatio      = 100.0
)

func (p *soundMgr) addPlayer(sp *audio.Player, done chan bool) {
	p.playersM.Lock()
	defer p.playersM.Unlock()

	p.players[sp] = done
}

func (p *soundMgr) init() {
	audioContext := audio.NewContext(defaultSampleRate)
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
	d, _, err := qaudio.Decode(newReadSeeker(source))
	if err != nil {
		source.Close()
		return
	}

	d = convert.ToStereo16(d)
	d = convert.Resample(d, audioContext.SampleRate())
	sp, err := audioContext.NewPlayer(&readCloser{d, source})
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
	sp.Play()
	if waitDone {
		waitForChan(done)
	}
	return
}

func (p *soundMgr) volume() float64 {
	for sp, _ := range p.players {
		return sp.Volume() * defaultRatio
	}
	return 0
}
func (p *soundMgr) SetVolume(volume float64) {
	for sp, _ := range p.players {
		sp.SetVolume(volume / defaultRatio)
	}
	return
}
func (p *soundMgr) ChangeVolume(delta float64) {
	v := p.volume()
	for sp, _ := range p.players {
		sp.SetVolume((v + delta) / defaultRatio)
	}
	return
}

// -------------------------------------------------------------------------------------
