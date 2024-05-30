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
