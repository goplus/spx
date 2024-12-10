package ui

import (
	"math"
	"strings"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/text"
	"github.com/realdream-ai/mathf"
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
	panel := engine.NewUiNode[UiSay]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiSay) OnStart() {
	pself.vboxL = SyncBindUI[UiNode](pself.GetId(), "VL")
	pself.labelL = SyncBindUI[UiNode](pself.GetId(), "VL/BG/Label")
	pself.vboxR = SyncBindUI[UiNode](pself.GetId(), "VR")
	pself.labelR = SyncBindUI[UiNode](pself.GetId(), "VR/BG/Label")
}

func (pself *UiSay) SetText(winSize mathf.Vec2, pos mathf.Vec2, size mathf.Vec2, msg string) {
	camPos := cameraMgr.GetLocalPosition(pos)
	x, y := camPos.X, camPos.Y
	isLeft := x <= 0
	xPos := x
	yPos := y + size.Y/2
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
	maxYPos := winSize.Y/2 - uiHeight
	yPos = math.Max(-winSize.Y/2, math.Min(yPos, maxYPos))
	xPos = math.Max(-winSize.X/2, math.Min(x, winSize.X/2))

	uiMgr.SetVisible(pself.vboxL.GetId(), isLeft)
	uiMgr.SetVisible(pself.vboxR.GetId(), !isLeft)
	uiMgr.SetPosition(pself.GetId(), WorldToUI(mathf.NewVec2(xPos, yPos)))
	uiMgr.SetText(label, finalMsg)
}
