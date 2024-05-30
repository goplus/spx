package skeleton

import (
	"math"

	"github.com/goplus/spx/internal/math32"
)

type Skeleton struct {
	Bones []*Bone
}
type Bone struct {
	Name   string
	Parent *Bone
	Pos    math32.Vector2
	Deg    float64
}

func (t *Bone) Local2World(pos math32.Vector2) math32.Vector2 {
	rad := t.Deg * math.Pi / 180
	c := math.Cos(rad)
	s := math.Sin(rad)
	return math32.Vector2{
		X: t.Pos.X + c*pos.X + -s*pos.Y,
		Y: t.Pos.Y + s*pos.X + c*pos.Y,
	}
}

func BuildSkeleton(hierarchyData []HierarchyData) *Skeleton {
	skeleton := &Skeleton{}
	name2Bone := make(map[string]*Bone)
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
		bone.Pos = parent.Local2World(math32.Vector2{X: data.PosRot.X, Y: data.PosRot.Y})
		bone.Deg = parent.Deg + data.PosRot.Z
	}

	skeleton.Bones[0].Pos = math32.Vector2{}
	return skeleton
}
