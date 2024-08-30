package vertex

import (
	"log"

	spxfs "github.com/goplus/spx/fs"
	"github.com/goplus/spx/internal/anim/common"
	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type Animator struct {
	common.Animator
	Mesh          *AnimMesh
	curFrameIndex int
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
		clip.FrameCount = clip.Data.FrameCount
		pself.Clips[clipConfig.Name] = clip
	}
	cfg_triangles := pself.Mesh.Triangles
	cfg_vreteices := pself.Mesh.Vertices

	vertexCount := len(cfg_vreteices) / 2
	meshCount := len(cfg_triangles)

	sizePoint := pself.Image.Bounds().Size()
	size := math32.Vector2{X: float64(sizePoint.X), Y: float64(sizePoint.Y)}
	uvOffset := avatarConfig.UvOffset
	// init mesh verteies
	uvs := pself.Mesh.Uvs
	vtxs := make([]ebiten.Vertex, vertexCount)
	for j := 0; j < vertexCount; j++ {
		vtxs[j].ColorR = 1
		vtxs[j].ColorG = 1
		vtxs[j].ColorB = 1
		vtxs[j].ColorA = 1
		vtxs[j].SrcX = float32(uvs[j*2+0]*size.X + uvOffset.X)          // convert uv to render coord
		vtxs[j].SrcY = float32(size.Y - uvs[j*2+1]*size.Y + uvOffset.Y) // flip y
	}
	rdUvs := make([]math32.Vector2, vertexCount)
	for i := 0; i < vertexCount; i++ {
		rdUvs[i] = *math32.NewVector2(uvs[i*2+0], uvs[i*2+1])
	}
	pself.Mesh.RenderUvs = rdUvs

	pself.RenderVerteies = make([][]ebiten.Vertex, meshCount)
	pself.RenderIndeies = make([][]uint16, meshCount)
	for k := 0; k < meshCount; k++ {
		pself.RenderVerteies[k] = vtxs
		pself.RenderIndeies[k] = cfg_triangles[k]
	}

	pself.RederOrder = make([]int, meshCount)
	pself.Play(config.DefaultClip)
	pself.Position = *math32.NewVector2(100, 100)
	return pself
}

func (pself *Animator) Update() {
	if !pself.HasClip(pself.CurClipName) {
		return
	}
	// update bones
	animData := pself.Clips[pself.CurClipName].(*AnimClip).Data
	if animData.FrameCount == 0 {
		return
	}
	meshCount := len(animData.RenderOrders)
	if meshCount == 0 {
		return
	}

	state := pself.GetClipState(pself.CurClipName)
	state.Time += engine.Time.DeltaTime
	frameIndex := state.GetCurFrame()
	pself.curFrameIndex = frameIndex
	pself.UpdateToFrame(frameIndex)

}
func (pself *Animator) UpdateToFrame(frameIndex int) {
	animData := pself.Clips[pself.CurClipName].(*AnimClip).Data
	vertexCount := len(animData.AnimFramesVertex) / animData.FrameCount / 2
	pself.RederOrder = animData.RenderOrders[frameIndex]

	// convert2WorldSpace
	RenderVerteies := pself.RenderVerteies[0]
	offset := frameIndex * vertexCount * 2
	for j := 0; j < vertexCount; j++ {
		x := animData.AnimFramesVertex[offset+j*2+0]
		y := animData.AnimFramesVertex[offset+j*2+1]
		pos := pself.Local2World(*math32.NewVector2(x, y))
		RenderVerteies[j].DstX = float32(pos.X)
		RenderVerteies[j].DstY = float32(pos.Y)
	}
}

func (pself *Animator) GetFrameData() common.AnimExportFrame {
	animData := pself.Clips[pself.CurClipName].(*AnimClip).Data
	vertexCount := len(animData.AnimFramesVertex) / animData.FrameCount / 2
	meshCount := len(pself.RederOrder)
	retVal := common.AnimExportFrame{}
	retVal.Meshes = make([]common.AnimExportMesh, meshCount)
	logicVertices := make([]math32.Vector3, vertexCount)
	offset := pself.curFrameIndex * vertexCount * 2
	for j := 0; j < vertexCount; j++ {
		x := animData.AnimFramesVertex[offset+j*2+0]
		y := animData.AnimFramesVertex[offset+j*2+1]
		logicVertices[j] = *math32.NewVector3(x, y, 0)
	}
	retVal.RederOrder = pself.RederOrder
	if pself.curFrameIndex == 0 {
		for k := 0; k < meshCount; k++ {
			item := common.AnimExportMesh{}
			item.Indices = pself.Mesh.Triangles[k]
			retVal.Meshes[k] = item
		}
		retVal.Meshes[0].Uvs = pself.Mesh.RenderUvs
	} else {
		retVal.Meshes[0] = common.AnimExportMesh{}
	}
	retVal.Meshes[0].Vertices = logicVertices
	return retVal
}
