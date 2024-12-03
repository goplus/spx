package spx

type Widget interface {
	GetName() WidgetName
	Visible() bool
	Show()
	Hide()

	Xpos() float64
	Ypos() float64
	SetXpos(x float64)
	SetYpos(y float64)
	SetXYpos(x float64, y float64)
	ChangeXpos(dx float64)
	ChangeYpos(dy float64)
	ChangeXYpos(dx float64, dy float64)

	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)
}

type WidgetName = string
