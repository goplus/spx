package skeleton

import "github.com/goplus/spx/internal/math32"

type SpriteAnimator struct {
	Skeleton   *Skeleton
	AnimData   *SpriteAnimData
	PrefabData *SpritePrefabData

	CurrentFrame int
	Bones        []*Bone
	Vertices     [][]math32.Vector3
}

func NewSpriteAnimator(prefabData *SpritePrefabData, animData *SpriteAnimData) *SpriteAnimator {
	skel := BuildSkeleton(prefabData.Hierarchy)
	pself := &SpriteAnimator{
		Skeleton:   skel,
		AnimData:   animData,
		PrefabData: prefabData,
	}
	pself.Init()
	return pself
}

func (pself *SpriteAnimator) Init() {
	for i := 0; i < int(lastBone); i++ {
		if boneTrans, ok := pself.Skeleton.Name2Bone[EBoneNames(i).String()]; ok {
			pself.Bones = append(pself.Bones, boneTrans)
		}
	}
	pself.Vertices = make([][]math32.Vector3, 0)
	for _, skinData := range pself.PrefabData.SkinMesh {
		pself.Vertices = append(pself.Vertices, make([]math32.Vector3, len(skinData.Vertices)))
	}
}

func (pself *SpriteAnimator) Update() {
	if pself.AnimData == nil {
		return
	}

	animData := pself.AnimData.AnimData
	if len(animData) == 0 {
		return
	}

	frame := animData[pself.CurrentFrame]
	for i := 0; i < len(pself.Bones); i++ {
		offset := i * 3
		pos := math32.NewVector2Zero()
		pos.X = frame.PosDeg[offset+0]
		pos.Y = frame.PosDeg[offset+1]
		pself.Bones[i].LocalPos = *pos
		pself.Bones[i].LocalDeg = frame.PosDeg[offset+2]
	}
	pself.Skeleton.UpdateSkeleton(math32.Vector2{}, 0)

	for i, skinData := range pself.PrefabData.SkinMesh {
		skinData.Deform(math32.NewMatrix4Indentity(), pself.Skeleton.Name2Bone, pself.Vertices[i])
	}
	pself.CurrentFrame = (pself.CurrentFrame + 1) % len(animData)
}
