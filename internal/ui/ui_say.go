package ui

import (
	"math"
	"strings"

	. "github.com/realdream-ai/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/gdi"
)

const (
	SayMsgSpliteWidth   = 25
	SayMsgLineHeight    = 26
	SayMsgDefaultHeight = 77
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
		finalMsg = gdi.SplitString(msg, SayMsgSpliteWidth)
	}
	lineCount := strings.Count(finalMsg, "\n")
	uiHeight := SayMsgDefaultHeight + float64(lineCount)*SayMsgLineHeight
	maxYPos := winY/2 - uiHeight
	yPos = math.Max(-winY/2, math.Min(yPos, maxYPos))
	xPos = math.Max(-winX/2, math.Min(x, winX/2))

	engine.SyncUiSetPosition(pself.GetId(), WorldToScreen(xPos, yPos))
	engine.SyncUiSetText(label, finalMsg)
}
