package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	renderTexture *ebiten.Image
	worldWidth    int
	worldHeight   int
	renderScale   float64
)

func GetRenderScale() float64 {
	if renderScale == 0 {
		return 1
	}
	return renderScale
}

func SetRenderInfo(screen *ebiten.Image, width int, height int, scale float64) {
	renderTexture = screen
	worldWidth = width
	worldHeight = height
	renderScale = scale
}

func Draw(screen *ebiten.Image, img *ebiten.Image) {
	renderTexture.DrawImage(img, nil)
}

func DrawWithPos(screen *ebiten.Image, img *ebiten.Image, x float64, y float64) {
	op := new(ebiten.DrawImageOptions)
	op.GeoM.Translate(x, y)
	renderTexture.DrawImage(img, op)
}

func DrawWithPosColor(screen *ebiten.Image, img *ebiten.Image, x float64, y float64, color ebiten.ColorScale) {
	op := new(ebiten.DrawImageOptions)
	op.GeoM.Translate(x, y)
	op.ColorScale = color
	renderTexture.DrawImage(img, op)
}

func GetDrawContextSize(screen *ebiten.Image) (int, int) {
	return worldWidth, worldHeight // TODO return the real renderTexture's size
	//return screen.Bounds().Dx(), screen.Bounds().Dy()
}

func DrawSprite(screen *ebiten.Image, img *ebiten.Image, matrix ebiten.GeoM) {
	op := new(ebiten.DrawImageOptions)
	op.Filter = ebiten.FilterLinear
	op.GeoM = matrix
	renderTexture.DrawImage(img, op)
}
func DrawSpriteRectShader(screen *ebiten.Image, img *ebiten.Image, matrix ebiten.GeoM, shaderFrag []byte, uniforms map[string]any) {
	op := new(ebiten.DrawRectShaderOptions)
	op.GeoM = matrix
	op.Uniforms = uniforms
	shader, err := ebiten.NewShader(shaderFrag)
	if err != nil {
		panic(err)
	}
	op.Images[0] = img
	imgSize := img.Bounds().Size()
	renderTexture.DrawRectShader(imgSize.X, imgSize.Y, shader, op)
}
