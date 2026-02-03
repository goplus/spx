package ui

import (
	"math"
	"strings"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/text"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

// Constants for say message layout
const (
	sayMsgSpliteWidth   = 25
	sayMsgLineHeight    = 26
	sayMsgDefaultHeight = 77
	thinkScale          = 0.8
)

// Style constants for say/think bubbles
const (
	StyleSay   = 1
	StyleThink = 2
)

// sayNodes holds the UI nodes for a specific style and direction
type sayNodes struct {
	vbox  *UiNode
	label *UiNode
}

// UiSay represents the UI component for displaying say and think bubbles
type UiSay struct {
	UiNode
	left       sayNodes
	right      sayNodes
	leftThink  sayNodes
	rightThink sayNodes
}

// NewUiSay creates a new UiSay instance
func NewUiSay() *UiSay {
	panel := engine.NewUiNode[UiSay]()
	return panel
}

// OnStart initializes the UI nodes
// Warning: this method is called in main thread
func (s *UiSay) OnStart() {
	s.left = sayNodes{
		vbox:  SyncBindUI[UiNode](s.GetId(), "VL"),
		label: SyncBindUI[UiNode](s.GetId(), "VL/BG/Label"),
	}
	s.right = sayNodes{
		vbox:  SyncBindUI[UiNode](s.GetId(), "VR"),
		label: SyncBindUI[UiNode](s.GetId(), "VR/BG/Label"),
	}
	s.leftThink = sayNodes{
		vbox:  SyncBindUI[UiNode](s.GetId(), "VLThink"),
		label: SyncBindUI[UiNode](s.GetId(), "VLThink/BG/MC/Label"),
	}
	s.rightThink = sayNodes{
		vbox:  SyncBindUI[UiNode](s.GetId(), "VRThink"),
		label: SyncBindUI[UiNode](s.GetId(), "VRThink/BG/MC/Label"),
	}
}

// SetText sets the text content and position for the say/think bubble
func (s *UiSay) SetText(winSize mathf.Vec2, pos mathf.Vec2, size mathf.Vec2, msg string, style int) {
	position := s.calculatePosition(winSize, pos, size, msg)
	isLeft := position.X <= 0
	isThink := style == StyleThink
	scale := s.calculateScale(winSize, isThink)
	nodes := s.selectNodes(isLeft, isThink)
	formattedMsg := s.formatMessage(msg)

	s.updateVisibility(isLeft, isThink)
	s.updateUI(position, scale, nodes.label.GetId(), formattedMsg)
}

// calculateScale computes the uniform scale based on window size
func (s *UiSay) calculateScale(winSize mathf.Vec2, isThink bool) mathf.Vec2 {
	specialScale := 1.0
	if isThink {
		specialScale = thinkScale
	}
	baseSize := mathf.NewVec2(float64(baseScreenWidth), float64(baseScreenHeight))
	scaleVec := winSize.Div(baseSize)

	// Choose the smaller scale to maintain aspect ratio
	uniformScale := math.Min(scaleVec.X, scaleVec.Y)

	zoom := gdx.CameraMgr.GetCameraZoom()
	windowScaleVec := mathf.NewVec2(windowScale, windowScale)

	return zoom.Div(windowScaleVec).Mulf(uniformScale * specialScale)
}

// calculatePosition computes the UI position based on sprite position and size
func (s *UiSay) calculatePosition(winSize mathf.Vec2, pos mathf.Vec2, size mathf.Vec2, msg string) mathf.Vec2 {
	zoom := gdx.CameraMgr.GetCameraZoom()
	camPos := gdx.CameraMgr.GetCameraPosition()
	camPos.Y = -camPos.Y
	localPos := pos.Sub(camPos)
	localPos = localPos.Mul(zoom.Divf(windowScale))

	position := mathf.NewVec2(localPos.X, localPos.Y+size.Y/2)
	if clampUIPositionInScreen {
		position = s.clampPosition(position, winSize, msg)
	}
	return position
}

// clampPosition ensures the UI bubble stays within screen boundaries
func (s *UiSay) clampPosition(position mathf.Vec2, winSize mathf.Vec2, msg string) mathf.Vec2 {
	lineCount := strings.Count(msg, "\n")
	uiHeight := sayMsgDefaultHeight + float64(lineCount)*sayMsgLineHeight

	maxYPos := winSize.Y/2 - uiHeight
	clampedY := math.Max(-winSize.Y/2, math.Min(position.Y, maxYPos))
	clampedX := math.Max(-winSize.X/2, math.Min(position.X, winSize.X/2))

	return mathf.NewVec2(clampedX, clampedY)
}

// selectNodes returns the appropriate UI nodes based on direction and style
func (s *UiSay) selectNodes(isLeft bool, isThink bool) sayNodes {
	switch {
	case isThink && isLeft:
		return s.leftThink
	case isThink && !isLeft:
		return s.rightThink
	case !isThink && isLeft:
		return s.left
	default: // !isThink && !isLeft
		return s.right
	}
}

// formatMessage formats the message with line breaks if needed
func (s *UiSay) formatMessage(msg string) string {
	if strings.ContainsRune(msg, '\n') {
		return msg
	}
	return text.SplitLines(msg, sayMsgSpliteWidth)
}

// updateVisibility sets the visibility of all UI nodes based on style and direction
func (s *UiSay) updateVisibility(isLeft bool, isThink bool) {
	// Direct gdx calls to avoid callInMainThread deadlock when called from main thread (onUpdate)
	gdx.UiMgr.SetVisible(s.left.vbox.GetId(), !isThink && isLeft)
	gdx.UiMgr.SetVisible(s.right.vbox.GetId(), !isThink && !isLeft)
	gdx.UiMgr.SetVisible(s.leftThink.vbox.GetId(), isThink && isLeft)
	gdx.UiMgr.SetVisible(s.rightThink.vbox.GetId(), isThink && !isLeft)
}

// updateUI updates the scale, position, and text of the UI element
func (s *UiSay) updateUI(position mathf.Vec2, scale mathf.Vec2, label engine.Object, text string) {
	gdx.UiMgr.SetScale(s.GetId(), scale)
	gdx.UiMgr.SetPosition(s.GetId(), WorldToUI(position, true))
	gdx.UiMgr.SetText(label, text)
}
