//go:build !js
// +build !js

package audiorecord

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/goplus/spx/internal/coroutine"
)

var device *CaptureDevice
var deviceIsStart bool
var co *coroutine.Coroutines
var deviceVolume float64

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

func init() {

	device = CaptureOpenDevice("", audioSampleRate, FormatMono16, audioFrameSize)
	co = coroutine.New()
}

func StartRecorder() {
	if deviceIsStart == true {
		return
	}
	device.CaptureStart()
	deviceIsStart = true

	co.CreateAndStart(true, nil, func(me coroutine.Thread) int {
		for {
			if deviceIsStart == false {
				return 0
			}

			fsize := audioFrameSize
			buff := device.CaptureSamples(uint32(fsize))
			if len(buff) != int(fsize)*2 {
				continue
			}

			int16Buffer := make([]int16, fsize)
			for i := range int16Buffer {
				int16Buffer[i] = int16(binary.LittleEndian.Uint16(buff[i*2 : (i+1)*2]))
			}
			deviceVolume = doubleCalculateVolume(int16Buffer)
			//log.Printf("deviceVolume %f", deviceVolume)
			time.Sleep(audioInterval)
		}
	})

}

func StopRecorder() {
	if deviceIsStart == false {
		return
	}
	if device != nil {
		device.CaptureStop()
	}
	//co.Abort()
	deviceIsStart = false
}

func Loudness() float64 {
	return deviceVolume
}
