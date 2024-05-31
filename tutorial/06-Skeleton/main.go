package main

import (
	"encoding/json"
	"fmt"
	_ "image/png"
	"io/ioutil"
	"log"

	"github.com/goplus/spx/internal/skeleton"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

type Game struct {
	animator *skeleton.SpriteAnimator
}

func NewGame() *Game {
	g := &Game{}
	return g
}

func (g *Game) Update() error {
	g.animator.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.animator.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) CreateCharacter() {
	data := &skeleton.SpriteAnimatorConfig{}
	jsonData, err := ioutil.ReadFile("./assets/sprites/Tom/animator.json")
	if err != nil {
		fmt.Println("File reading error", err)
		return
	}
	err = json.Unmarshal(jsonData, data)
	if err != nil {
		fmt.Println("File Unmarshal error", err)
		return
	}
	g.animator = skeleton.NewSpriteAnimator(data)
	g.animator.Transform.X = screenWidth / 2
	g.animator.Transform.Y = screenHeight / 2
}

func main() {
	game := NewGame()
	game.CreateCharacter()
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("(Spx:06-Skeleton)")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
