package ui

import (
	"math"
	"strings"

	. "github.com/realdream-ai/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/text"
)

const (
	sayMsgSpliteWidth   = 25
	sayMsgLineHeight    = 26
	sayMsgDefaultHeight = 77
)

type UiSay struct {
	UiNode
	vboxL  *UiNode
	labelL *UiNode
	vboxR  *UiNode
	labelR *UiNode
}

func NewUiSay() *UiSay {
	panel := engine.SyncCreateEngineUiNode[UiSay]("")
	return panel
}

func (pself *UiSay) OnStart() {
	pself.vboxL = BindUI[UiNode](pself.GetId(), "VL")
	pself.labelL = BindUI[UiNode](pself.GetId(), "VL/BG/Label")
	pself.vboxR = BindUI[UiNode](pself.GetId(), "VR")
	pself.labelR = BindUI[UiNode](pself.GetId(), "VR/BG/Label")
}

func (pself *UiSay) SetText(winX, winY float64, x, y float64, w, h float64, msg string) {
	x, y = engine.SyncGetCameraLocalPosition(x, y)
	isLeft := x <= 0
	xPos := x
	yPos := y + h/2
	engine.SyncUiSetVisible(pself.vboxL.GetId(), isLeft)
	engine.SyncUiSetVisible(pself.vboxR.GetId(), !isLeft)
	label := pself.labelL.GetId()
	if !isLeft {
		label = pself.labelR.GetId()
	}
	hasNextLine := strings.ContainsRune(msg, '\n')
	finalMsg := msg
	if !hasNextLine {
		finalMsg = text.SplitLines(msg, sayMsgSpliteWidth)
	}
	lineCount := strings.Count(finalMsg, "\n")
	uiHeight := sayMsgDefaultHeight + float64(lineCount)*sayMsgLineHeight
	maxYPos := winY/2 - uiHeight
	yPos = math.Max(-winY/2, math.Min(yPos, maxYPos))
	xPos = math.Max(-winX/2, math.Min(x, winX/2))

	engine.SyncUiSetPosition(pself.GetId(), WorldToScreen(xPos, yPos))
	engine.SyncUiSetText(label, finalMsg)
}
