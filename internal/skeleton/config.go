package skeleton

import (
	"fmt"

	"github.com/goplus/spx/internal/math32"
)

type SpriteAnimatorConfig struct {
	Name        string                 `json:"Name"`
	Prefab      string                 `json:"Prefab"`
	Image       string                 `json:"Image"`
	Scale       math32.Vector2         `json:"Scale"`
	Offset      math32.Vector2         `json:"Offset"`
	DefaultClip string                 `json:"DefaultClip"`
	Clips       []SpriteAnimClipConfig `json:"Clips"`
}

type SpriteAnimClipConfig struct {
	Name  string  `json:"Name"`
	Loop  bool    `json:"Loop"`
	Speed float64 `json:"Speed"`
	Path  string  `json:"Path"`
}

type SpriteAnimClip struct {
	Name   string `json:"Name"`
	Config SpriteAnimClipConfig
	Data   spriteAnimData
}

type spriteAnimData struct {
	AnimData []frameData `json:"AnimData"`
}

type frameData struct {
	PosDeg []float64 `json:"PosDeg"`
}

type spritePrefabData struct {
	Name        string           `json:"Name"`
	Hierarchy   []hierarchyData  `json:"Hierarchy"`
	SkinMesh    []spriteSkinData `json:"SkinMesh"`
	RenderOrder []int            `json:"RenderOrder"`
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
