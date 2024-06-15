package skeleton

import (
	"fmt"

	"github.com/goplus/spx/internal/anim/common"
	"github.com/goplus/spx/internal/math32"
)

type AnimClip struct {
	common.AnimClip
	Data spriteAnimData
}

type AnimMesh struct {
	Name        string           `json:"Name"`
	Hierarchy   []hierarchyData  `json:"Hierarchy"`
	SkinMesh    []spriteSkinData `json:"SkinMesh"`
	RenderOrder []int            `json:"RenderOrder"`
}

type spriteAnimData struct {
	AnimData []frameData `json:"AnimData"`
}

type frameData struct {
	PosDeg []float64 `json:"PosDeg"`
}

type hierarchyData struct {
	Name   string         `json:"Name"`
	PosRot math32.Vector3 `json:"PosRot"`
	Order  int            `json:"Order"`
	Parent string         `json:"Parent"`
}

func (h hierarchyData) String() string {
	return fmt.Sprintf("Name : %s, PosRot : %v, Order : %d, Parent : %s", h.Name, h.PosRot, h.Order, h.Parent)
}

type spriteSkinData struct {
	Name           string           `json:"Name"`
	Indices        []uint16         `json:"Indices"`
	Vertices       []math32.Vector3 `json:"Vertices"`
	BoneWeights    []boneWeight     `json:"BoneWeights"`
	BindPoses      []math32.Matrix4 `json:"BindPoses"`
	BoneTransforms []string         `json:"BoneTransforms"`
	Uvs            []math32.Vector2 `json:"Uvs"`
}

type boneWeight struct {
	Weight0    float64 `json:"m_Weight0"`
	Weight1    float64 `json:"m_Weight1"`
	Weight2    float64 `json:"m_Weight2"`
	Weight3    float64 `json:"m_Weight3"`
	BoneIndex0 int     `json:"m_BoneIndex0"`
	BoneIndex1 int     `json:"m_BoneIndex1"`
	BoneIndex2 int     `json:"m_BoneIndex2"`
	BoneIndex3 int     `json:"m_BoneIndex3"`
}
