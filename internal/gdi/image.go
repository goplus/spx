package gdi

import (
	"image"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
)

type Image struct {
	ebiImg *ebiten.Image
	img    *image.RGBA
}

func NewImageSize(width, height int) Image {
	return Image{
		ebiImg: ebiten.NewImage(width, height),
		img:    nil,
	}
}

func NewImageFrom(img image.Image) Image {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		bounds := img.Bounds()
		bounds.Sub(bounds.Min)
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, img.Bounds().Min, draw.Src)
	}
	ebiImg := ebiten.NewImageFromImage(rgba)
	return Image{ebiImg, rgba}
}

func (i Image) Ebiten() *ebiten.Image {
	return i.ebiImg
}

func (i Image) Origin() *image.RGBA {
	return i.img
}

func (i Image) IsValid() bool {
	return i.ebiImg != nil
}

func (i Image) Bounds() image.Rectangle {
	return i.ebiImg.Bounds()
}

func (i Image) Size() (width, height int) {
	return i.ebiImg.Size()
}

func (i Image) SubImage(rect image.Rectangle) Image {
	var sub *ebiten.Image
	var originSub *image.RGBA
	if img := i.ebiImg.SubImage(rect); img != nil {
		sub = img.(*ebiten.Image)
	}
	if img := i.img; img != nil {
		originSub = i.img.SubImage(rect).(*image.RGBA)
	}
	return Image{sub, originSub}
}
