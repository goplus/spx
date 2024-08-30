package common

import (
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type AnimExportFrame struct {
	RederOrder []int
	Meshes     []AnimExportMesh
}

type AnimExportMesh struct {
	Indices  []uint16         `json:"Indices"`
	Uvs      []math32.Vector2 `json:"Uvs"`
	Vertices []math32.Vector3 `json:"Vertices"`
}

type Avatar struct {
	Image *ebiten.Image
	// transform
	Scale  math32.Vector2
	Offset math32.Vector2

	LogicVertices [][]math32.Vector3
	// render data
	RenderBones    []math32.Vector2
	RenderVerteies [][]ebiten.Vertex
	RenderIndeies  [][]uint16
	RederOrder     []int
}

type Animator struct {
	Position math32.Vector2

	// runtime data
	Clips       map[string]IAnimClip
	CurClipName string
	ClipStates  map[string]*AnimClipState

	Avatar
}

func (pself *Animator) GetClipState(clipName string) *AnimClipState {
	if !pself.HasClip(clipName) {
		return nil
	}
	if pself.ClipStates == nil {
		pself.ClipStates = make(map[string]*AnimClipState)
	}
	if _, ok := pself.ClipStates[clipName]; !ok {
		clip := pself.Clips[clipName]
		pself.ClipStates[clipName] = &AnimClipState{
			AnimClipConfig: clip.GetConfig(),
			FrameCount:     clip.GetFramesCount(),
			Speed:          1,
			Time:           0,
		}
	}
	return pself.ClipStates[clipName]
}
func (pself *Animator) SetPosition(pos math32.Vector2) {
	pself.Position = pos
}

func (pself *Animator) GetClips() []string {
	clips := make([]string, 0)
	for k := range pself.Clips {
		clips = append(clips, k)
	}
	return clips
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

func (pself *Animator) Play(clipName string) *AnimClipState {
	if !pself.HasClip(clipName) {
		return nil
	}
	if pself.CurClipName == clipName {
		return pself.GetClipState(clipName)
	}
	pself.CurClipName = clipName
	state := pself.GetClipState(clipName)
	state.Time = 0
	state.Speed = 1
	return state
}

func (pself *Animator) Local2World(v math32.Vector2) math32.Vector2 {
	vt := v.Multiply(&pself.Scale).Add(&pself.Offset)
	vt.Y = -vt.Y
	return *vt.Add(&pself.Position)
}
