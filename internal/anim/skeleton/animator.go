package skeleton

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path"

	spxfs "github.com/goplus/spx/fs"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type SpriteAnimator struct {
	SpritePos math32.Vector2
	Prefab    *spritePrefabData
	Image     *ebiten.Image
	// transform
	Scale  math32.Vector2
	Offset math32.Vector2

	Skeleton *Skeleton
	Clips    map[string]*SpriteAnimClip

	// runtime data
	CurClipName   string
	CurFrame      int
	logicBones    []*Bone
	logicVertices [][]math32.Vector3

	// render data
	renderBones    []math32.Vector2
	renderVerteies [][]ebiten.Vertex
	renderIndeies  [][]uint16
	rederOrder     []int
}

func NewSpriteAnimator(baseDir string, fs spxfs.Dir, animatorPath string) *SpriteAnimator {
	var config SpriteAnimatorConfig
	err := loadJson(&config, fs, path.Join(baseDir, animatorPath))
	if err != nil {
		log.Panicf("animator config [%s] not exist", animatorPath)
	}
	pself := &SpriteAnimator{}
	pself.Clips = make(map[string]*SpriteAnimClip)
	pself.CurClipName = ""
	pself.CurFrame = 0
	pself.Scale = config.Scale
	pself.Offset = *config.Offset.Multiply(&config.Scale)
	pself.Prefab = &spritePrefabData{}
	err = loadJson(pself.Prefab, fs, path.Join(baseDir, config.Prefab))
	if err != nil {
		log.Panicf("animator prefab [%s] not exist", path.Join(baseDir, config.Prefab))
	}
	pself.Image, err = loadImage(fs, path.Join(baseDir, config.Image))
	if err != nil {
		log.Panicf("animator image [%s] not exist", path.Join(baseDir, config.Prefab))
	}
	for _, clipConfig := range config.Clips {
		clip := &SpriteAnimClip{}
		clip.Name = clipConfig.Name
		clip.Config = clipConfig
		err = loadJson(&clip.Data, fs, path.Join(baseDir, clipConfig.Path))
		if err != nil {
			log.Panicf("animator clip [%s] not exist", path.Join(baseDir, clipConfig.Path))
		}
		pself.Clips[clipConfig.Name] = clip
	}
	pself.Skeleton = buildSkeleton(pself.Prefab.Hierarchy)
	for i := 0; i < int(lastBone); i++ {
		if bone, ok := pself.Skeleton.Name2Bone[EBoneNames(i).String()]; ok {
			pself.logicBones = append(pself.logicBones, bone)
		}
	}
	pself.logicVertices = make([][]math32.Vector3, len(pself.Prefab.SkinMesh))
	for k, skinData := range pself.Prefab.SkinMesh {
		pself.logicVertices[k] = make([]math32.Vector3, len(skinData.Vertices))
	}

	pself.renderVerteies = make([][]ebiten.Vertex, len(pself.Prefab.SkinMesh))
	pself.renderIndeies = make([][]uint16, len(pself.Prefab.SkinMesh))
	sizePoint := pself.Image.Bounds().Size()
	size := math32.Vector2{X: float64(sizePoint.X), Y: float64(sizePoint.Y)}
	for k, skinData := range pself.Prefab.SkinMesh {
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
		pself.renderVerteies[k] = vtxs
		// bind to render data
		pself.renderIndeies[k] = skinData.Indices
	}
	pself.rederOrder = pself.Prefab.RenderOrder

	pself.renderBones = make([]math32.Vector2, len(pself.logicBones))
	pself.Play(config.DefaultClip)
	pself.SpritePos = *math32.NewVector2(100, 100)
	return pself
}

func (pself *SpriteAnimator) Update() {
	if pself.Clips == nil {
		return
	}
	if _, ok := pself.Clips[pself.CurClipName]; !ok {
		return
	}
	// update bones
	animData := pself.Clips[pself.CurClipName].Data.AnimData
	if len(animData) == 0 {
		return
	}
	pself.CurFrame = (pself.CurFrame) % len(animData)
	frame := animData[pself.CurFrame]
	for i := 0; i < len(pself.logicBones); i++ {
		offset := i * 3
		pos := math32.NewVector2Zero()
		pos.X = frame.PosDeg[offset+0]
		pos.Y = frame.PosDeg[offset+1]
		pself.logicBones[i].LocalPos = *pos
		pself.logicBones[i].LocalDeg = frame.PosDeg[offset+2]
	}
	pself.Skeleton.updateSkeleton(math32.Vector2{}, 0)
	pself.CurFrame++

	// deform
	skinMeshes := pself.Prefab.SkinMesh
	for i, skinData := range skinMeshes {
		deform(&skinData, math32.NewMatrix4Indentity(), pself.Skeleton.Name2Bone, pself.logicVertices[i])
	}
	// convert2WorldSpace
	for i := 0; i < len(pself.logicBones); i++ {
		pos := pself.logicBones[i].Pos
		pself.renderBones[i] = pself.local2World(pos)
	}

	for i := 0; i < len(skinMeshes); i++ {
		vertices := pself.logicVertices[i]
		renderVerteies := pself.renderVerteies[i]
		for j := 0; j < len(vertices); j++ {
			pos := pself.local2World(*math32.NewVector2(vertices[j].X, vertices[j].Y))
			renderVerteies[j].DstX = float32(pos.X)
			renderVerteies[j].DstY = float32(pos.Y)
		}
	}
}
func (pself *SpriteAnimator) Draw(screen *ebiten.Image) {

	//pself.drawBone(screen)
	op := &ebiten.DrawTrianglesOptions{}
	op.Address = ebiten.AddressUnsafe
	for k := 0; k < len(pself.rederOrder); k++ {
		i := pself.rederOrder[k] // render in correct order
		verteies := pself.renderVerteies[i]
		indices := pself.renderIndeies[i]
		screen.DrawTriangles(verteies, indices, pself.Image, op)
	}
}

func (pself *SpriteAnimator) Play(clipName string) {
	if pself.CurClipName == clipName {
		return
	}
	if _, ok := pself.Clips[clipName]; !ok {
		return
	}
	pself.CurClipName = clipName
	pself.CurFrame = 0
}
func (pself *SpriteAnimator) local2World(v math32.Vector2) math32.Vector2 {
	vt := v.Multiply(&pself.Scale).Add(&pself.Offset)
	vt.Y = -vt.Y
	return *vt.Add(&pself.SpritePos)
}

func (pself *SpriteAnimator) drawBone(screen *ebiten.Image) {
	for i := 0; i < len(pself.renderBones); i++ {
		pos := pself.renderBones[i]
		c := color.RGBA{R: uint8(0xbb), G: uint8(0xdd), B: uint8(0xff), A: 0xff}
		vector.StrokeLine(screen, float32(pos.X), float32(pos.Y), float32(pos.X)+3, float32(pos.Y)+3, 1, c, true)
	}
}
func loadJson(ret interface{}, fs spxfs.Dir, file string) (err error) {
	f, err := fs.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}
func loadImage(fs spxfs.Dir, path string) (*ebiten.Image, error) {
	file, err := fs.Open(path)
	if err != nil {
		fmt.Println("Error: File could not be opened ", path)
		os.Exit(1)
	}
	defer file.Close()
	data, _, err := image.Decode(file)
	if err != nil {
		fmt.Println("Error: Image could not be decoded ", path)
		os.Exit(1)
	}
	img := ebiten.NewImageFromImage(data)
	return img, err
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
