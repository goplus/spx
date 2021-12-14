package widget

import (
	img "image"

	"github.com/goplus/spx/internal/ebitenui/image"
	"github.com/goplus/spx/internal/ebitenui/input"

	"github.com/hajimehoshi/ebiten/v2"
)

type Container struct {
	BackgroundImage     *image.NineSlice
	AutoDisableChildren bool

	widgetOpts  []WidgetOpt
	layout      Layouter
	layoutDirty bool

	init     *MultiOnce
	widget   *Widget
	children []PreferredSizeLocateableWidget
}

type ContainerOpt func(c *Container)

type RemoveChildFunc func()

type ContainerOptions struct {
}

var ContainerOpts ContainerOptions

type PreferredSizeLocateableWidget interface {
	HasWidget
	PreferredSizer
	Locateable
}

func NewContainer(opts ...ContainerOpt) *Container {
	c := &Container{
		init: &MultiOnce{},
	}

	c.init.Append(c.createWidget)

	for _, o := range opts {
		o(c)
	}

	return c
}

func (o ContainerOptions) WidgetOpts(opts ...WidgetOpt) ContainerOpt {
	return func(c *Container) {
		c.widgetOpts = append(c.widgetOpts, opts...)
	}
}

func (o ContainerOptions) BackgroundImage(i *image.NineSlice) ContainerOpt {
	return func(c *Container) {
		c.BackgroundImage = i
	}
}

func (o ContainerOptions) AutoDisableChildren() ContainerOpt {
	return func(c *Container) {
		c.AutoDisableChildren = true
	}
}

func (o ContainerOptions) Layout(layout Layouter) ContainerOpt {
	return func(c *Container) {
		c.layout = layout
	}
}

func (c *Container) AddChild(child PreferredSizeLocateableWidget) RemoveChildFunc {
	c.init.Do()

	if child == nil {
		panic("cannot add nil child")
	}

	c.children = append(c.children, child)

	child.GetWidget().parent = c.widget

	c.RequestRelayout()

	return func() {
		c.removeChild(child)
	}
}

func (c *Container) removeChild(child PreferredSizeLocateableWidget) {
	index := -1
	for i, ch := range c.children {
		if ch == child {
			index = i
			break
		}
	}

	if index < 0 {
		return
	}

	c.children = append(c.children[:index], c.children[index+1:]...)

	child.GetWidget().parent = nil

	c.RequestRelayout()
}

func (c *Container) RequestRelayout() {
	c.init.Do()

	c.layoutDirty = true

	for _, ch := range c.children {
		if r, ok := ch.(Relayoutable); ok {
			r.RequestRelayout()
		}
	}
}

func (c *Container) GetWidget() *Widget {
	c.init.Do()
	return c.widget
}

func (c *Container) PreferredSize() (int, int) {
	c.init.Do()

	if c.layout == nil {
		return 50, 50
	}

	return c.layout.PreferredSize(c.children)
}

func (c *Container) SetLocation(rect img.Rectangle) {
	c.init.Do()
	c.widget.Rect = rect
}

func (c *Container) Render(screen *ebiten.Image, def DeferredRenderFunc) {
	c.init.Do()

	if c.AutoDisableChildren {
		for _, ch := range c.children {
			ch.GetWidget().Disabled = c.widget.Disabled
		}
	}

	c.widget.Render(screen, def)

	c.doLayout()

	c.draw(screen)

	for _, ch := range c.children {
		if cr, ok := ch.(Renderer); ok {
			cr.Render(screen, def)
		}
	}
}

func (c *Container) doLayout() {
	if c.layout != nil && c.layoutDirty {
		c.layout.Layout(c.children, c.widget.Rect)
		c.layoutDirty = false
	}
}

func (c *Container) SetupInputLayer(def input.DeferredSetupInputLayerFunc) {
	c.init.Do()

	for _, ch := range c.children {
		if il, ok := ch.(input.Layerer); ok {
			il.SetupInputLayer(def)
		}
	}
}

func (c *Container) draw(screen *ebiten.Image) {
	if c.BackgroundImage != nil {
		c.BackgroundImage.Draw(screen, c.widget.Rect.Dx(), c.widget.Rect.Dy(), c.widget.drawImageOptions)
	}
}

func (c *Container) createWidget() {
	c.widget = NewWidget(c.widgetOpts...)
	c.widgetOpts = nil
}

// WidgetAt implements WidgetLocator.
func (c *Container) WidgetAt(x int, y int) HasWidget {
	c.init.Do()

	p := img.Point{x, y}

	if !p.In(c.GetWidget().Rect) {
		return nil
	}

	for _, ch := range c.children {
		if wl, ok := ch.(Locater); ok {
			w := wl.WidgetAt(x, y)
			if w != nil {
				return w
			}

			continue
		}

		if p.In(ch.GetWidget().Rect) {
			return ch
		}
	}

	return c
}
