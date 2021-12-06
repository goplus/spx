package main

import (
	"image/color"
	"log"
	"strconv"

	_ "image/png"

	"github.com/goplus/spx/internal/ebitenui"
	"github.com/goplus/spx/internal/ebitenui/image"
	"github.com/goplus/spx/internal/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/image/font"

	xfont "github.com/goplus/spx/internal/gdi/font"
)

type game struct {
	ui *ebitenui.UI
}

// Layout implements Game.
func (g *game) Layout(outsideWidth int, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// Update implements Game.
func (g *game) Update() error {
	// update the UI
	g.ui.Update()
	return nil
}

// Draw implements Ebiten's Draw method.
func (g *game) Draw(screen *ebiten.Image) {
	// draw the UI onto the screen
	g.ui.Draw(screen)
}

func hexToColor(h string) color.Color {
	u, err := strconv.ParseUint(h, 16, 0)
	if err != nil {
		panic(err)
	}

	return color.RGBA{
		R: uint8(u & 0xff0000 >> 16),
		G: uint8(u & 0xff00 >> 8),
		B: uint8(u & 0xff),
		A: 255,
	}
}

const (
	textIdleColor               = "dff4ff"
	textDisabledColor           = "5a7a91"
	textInputCaretColor         = "e7c34b"
	textInputDisabledCaretColor = "766326"
)

func main() {
	// construct a new container that serves as the root of the UI hierarchy
	rootContainer := widget.NewContainer(
		// the container will use a plain color as its background
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{0x13, 0x1a, 0x22, 0xff})),
	)

	// add the button as a child of the container
	idle, _, err := ebitenutil.NewImageFromFile("res/text-input-idle.png")
	if err != nil {
		log.Panicln(err)
	}
	disabled, _, err := ebitenutil.NewImageFromFile("res/text-input-disabled.png")
	if err != nil {
		log.Panicln(err)
	}
	image := &widget.TextInputImage{
		Idle:     image.NewNineSlice(idle, [3]int{9, 14, 6}, [3]int{9, 14, 6}),
		Disabled: image.NewNineSlice(disabled, [3]int{9, 14, 6}, [3]int{9, 14, 6}),
	}
	color := &widget.TextInputColor{
		Idle:          hexToColor(textIdleColor),
		Disabled:      hexToColor(textDisabledColor),
		Caret:         hexToColor(textInputCaretColor),
		DisabledCaret: hexToColor(textInputDisabledCaretColor),
	}
	const dpi = 72
	defaultFont := xfont.NewDefault(&xfont.Options{
		Size:    15,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	t := widget.NewTextInput(
		widget.TextInputOpts.Padding(widget.Insets{
			Left:   13,
			Right:  13,
			Top:    7,
			Bottom: 7,
		}),
		widget.TextInputOpts.Image(image),
		widget.TextInputOpts.Color(color),
		widget.TextInputOpts.Face(defaultFont),
		widget.TextInputOpts.CaretOpts(
			widget.CaretOpts.Size(defaultFont, 2),
		),
		widget.TextInputOpts.Placeholder("Enter text here"),
	)
	rootContainer.AddChild(t)

	// construct the UI
	ui := ebitenui.UI{
		Container: rootContainer,
	}
	game := game{
		ui: &ui,
	}

	// run Ebiten main loop
	err = ebiten.RunGame(&game)
	if err != nil {
		log.Println(err)
	}
}
