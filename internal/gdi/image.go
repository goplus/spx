package gdi

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

type SpxImage struct {
	ebiImg *ebiten.Image
	img    *image.RGBA
}

func NewSpxImage(ebiImg *ebiten.Image, img *image.RGBA) *SpxImage {
	spximg := &SpxImage{}
	spximg.ebiImg = ebiImg
	spximg.img = img
	return spximg
}

func (i *SpxImage) EbiImg() *ebiten.Image {
	return i.ebiImg
}
func (i *SpxImage) OriImg() *image.RGBA {
	return i.img
}

func (i *SpxImage) Bounds() image.Rectangle {
	return i.ebiImg.Bounds()
}

func (i *SpxImage) Size() (width, height int) {
	return i.ebiImg.Size()
}

func (i *SpxImage) SubImage(rect image.Rectangle) *SpxImage {
	if sub := i.ebiImg.SubImage(rect); sub != nil {
		originSub := i.img.SubImage(rect)
		spximg := NewSpxImage(sub.(*ebiten.Image), originSub.(*image.RGBA))
		return spximg
	}
	return nil
}
