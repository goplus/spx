package spx

import (
	"image"
	"math"
	"strconv"

	_ "image/jpeg" // for image decode
	_ "image/png"  // for image decode

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/pkg/errors"

	spxfs "github.com/goplus/spx/fs"
)

func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
}

func normalizeDirection(dir float64) float64 {
	if dir <= -180 {
		dir += 360
	} else if dir > 180 {
		dir -= 360
	}
	return dir
}

type switchAction int

const (
	Prev switchAction = -1
	Next switchAction = 1
)

// -------------------------------------------------------------------------------------

type imagePoint struct {
	x, y float64
}

type imageLoader interface {
	load(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error)
}

type imageLoaderByPath string

func (path imageLoaderByPath) load(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error) {
	f, err := fs.Open(string(path))
	if err != nil {
		return nil, errors.Wrapf(err, "imageLoader: open file `%s` failed", path)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.Wrapf(err, "imageLoader: file `%s` is not an image", path)
	}

	ret := ebiten.NewImageFromImage(img)
	return ret, nil
}

// -------------------------------------------------------------------------------------

type delayloadImage struct {
	cache  *ebiten.Image
	pt     imagePoint
	loader imageLoader
}

func (p *delayloadImage) ensure(fs spxfs.Dir) {
	if p.cache == nil {
		var err error
		if p.cache, err = p.loader.load(fs, &p.pt); err != nil {
			panic(err)
		}
	}
}

type costumeSetImage struct {
	cache  *ebiten.Image
	loader imageLoader
	width  int
	nx     int
}

func (p *costumeSetImage) ensure(fs spxfs.Dir) {
	if p.cache == nil {
		var err error
		if p.cache, err = p.loader.load(fs, nil); err != nil {
			panic(err)
		}
		p.width = p.cache.Bounds().Dx() / p.nx
	}
}

type imageLoaderByCostumeSet struct {
	costumeSet *costumeSetImage
	index      int
}

func (p *imageLoaderByCostumeSet) load(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error) {
	costumeSet := p.costumeSet
	if costumeSet.cache == nil {
		p.costumeSet.ensure(fs)
	}
	cache, width := costumeSet.cache, costumeSet.width
	bounds := cache.Bounds()
	min := image.Point{X: bounds.Min.X + width*p.index, Y: bounds.Min.Y}
	max := image.Point{X: min.X + width, Y: bounds.Max.Y}
	pt.x, pt.y = float64(width>>1), float64(bounds.Dy()>>1)
	if img := cache.SubImage(image.Rectangle{Min: min, Max: max}); img != nil {
		return img.(*ebiten.Image), nil
	}
	panic("disposed image")
}

// -------------------------------------------------------------------------------------

type costume struct {
	name string
	img  delayloadImage

	faceLeft         float64
	bitmapResolution int
}

func newCostumeWith(name string, img *costumeSetImage, faceLeft float64, i, bitmapResolution int) *costume {
	loader := &imageLoaderByCostumeSet{costumeSet: img, index: i}
	return &costume{
		name: name, img: delayloadImage{loader: loader},
		faceLeft: faceLeft, bitmapResolution: bitmapResolution,
	}
}

func newCostume(base string, c *costumeConfig) *costume {
	loader := imageLoaderByPath(base + c.Path)
	return &costume{
		name: c.Name, img: delayloadImage{loader: loader, pt: imagePoint{c.X, c.Y}},
		faceLeft: c.FaceLeft, bitmapResolution: c.BitmapResolution,
	}
}

func (p *costume) needImage(fs spxfs.Dir) (*ebiten.Image, float64, float64) {
	if p.img.cache == nil {
		p.img.ensure(fs)
	}
	return p.img.cache, p.img.pt.x, p.img.pt.y
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes            []*costume
	currentCostumeIndex int
}

func (p *baseObj) initWith(base string, cs *costumeSet, currentCostumeIndex int) {
	nx, faceLeft, bitmapResolution := cs.Nx, cs.FaceLeft, cs.BitmapResolution
	costumeSetLoader := imageLoaderByPath(base + cs.Path)
	img := &costumeSetImage{loader: costumeSetLoader, nx: nx}
	p.costumes = make([]*costume, nx)
	if cs.Items == nil {
		for index := 0; index < nx; index++ {
			p.costumes[index] = newCostumeWith(strconv.Itoa(index), img, faceLeft, index, bitmapResolution)
		}
	} else {
		index := 0
		for _, item := range cs.Items {
			for i := 0; i < item.N; i++ {
				name := item.NamePrefix + strconv.Itoa(i)
				p.costumes[i] = newCostumeWith(name, img, faceLeft, index, bitmapResolution)
				index++
			}
		}
		if index != nx {
			panic("costumeSet load uncompleted")
		}
	}
	if currentCostumeIndex >= nx || currentCostumeIndex < 0 {
		currentCostumeIndex = 0
	}
	p.currentCostumeIndex = currentCostumeIndex
}

func (p *baseObj) init(base string, costumes []*costumeConfig, currentCostumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = newCostume(base, c)
	}
	if currentCostumeIndex >= len(costumes) || currentCostumeIndex < 0 {
		currentCostumeIndex = 0
	}
	p.currentCostumeIndex = currentCostumeIndex
}

func (p *baseObj) initFrom(src *baseObj) {
	p.costumes = src.costumes
	p.currentCostumeIndex = src.currentCostumeIndex
}

func (p *baseObj) findCostume(name string) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (p *baseObj) goSetCostume(val interface{}) bool {
	switch v := val.(type) {
	case string:
		return p.setCostumeByName(v)
	case int:
		return p.setCostumeByIndex(v)
	case switchAction:
		if v == Prev {
			p.goPrevCostume()
		} else {
			p.goNextCostume()
		}
		return true
	case float64:
		return p.setCostumeByIndex(int(v))
	default:
		panic("setCostume: invalid argument type")
	}
}

func (p *baseObj) setCostumeByIndex(idx int) bool {
	if idx >= len(p.costumes) {
		panic("invalid costume index")
	}
	if p.currentCostumeIndex != idx {
		p.currentCostumeIndex = idx
		return true
	}
	return false
}

func (p *baseObj) setCostumeByName(name string) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

func (p *baseObj) goPrevCostume() {
	p.currentCostumeIndex = (len(p.costumes) + p.currentCostumeIndex - 1) % len(p.costumes)
}

func (p *baseObj) goNextCostume() {
	p.currentCostumeIndex = (p.currentCostumeIndex + 1) % len(p.costumes)
}

func (p *baseObj) getCostumeIndex() int {
	return p.currentCostumeIndex
}

func (p *baseObj) getCostumeName() string {
	return p.costumes[p.currentCostumeIndex].name
}

// -------------------------------------------------------------------------------------
