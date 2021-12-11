package spx

import (
	"github.com/goplus/spx/internal/audiorecord"
)

type AudioRecord struct {
}

func NewAudioRecord() *AudioRecord {
	return &AudioRecord{}
}

func (a *AudioRecord) StartRecord() {
	audiorecord.StartRecorder()
}
func (a *AudioRecord) StopRecord() {
	audiorecord.StopRecorder()
}
func (a *AudioRecord) Loudness() float64 {
	return audiorecord.GetVolume()
}
