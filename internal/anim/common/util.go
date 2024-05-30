package common

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path"

	spxfs "github.com/goplus/spx/fs"
	"github.com/hajimehoshi/ebiten/v2"
)

func LoadJson(ret interface{}, fs spxfs.Dir, dir string, fileName string) (err error) {
	f, err := fs.Open(path.Join(dir, fileName))
	if err != nil {
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}
func LoadImage(fs spxfs.Dir, dir string, fileName string) (*ebiten.Image, error) {
	file, err := fs.Open(path.Join(dir, fileName))
	if err != nil {
		fmt.Println("Error: File could not be opened ", fileName)
		os.Exit(1)
	}
	defer file.Close()
	data, _, err := image.Decode(file)
	if err != nil {
		fmt.Println("Error: Image could not be decoded ", fileName)
		os.Exit(1)
	}
	img := ebiten.NewImageFromImage(data)
	return img, err
}
