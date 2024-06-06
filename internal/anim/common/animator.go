package common

import (
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type IAnimClip interface {
}
type AnimClip struct {
	Name   string `json:"Name"`
	Config AnimClipConfig
}
type Animator struct {
	Position math32.Vector2
	Image    *ebiten.Image
	// transform
	Scale  math32.Vector2
	Offset math32.Vector2

	// runtime data
	Clips         map[string]IAnimClip
	CurClipName   string
	CurFrame      int
	LogicVertices [][]math32.Vector3

	// render data
	RenderBones    []math32.Vector2
	RenderVerteies [][]ebiten.Vertex
	RenderIndeies  [][]uint16
	RederOrder     []int
}

func (pself *Animator) SetPosition(pos math32.Vector2) {
	pself.Position = pos
}

func (pself *Animator) HasClip(name string) bool {
	if pself.Clips == nil {
		return false
	}
	if _, ok := pself.Clips[name]; ok {
		return true
	}
	return false
}
func (pself *Animator) Draw(screen *ebiten.Image) {
	//pself.drawBone(screen)
	op := &ebiten.DrawTrianglesOptions{}
	op.Address = ebiten.AddressUnsafe
	for k := 0; k < len(pself.RederOrder); k++ {
		i := pself.RederOrder[k] // render in correct order
		verteies := pself.RenderVerteies[i]
		indices := pself.RenderIndeies[i]
		screen.DrawTriangles(verteies, indices, pself.Image, op)
	}
}

func (pself *Animator) Play(clipName string) {
	if pself.CurClipName == clipName {
		return
	}
	if !pself.HasClip(clipName) {
		return
	}
	pself.CurClipName = clipName
	pself.CurFrame = 0
}

func (pself *Animator) Local2World(v math32.Vector2) math32.Vector2 {
	vt := v.Multiply(&pself.Scale).Add(&pself.Offset)
	vt.Y = -vt.Y
	return *vt.Add(&pself.Position)
}
