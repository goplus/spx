//go:build !js
// +build !js

package audiorecord

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/goplus/spx/internal/coroutine"
)

const (
	// is the audio sample rate (in hertz) for incoming and
	audioSampleRate uint32 = 16000
	// is the number of audio frames that should be sent in a 10ms window.
	audioFrameSize = 6 * audioSampleRate / 100

	// audioDefaultInterval is the default interval that audio packets are sent
	audioInterval = 6 * 10 * time.Millisecond

	MAV_VOLUME float64 = 39.11730073691797
)

func doubleCalculateVolume(buffer []int16) float64 {
	sumVolume := 0.0
	avgVolume := 0.0
	volume := 0.0
	for i := 0; i < len(buffer); i += 1 {
		temp := buffer[i]
		if int(temp) >= 0x8000 {
			temp = int16(0xffff - int(temp))
		}
		sumVolume += math.Abs(float64(temp))
	}
	avgVolume = sumVolume / float64(len(buffer)) / 2.0
	volume = math.Log10(1+avgVolume) * 10
	return volume / MAV_VOLUME
}

type Recorder struct {
	deviceVolume float64
	device       *CaptureDevice
}

func Open(gco *coroutine.Coroutines) *Recorder {
	device := CaptureOpenDevice("", audioSampleRate, FormatMono16, audioFrameSize)
	device.CaptureStart()
	p := &Recorder{device: device}
	gco.CreateAndStart(false, nil, func(me coroutine.Thread) int {
		for {
			fsize := audioFrameSize
			buff := device.CaptureSamples(uint32(fsize))
			if len(buff) != int(fsize)*2 {
				continue
			}

			int16Buffer := make([]int16, fsize)
			for i := range int16Buffer {
				int16Buffer[i] = int16(binary.LittleEndian.Uint16(buff[i*2 : (i+1)*2]))
			}
			p.deviceVolume = doubleCalculateVolume(int16Buffer)
			time.Sleep(audioInterval)
		}
	})
	return p
}

func (p *Recorder) Close() error {
	if p.device != nil {
		p.device.CaptureStop()
		p.device = nil
	}
	return nil
}

func (p *Recorder) Loudness() float64 {
	return p.deviceVolume
}
