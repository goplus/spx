package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/goplus/spx/internal/math32"
	"github.com/goplus/spx/internal/skeleton"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenWidth  = 640
	screenHeight = 480
	scale        = 64
	starsCount   = 1024
	vectexScale  = 30.0
)

type Vertex struct {
	X, Y float32
}

func (s *Vertex) Draw(screen *ebiten.Image) {
	c := color.RGBA{
		R: uint8(0xbb * 128 / 0xff),
		G: uint8(0xdd * 128 / 0xff),
		B: uint8(0xff * 128 / 0xff),
		A: 0xff}
	vector.StrokeLine(screen, s.X, s.Y, s.X+3, s.Y+3, 1, c, true)
}

type Game struct {
	prefabData skeleton.SpritePrefabData
	animData   skeleton.SpriteAnimData
	animator   *skeleton.SpriteAnimator
	verteies   [starsCount]Vertex
	bones      [starsCount]Vertex
	actorImg   *ebiten.Image
}

func NewGame() *Game {
	g := &Game{}
	return g
}

func (g *Game) Update() error {
	g.animator.Update()
	g.updateAnimation()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for i := 0; i < starsCount; i++ {
		//g.verteies[i].Draw(screen)
	}
	for i := 0; i < starsCount; i++ {
		g.bones[i].Draw(screen)
	}

	meshes := g.animator.Vertices
	skinMeshes := g.animator.PrefabData.SkinMesh
	renderOrder := g.animator.PrefabData.RenderOrder
	op := &ebiten.DrawTrianglesOptions{}
	op.Address = ebiten.AddressUnsafe
	size := g.actorImg.Bounds().Size()

	for k := 0; k < len(skinMeshes); k++ {
		i := renderOrder[k]
		vs := []ebiten.Vertex{}
		vertices := meshes[i]
		uvs := skinMeshes[i].Uvs
		for j := 0; j < len(vertices); j++ {
			pos := toVector3(vertices[j])
			uv := toUV2(uvs[j], size)
			vs = append(vs, ebiten.Vertex{
				DstX:   pos.X,
				DstY:   pos.Y,
				SrcX:   uv.X,
				SrcY:   uv.Y,
				ColorR: 1,
				ColorG: 1,
				ColorB: 1,
				ColorA: 1,
			})
		}
		indices := skinMeshes[i].Indices
		screen.DrawTriangles(vs, indices, g.actorImg, op)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) CreateCharacter() {
	{
		data, err := ioutil.ReadFile("./assets/sprites/Tom/animation/run.json")
		if err != nil {
			fmt.Println("File reading error", err)
			return
		}

		err = json.Unmarshal(data, &g.animData)
		if err != nil {
			fmt.Println("Error parsing JSON", err)
			return
		}
	}
	{
		data, err := ioutil.ReadFile("./assets/sprites/Tom/prefab.json")
		if err != nil {
			fmt.Println("File reading error", err)
			return
		}

		err = json.Unmarshal(data, &g.prefabData)
		if err != nil {
			fmt.Println("Error parsing JSON", err)
			return
		}
	}

	// Open the image
	{
		path := "assets/sprites/Tom/10.png"
		file, err := os.Open(path)
		if err != nil {
			fmt.Println("Error: File could not be opened ", path)
			os.Exit(1)
		}
		defer file.Close()
		img, _, err := image.Decode(file)
		if err != nil {
			fmt.Println("Error: Image could not be decoded ", path)
			os.Exit(1)
		}
		g.actorImg = ebiten.NewImageFromImage(img)
	}

	g.animator = skeleton.NewSpriteAnimator(&g.prefabData, &g.animData)
	g.updateAnimation()

}
func toVector3(v math32.Vector3) Vertex {
	x := float32(v.X*vectexScale) + screenWidth*0.5
	y := screenHeight - (float32(v.Y*vectexScale) + screenHeight*0.5)
	return Vertex{x, y}
}
func toUV2(v math32.Vector2, size image.Point) Vertex {
	x := float32(v.X) * float32(size.X)
	y := float32(size.Y) - float32(v.Y)*float32(size.Y) // flip y
	return Vertex{x, y}
}

func toVector2(v math32.Vector2) Vertex {
	x := float32(v.X*vectexScale) + screenWidth*0.5
	y := screenHeight - (float32(v.Y*vectexScale) + screenHeight*0.5)
	return Vertex{x, y}

}
func (g *Game) updateAnimation() {
	bones := g.animator.Bones
	for i := 0; i < len(bones); i++ {
		pos := bones[i].Pos
		g.bones[i] = toVector2(pos)
	}
	meshes := g.animator.Vertices
	idx := 0
	for i := 0; i < len(meshes); i++ {
		vertices := meshes[i]
		for j := 0; j < len(vertices); j++ {
			g.verteies[idx] = toVector3(vertices[j])
			idx++
		}
	}
}

func main() {

	game := NewGame()
	game.CreateCharacter()

	rand.Seed(time.Now().UnixNano())
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("(Spx:06-Skeleton)")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
