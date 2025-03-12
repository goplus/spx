package audiorecord

// TODO(tanjp): implement this
import (
	"github.com/goplus/spx/internal/coroutine"
)

const (
	VOLUMEMAX = 32767.0
	VOLUMEMIN = -32768.0
)

type Recorder struct {
}

func Open(gco *coroutine.Coroutines) *Recorder {
	panic("audio recorder is not implemented yet.")
}

func (p *Recorder) Close() error {
	panic("audio recorder is not implemented yet.")
}

func (p *Recorder) Loudness() float64 {
	panic("audio recorder is not implemented yet.")
}
