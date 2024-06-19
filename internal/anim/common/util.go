package common

import (
	"encoding/json"
	"fmt"
	"image"
	"os"

	spxfs "github.com/goplus/spx/fs"
	"github.com/hajimehoshi/ebiten/v2"
)

func LoadJson(ret interface{}, fs spxfs.Dir, file string) (err error) {
	f, err := fs.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}
func LoadImage(fs spxfs.Dir, path string) (*ebiten.Image, error) {
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
