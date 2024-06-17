package engine

import "github.com/hajimehoshi/ebiten/v2"

func Draw(screen *ebiten.Image, img *ebiten.Image) {
	screen.DrawImage(img, nil)
}

func DrawWithPos(screen *ebiten.Image, img *ebiten.Image, x float64, y float64) {
	op := new(ebiten.DrawImageOptions)
	op.GeoM.Translate(x, y)
	screen.DrawImage(img, op)
}

func DrawWithPosColor(screen *ebiten.Image, img *ebiten.Image, x float64, y float64, color ebiten.ColorScale) {
	options := new(ebiten.DrawImageOptions)
	options.GeoM.Translate(float64(x), float64(y))
	options.ColorScale = color
	screen.DrawImage(img, options)
}

func GetDrawContextSize(screen *ebiten.Image) (int, int) {
	return screen.Bounds().Dx(), screen.Bounds().Dy()
}
