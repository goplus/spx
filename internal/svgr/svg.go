package svg

import (
	"bytes"
	"errors"
	"image"
	"io"
	"io/ioutil"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//const svgHeader = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
//<svg onload="loaded()" xmlns="http://www.w3.org/2000/svg`
//Current placeholder to at least match all SVGs, need actual XML parsing for a real detection
//Also match files starting with an <svg tag
const svgHeader = `<`

func init() {
	image.RegisterFormat("svg", svgHeader, Decode, DecodeConfig)
}

// -------------------------------------------------------------------------------------

// decode Encode SVG to image.Image object.
//
func decode(input []byte, width int, height int) (image.Image, error) {
	in := bytes.NewReader(input)
	icon, err := oksvg.ReadIconStream(in)
	if err != nil {
		return nil, err
	}

	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	if width > 0 && height > 0 {
		w, h = width, height
		icon.SetTarget(0, 0, float64(w), float64(h))
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scannerGV := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scannerGV)
	icon.Draw(raster, 1.0)
	return img, nil
}

// Decode decodes the first frame of an SVG file into an image.
//
func Decode(r io.Reader) (image.Image, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return decode(b, 0, 0)
}

// DecodeSize func.
//
func DecodeSize(r io.Reader, width int, height int) (image.Image, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return decode(b, width, height)
}

// DecodeConfig returns metadata.
//
func DecodeConfig(r io.Reader) (image.Config, error) {
	return image.Config{}, errors.New("not implemented")
}

// -------------------------------------------------------------------------------------
