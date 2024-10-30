package timeline

type TimelineGroup struct {
	Timeline
	Tracks []ITimeline
}

func (tg *TimelineGroup) Step(time *float64) ITimeline {
	tg.Timeline.Step(time)
	return nil
}

// AddTrack 方法
func (tg *TimelineGroup) AddTrack(track ITimeline) {
	tg.Tracks = append(tg.Tracks, track)
}
