package ebitenui

import (
	"image"

	"github.com/goplus/spx/internal/ebitenui/event"
	"github.com/goplus/spx/internal/ebitenui/input"
	internalinput "github.com/goplus/spx/internal/ebitenui/internal/input"
	"github.com/goplus/spx/internal/ebitenui/widget"

	"github.com/hajimehoshi/ebiten/v2"
)

// UI encapsulates a complete user interface that can be rendered onto the screen.
// There should only be exactly one UI per application.
type UI struct {
	// Container is the root container of the UI hierarchy.
	Container *widget.Container

	lastRect      image.Rectangle
	focusedWidget widget.HasWidget
	inputLayerers []input.Layerer
	renderers     []widget.Renderer
}

// RemoveWindowFunc is a function to remove a Window from rendering.
type RemoveWindowFunc func()

// Update updates u. This method should be called in the Ebiten Update function.
func (u *UI) Update() {
	internalinput.Update()
}

// Draw renders u onto screen. This function should be called in the Ebiten Draw function.
//
// If screen's size changes from one frame to the next, u.Container.RequestRelayout is called.
func (u *UI) Draw(screen *ebiten.Image) {
	event.ExecuteDeferred()

	internalinput.Draw()
	defer internalinput.AfterDraw()

	w, h := screen.Size()
	rect := image.Rect(0, 0, w, h)

	defer func() {
		u.lastRect = rect
	}()

	if rect != u.lastRect {
		u.Container.RequestRelayout()
	}

	u.handleFocus()
	u.setupInputLayers()
	u.Container.SetLocation(rect)
	u.render(screen)
}

func (u *UI) handleFocus() {
	if input.MouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if u.focusedWidget != nil {
			u.focusedWidget.(widget.Focuser).Focus(false)
			u.focusedWidget = nil
		}

		x, y := input.CursorPosition()
		w := u.Container.WidgetAt(x, y)
		if w != nil {
			if f, ok := w.(widget.Focuser); ok {
				f.Focus(true)
				u.focusedWidget = w
			}
		}
	}
}

func (u *UI) setupInputLayers() {
	num := 1 // u.Container
	if cap(u.inputLayerers) < num {
		u.inputLayerers = make([]input.Layerer, num)
	}

	u.inputLayerers = u.inputLayerers[:0]
	u.inputLayerers = append(u.inputLayerers, u.Container)

	// TODO: SetupInputLayersWithDeferred should reside in "internal" subpackage
	input.SetupInputLayersWithDeferred(u.inputLayerers)
}

func (u *UI) render(screen *ebiten.Image) {
	num := 1 // u.Container
	if cap(u.renderers) < num {
		u.renderers = make([]widget.Renderer, num)
	}

	u.renderers = u.renderers[:0]
	u.renderers = append(u.renderers, u.Container)

	// TODO: RenderWithDeferred should reside in "internal" subpackage
	widget.RenderWithDeferred(screen, u.renderers)
}
