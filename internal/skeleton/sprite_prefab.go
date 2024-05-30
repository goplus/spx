package skeleton

import (
	"fmt"

	"github.com/goplus/spx/internal/math32"
)

type SpritePrefabData struct {
	Name      string           `json:"Name"`
	Hierarchy []HierarchyData  `json:"Hierarchy"`
	SkinMesh  []SpriteSkinData `json:"SkinMesh"`
}

type HierarchyData struct {
	Name   string         `json:"Name"`
	PosRot math32.Vector3 `json:"PosRot"`
	Order  int            `json:"Order"`
	Parent string         `json:"Parent"`
}

func (h HierarchyData) String() string {
	return fmt.Sprintf("Name : %s, PosRot : %v, Order : %d, Parent : %s", h.Name, h.PosRot, h.Order, h.Parent)
}

type SpriteSkinData struct {
	Name           string           `json:"Name"`
	Indices        []uint16         `json:"Indices"`
	Vertices       []math32.Vector3 `json:"Vertices"`
	BoneWeights    []BoneWeight     `json:"BoneWeights"`
	BindPoses      []math32.Matrix4 `json:"BindPoses"`
	BoneTransforms []string         `json:"BoneTransforms"`
}

type BoneWeight struct {
	Weight0    float64 `json:"m_Weight0"`
	Weight1    float64 `json:"m_Weight1"`
	Weight2    float64 `json:"m_Weight2"`
	Weight3    float64 `json:"m_Weight3"`
	BoneIndex0 int     `json:"m_BoneIndex0"`
	BoneIndex1 int     `json:"m_BoneIndex1"`
	BoneIndex2 int     `json:"m_BoneIndex2"`
	BoneIndex3 int     `json:"m_BoneIndex3"`
}

func (s *SpriteSkinData) Deform(rootInv *math32.Matrix4, name2Trans map[string]*Bone, deformed []math32.Vector3) {
	boneTransformsMatrix := make([]*math32.Matrix4, len(name2Trans))
	if len(boneTransformsMatrix) == 0 {
		return
	}
	for i := 0; i < len(boneTransformsMatrix); i++ {
		if i >= len(s.BoneTransforms) {
			boneTransformsMatrix[i] = math32.NewMatrix4Indentity()
		} else {
			boneTransformsMatrix[i] = name2Trans[s.BoneTransforms[i]].GetLocal2WorldMatrix()
		}
	}

	for i := 0; i < len(boneTransformsMatrix); i++ {
		var bindPoseMat *math32.Matrix4
		if i >= len(s.BoneTransforms) {
			bindPoseMat = math32.NewMatrix4Indentity()
		} else {
			bindPoseMat = &s.BindPoses[i]
		}
		boneTransformMat := boneTransformsMatrix[i]
		boneTransformsMatrix[i] = rootInv.Mul__1(boneTransformMat.Mul__1(bindPoseMat))
	}
	if len(s.Vertices) != len(s.BoneWeights) {
		panic(fmt.Sprintf("Vertices and BoneWeights must have the same length %d != %d", len(s.Vertices), len(s.BoneWeights)))
	}

	for i := 0; i < len(s.Vertices); i++ {
		bone0 := s.BoneWeights[i].BoneIndex0
		bone1 := s.BoneWeights[i].BoneIndex1
		bone2 := s.BoneWeights[i].BoneIndex2
		bone3 := s.BoneWeights[i].BoneIndex3

		vertex := s.Vertices[i]
		point := boneTransformsMatrix[bone0].Mul__0(vertex).Scale(s.BoneWeights[i].Weight0).
			Add(boneTransformsMatrix[bone1].Mul__0(vertex).Scale(s.BoneWeights[i].Weight1)).
			Add(boneTransformsMatrix[bone2].Mul__0(vertex).Scale(s.BoneWeights[i].Weight2)).
			Add(boneTransformsMatrix[bone3].Mul__0(vertex).Scale(s.BoneWeights[i].Weight3))
		deformed[i] = *point
	}

}
