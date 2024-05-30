package skeleton

type SpriteAnimData struct {
	AnimData []FrameData `json:"AnimData"`
}

type FrameData struct {
	PosDeg []float64 `json:"PosDeg"`
}
