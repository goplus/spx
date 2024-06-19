package common

type IAnimClip interface {
	GetConfig() AnimClipConfig
	GetFramesCount() int
}
type AnimClip struct {
	Name       string
	Config     AnimClipConfig
	FrameCount int
}

func (pself *AnimClip) GetConfig() AnimClipConfig {
	return pself.Config
}
func (pself *AnimClip) GetFramesCount() int {
	return pself.FrameCount
}
