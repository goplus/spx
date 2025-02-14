package timeline

import "fmt"

type Interval struct {
	Offset   float64
	Duration float64
}

func (i Interval) End() float64 {
	return i.Offset + i.Duration
}

func (i *Interval) Step(time float64) {
	i.Offset -= time
}

func (i Interval) Scale(scale float32) Interval {
	return Interval{
		Offset:   float64(float64(i.Offset) * float64(scale)),
		Duration: float64(float64(i.Duration) * float64(scale)),
	}
}

func (i Interval) Contains(time float64) bool {
	return i.Offset <= time && time <= i.End()
}

func (i Interval) String() string {
	return fmt.Sprintf("Interval{offset:%.3f, duration:%.3f}", i.Offset, i.Duration)
}
