package skeleton

import (
	"fmt"
	"image/color"
	"log"

	spxfs "github.com/goplus/spx/fs"
	"github.com/goplus/spx/internal/anim/common"
	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Animator struct {
	common.Animator
	Mesh       *AnimMesh
	Skeleton   *Skeleton
	LogicBones []*Bone
}

func NewAnimator(fs spxfs.Dir, spriteDir string, config *common.AnimatorConfig, avatarConfig *common.AvatarConfig) *Animator {
	pself := &Animator{}
	pself.Clips = make(map[string]common.IAnimClip)
	pself.CurClipName = ""
	pself.Scale = avatarConfig.Scale
	pself.Offset = *avatarConfig.Offset.Multiply(&avatarConfig.Scale)
	pself.Mesh = &AnimMesh{}
	err := common.LoadJson(pself.Mesh, fs, spriteDir, avatarConfig.Mesh)
	if err != nil {
		log.Panicf("animator Mesh [%s] not exist", avatarConfig.Mesh)
	}
	pself.Image, err = common.LoadImage(fs, spriteDir, avatarConfig.Image)
	if err != nil {
		log.Panicf("animator image [%s] not exist", avatarConfig.Mesh)
	}
	for _, clipConfig := range config.Clips {
		clip := &AnimClip{}
		clip.Name = clipConfig.Name
		clip.Config = clipConfig
		err = common.LoadJson(&clip.Data, fs, spriteDir, clipConfig.Path)
		if err != nil {
			log.Panicf("animator clip [%s] not exist", clipConfig.Path)
		}
		clip.FrameCount = len(clip.Data.AnimData)
		pself.Clips[clipConfig.Name] = clip
	}
	pself.Skeleton = buildSkeleton(pself.Mesh.Hierarchy)
	for i := 0; i < int(lastBone); i++ {
		if bone, ok := pself.Skeleton.Name2Bone[EBoneNames(i).String()]; ok {
			pself.LogicBones = append(pself.LogicBones, bone)
		}
	}
	pself.LogicVertices = make([][]math32.Vector3, len(pself.Mesh.SkinMesh))
	for k, skinData := range pself.Mesh.SkinMesh {
		pself.LogicVertices[k] = make([]math32.Vector3, len(skinData.Vertices))
	}

	pself.RenderVerteies = make([][]ebiten.Vertex, len(pself.Mesh.SkinMesh))
	pself.RenderIndeies = make([][]uint16, len(pself.Mesh.SkinMesh))
	sizePoint := pself.Image.Bounds().Size()
	size := math32.Vector2{X: float64(sizePoint.X), Y: float64(sizePoint.Y)}
	for k, skinData := range pself.Mesh.SkinMesh {
		// create render vertices
		uvs := skinData.Uvs
		vtxs := make([]ebiten.Vertex, len(skinData.Vertices))
		for j := 0; j < len(skinData.Vertices); j++ {
			vtxs[j].ColorR = 1
			vtxs[j].ColorG = 1
			vtxs[j].ColorB = 1
			vtxs[j].ColorA = 1
			vtxs[j].SrcX = float32(uvs[j].X * size.X)        // convert uv to render coord
			vtxs[j].SrcY = float32(size.Y - uvs[j].Y*size.Y) // flip y
		}
		pself.RenderVerteies[k] = vtxs
		// bind to render data
		pself.RenderIndeies[k] = skinData.Indices
	}
	pself.RederOrder = pself.Mesh.RenderOrder

	pself.RenderBones = make([]math32.Vector2, len(pself.LogicBones))
	pself.Play(config.DefaultClip)
	pself.Position = *math32.NewVector2(100, 100)
	return pself
}

func (pself *Animator) Update() {
	if !pself.HasClip(pself.CurClipName) {
		return
	}
	// update bones
	animData := pself.Clips[pself.CurClipName].(*AnimClip).Data.AnimData
	if len(animData) == 0 {
		return
	}
	state := pself.GetClipState(pself.CurClipName)
	state.Time += engine.Time.DeltaTime
	frameIndex := state.GetCurFrame()
	pself.UpdateToFrame(frameIndex)

}
func (pself *Animator) UpdateToFrame(frameIndex int) {
	animData := pself.Clips[pself.CurClipName].(*AnimClip).Data.AnimData
	frame := animData[frameIndex]
	for i := 0; i < len(pself.LogicBones); i++ {
		offset := i * 3
		pos := math32.NewVector2Zero()
		pos.X = frame.PosDeg[offset+0]
		pos.Y = frame.PosDeg[offset+1]
		pself.LogicBones[i].LocalPos = *pos
		pself.LogicBones[i].LocalDeg = frame.PosDeg[offset+2]
	}
	pself.Skeleton.updateSkeleton(math32.Vector2{}, 0)

	// deform
	skinMeshes := pself.Mesh.SkinMesh
	for i, skinData := range skinMeshes {
		deform(&skinData, math32.NewMatrix4Indentity(), pself.Skeleton.Name2Bone, pself.LogicVertices[i])
	}
	// convert2WorldSpace
	for i := 0; i < len(pself.LogicBones); i++ {
		pos := pself.LogicBones[i].Pos
		pself.RenderBones[i] = pself.Local2World(pos)
	}

	for i := 0; i < len(skinMeshes); i++ {
		vertices := pself.LogicVertices[i]
		RenderVerteies := pself.RenderVerteies[i]
		for j := 0; j < len(vertices); j++ {
			pos := pself.Local2World(*math32.NewVector2(vertices[j].X, vertices[j].Y))
			RenderVerteies[j].DstX = float32(pos.X)
			RenderVerteies[j].DstY = float32(pos.Y)
		}
	}
}

func (pself *Animator) GetFrameData() common.AnimExportFrame {
	retVal := common.AnimExportFrame{}
	retVal.Meshes = make([]common.AnimExportMesh, len(pself.Mesh.SkinMesh))
	for k := 0; k < len(pself.RederOrder); k++ {
		i := pself.RederOrder[k]
		item := common.AnimExportMesh{}
		item.Indices = pself.Mesh.SkinMesh[i].Indices
		item.Uvs = pself.Mesh.SkinMesh[i].Uvs
		logicVertices := pself.LogicVertices[i]
		// vertices is different in each frame,so should copy it deeply
		copyData := make([]math32.Vector3, len(logicVertices))
		copy(copyData, logicVertices)
		item.Vertices = copyData
		retVal.Meshes[i] = item
	}
	return retVal
}

func (pself *Animator) drawBone(screen *ebiten.Image) {
	for i := 0; i < len(pself.RenderBones); i++ {
		pos := pself.RenderBones[i]
		c := color.RGBA{R: uint8(0xbb), G: uint8(0xdd), B: uint8(0xff), A: 0xff}
		vector.StrokeLine(screen, float32(pos.X), float32(pos.Y), float32(pos.X)+3, float32(pos.Y)+3, 1, c, true)
	}
}

func deform(skinData *spriteSkinData, rootInv *math32.Matrix4, name2Trans map[string]*Bone, deformed []math32.Vector3) {
	boneTransformsMatrix := make([]*math32.Matrix4, len(name2Trans))
	if len(boneTransformsMatrix) == 0 {
		return
	}
	for i := 0; i < len(boneTransformsMatrix); i++ {
		if i >= len(skinData.BoneTransforms) {
			boneTransformsMatrix[i] = math32.NewMatrix4Indentity()
		} else {
			boneTransformsMatrix[i] = name2Trans[skinData.BoneTransforms[i]].getLocal2WorldMatrix()
		}
	}

	for i := 0; i < len(boneTransformsMatrix); i++ {
		var bindPoseMat *math32.Matrix4
		if i >= len(skinData.BoneTransforms) {
			bindPoseMat = math32.NewMatrix4Indentity()
		} else {
			bindPoseMat = &skinData.BindPoses[i]
		}
		boneTransformMat := boneTransformsMatrix[i]
		boneTransformsMatrix[i] = rootInv.Mul__1(boneTransformMat.Mul__1(bindPoseMat))
	}
	if len(skinData.Vertices) != len(skinData.BoneWeights) {
		panic(fmt.Sprintf("Vertices and BoneWeights must have the same length %d != %d", len(skinData.Vertices), len(skinData.BoneWeights)))
	}

	for i := 0; i < len(skinData.Vertices); i++ {
		bone0 := skinData.BoneWeights[i].BoneIndex0
		bone1 := skinData.BoneWeights[i].BoneIndex1
		bone2 := skinData.BoneWeights[i].BoneIndex2
		bone3 := skinData.BoneWeights[i].BoneIndex3

		vertex := skinData.Vertices[i]
		point := boneTransformsMatrix[bone0].Mul__0(vertex).Scale(skinData.BoneWeights[i].Weight0).
			Add(boneTransformsMatrix[bone1].Mul__0(vertex).Scale(skinData.BoneWeights[i].Weight1)).
			Add(boneTransformsMatrix[bone2].Mul__0(vertex).Scale(skinData.BoneWeights[i].Weight2)).
			Add(boneTransformsMatrix[bone3].Mul__0(vertex).Scale(skinData.BoneWeights[i].Weight3))
		deformed[i] = *point
	}

}
