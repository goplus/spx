package state

type SpriteRuntimeState struct {
	IsVisible           bool
	Cloned              bool
	IsDying             bool
	IsDirty             bool
	HasOnCloned         bool
	HasOnTouchStart     bool
	HasOnTouching       bool
	HasOnTouchEnd       bool
	DefaultCostumeIndex int
}
