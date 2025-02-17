//go:build android
// +build android

package audiorecord

import (
	"github.com/goplus/spx/internal/coroutine"
)

const (
	VOLUMEMAX = 32767.0
	VOLUMEMIN = -32768.0
)

type Recorder struct {
	deviceVolume float64
	lastValue    float64
}

func Open(gco *coroutine.Coroutines) *Recorder {
	p := &Recorder{
		deviceVolume: 0,
	}

	return p
}

func (p *Recorder) Close() error {

	p.deviceVolume = 0
	return nil
}

func (p *Recorder) Loudness() float64 {
	return p.deviceVolume
}
