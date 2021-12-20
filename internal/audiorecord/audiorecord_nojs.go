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

	VOLUMEMAX = 32767.0
	VOLUMEMIN = -32768.0
)

type Recorder struct {
	deviceVolume float64
	device       *CaptureDevice
	lastValue    float64
}

func Open(gco *coroutine.Coroutines) *Recorder {
	device := CaptureOpenDevice("", audioSampleRate, FormatMono16, audioFrameSize)
	device.CaptureStart()
	p := &Recorder{
		device:       device,
		deviceVolume: 0,
	}
	gco.CreateAndStart(true, nil, func(me coroutine.Thread) int {
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
			p.deviceVolume = p.doubleCalculateVolume(int16Buffer)
			gco.Sleep(audioInterval)
		}
	})
	return p
}

//loudness scaled 0 to 100
func (p *Recorder) doubleCalculateVolume(buffer []int16) float64 {

	var sum float64 = 0
	// compute the RMS of the sound
	for i := 0; i < len(buffer); i++ {
		// higher/lower values exceed 16bit
		val := math.Min(VOLUMEMAX, float64(buffer[i]))
		val = math.Max(VOLUMEMIN, val)
		val = val / VOLUMEMAX
		sum += math.Pow(val, 2)
	}
	rms := math.Sqrt(sum / float64(len(buffer)))
	// smooth the value, if it is descending
	if p.lastValue != 0 {
		rms = math.Max(rms, p.lastValue*0.6)
	}
	p.lastValue = rms

	// Scale the measurement so it's more sensitive to quieter sounds
	rms *= 1.63
	rms = math.Sqrt(rms)
	// Scale it up to 0-100 and round
	rms = math.Round(rms * 100)
	//log.Printf("rms %f", rms)
	// Prevent it from going above 100
	rms = math.Min(rms, 100)

	return rms / 100.0
}

func (p *Recorder) Close() error {
	if p.device != nil {
		p.device.CaptureStop()
		p.device = nil
	}
	p.deviceVolume = 0
	return nil
}

func (p *Recorder) Loudness() float64 {
	return p.deviceVolume
}
