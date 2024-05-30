package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log"
	"math/rand"
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

var (
	whiteImage = ebiten.NewImage(3, 3)
)

func init() {
	whiteImage.Fill(color.White)
}

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
	op := &ebiten.DrawTrianglesOptions{}
	op.Address = ebiten.AddressUnsafe
	for i := 0; i < len(meshes); i++ {
		vs := []ebiten.Vertex{}
		vertices := meshes[i]
		for j := 0; j < len(vertices); j++ {
			pos := toVector3(vertices[j])
			vs = append(vs, ebiten.Vertex{
				DstX:   pos.X,
				DstY:   pos.Y,
				SrcX:   0,
				SrcY:   0,
				ColorR: 1,
				ColorG: 1,
				ColorB: 1,
				ColorA: 1,
			})
		}
		indices := skinMeshes[i].Indices
		screen.DrawTriangles(vs, indices, whiteImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image), op)
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
	g.animator = skeleton.NewSpriteAnimator(&g.prefabData, &g.animData)
	g.updateAnimation()

}
func toVector3(v math32.Vector3) Vertex {
	x := float32(v.X*vectexScale) + screenWidth*0.5
	y := screenHeight - (float32(v.Y*vectexScale) + screenHeight*0.5)
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
