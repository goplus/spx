package skeleton

import (
	"math"

	"github.com/goplus/spx/internal/math32"
)

type Skeleton struct {
	Bones     []*Bone
	Name2Bone map[string]*Bone
}
type Bone struct {
	Name   string
	Parent *Bone
	Pos    math32.Vector2
	Deg    float64

	LocalPos math32.Vector2
	LocalDeg float64
}

func (pself *Bone) getLocal2WorldMatrix() *math32.Matrix4 {
	rad := pself.Deg * math.Pi / 180
	c := math.Cos(rad)
	s := math.Sin(rad)
	return &math32.Matrix4{
		c, -s, 0, pself.Pos.X,
		s, c, 0, pself.Pos.Y,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}
func (pself *Bone) local2World(pos math32.Vector2) math32.Vector2 {
	rad := pself.Deg * math.Pi / 180
	c := math.Cos(rad)
	s := math.Sin(rad)
	return math32.Vector2{
		X: pself.Pos.X + c*pos.X + -s*pos.Y,
		Y: pself.Pos.Y + s*pos.X + c*pos.Y,
	}
}

func (pself *Skeleton) updateSkeleton(rootPos math32.Vector2, rootDeg float64) {
	for _, bone := range pself.Bones {
		if bone.Parent == nil {
			bone.Pos = rootPos
			bone.Deg = rootDeg
			continue
		}
		bone.Pos = bone.Parent.local2World(bone.LocalPos)
		bone.Deg = bone.Parent.Deg + bone.LocalDeg
	}
}

func buildSkeleton(hierarchyData []hierarchyData) *Skeleton {
	skeleton := &Skeleton{}
	name2Bone := make(map[string]*Bone)
	skeleton.Name2Bone = name2Bone
	for _, item := range hierarchyData {
		goObj := &Bone{Name: item.Name}
		skeleton.Bones = append(skeleton.Bones, goObj)
		name2Bone[item.Name] = goObj
	}

	for i, data := range hierarchyData {
		if _, ok := name2Bone[data.Parent]; !ok {
			continue
		}
		parent := name2Bone[data.Parent]
		bone := skeleton.Bones[i]
		bone.Parent = parent
		bone.LocalPos = math32.Vector2{X: data.PosRot.X, Y: data.PosRot.Y}
		bone.LocalDeg = data.PosRot.Z
		bone.Pos = parent.local2World(bone.LocalPos)
		bone.Deg = parent.Deg + data.PosRot.Z
	}

	skeleton.Bones[0].Pos = math32.Vector2{}
	return skeleton
}
