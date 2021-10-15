package spx

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/goplus/spx/internal/gdi"
	"github.com/hajimehoshi/ebiten/v2"
)

// -------------------------------------------------------------------------------------

// stageMonitor class.
type stageMonitor struct {
	target  string
	val     string
	eval    func() string
	mode    int
	color   int
	x, y    float64
	label   string
	visible bool
}

/*
	"type": "stageMonitor",
	"target": "",
	"val": "getVar:score",
	"color": 15629590,
	"label": "score",
	"mode": 1,
	"sliderMin": 0,
	"sliderMax": 100,
	"x": 5,
	"y": 5,
	"isDiscrete": true,
	"visible": true
*/
func newStageMonitor(g reflect.Value, v specsp) (*stageMonitor, error) {
	target := v["target"].(string)
	val := v["val"].(string)
	eval := buildMonitorEval(g, target, val)
	if eval == nil {
		return nil, syscall.ENOENT
	}
	mode := int(v["mode"].(float64))
	color := int(v["color"].(float64))
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)
	return &stageMonitor{
		target: target, val: val, eval: eval,
		visible: visible, mode: mode, color: color, x: x, y: y, label: label,
	}, nil
}

func getTarget(g reflect.Value, target string) (reflect.Value, int) {
	if target == "" {
		return g, 1 // spx.Game
	}
	if val := findFieldPtr(g, target, 0); val != nil {
		if _, ok := val.(Shape); ok {
			return reflect.ValueOf(val).Elem(), 2 // (spx.Sprite, *Game)
		}
	}
	return reflect.Value{}, -1
}

func getValueRef(target reflect.Value, name string, from int) reflect.Value {
	if valPtr := findFieldPtr(target, name, from); valPtr != nil {
		return reflect.ValueOf(valPtr).Elem()
	}
	return reflect.Value{}
}

const (
	getVarPrefix = "getVar:"
)

func buildMonitorEval(g reflect.Value, t, val string) func() string {
	target, from := getTarget(g, t)
	if from < 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(val, getVarPrefix):
		name := val[len(getVarPrefix):]
		ref := getValueRef(target, name, from)
		if ref.IsValid() {
			return func() string {
				return fmt.Sprint(ref.Interface())
			}
		}
		log.Println("[WARN] stageMonitor: var not found -", name, target)
	default:
		log.Println("[WARN] stageMonitor: unknown command -", val)
	}
	return nil
}

func (p *stageMonitor) setVisible(visible bool) {
	p.visible = visible
}

const (
	stmDefaultW      = 47
	stmDefaultSmW    = 41
	stmCornerSize    = 2
	stmVertGapSm     = 4
	stmHoriGapSm     = 5
	stmBackground    = 193 | (196 << 8) | (199 << 16)
	stmBackgroundPen = 0
	stmTextRectPen   = 0xffffff
)

func (p *stageMonitor) draw(dc drawContext) {
	if !p.visible {
		return
	}
	val := p.eval()
	switch p.mode {
	case 2:
		x, y := int(p.x)+2, int(p.y)+6
		render := gdi.NewTextRender(defaultFont, 0x80000, 0)
		render.AddText(val)
		textW, h := render.Size()
		w := textW
		if w < stmDefaultW {
			w = stmDefaultW
		}
		drawRoundRect(dc, x, y, w, h, int(p.color), int(p.color))
		render.Draw(dc.Image, x+((w-textW)>>1), y, color.White, 0)
	default:
		x, y := int(p.x), int(p.y)
		labelRender := gdi.NewTextRender(defaultFont2, 0x80000, 0)
		labelRender.AddText(p.label)
		labelW, h := labelRender.Size()

		textRender := gdi.NewTextRender(defaultFontSm, 0x80000, 0)
		textRender.AddText(val)
		textW, textH := textRender.Size()
		textRectW := textW
		if textRectW < stmDefaultSmW {
			textRectW = stmDefaultSmW
		}
		w := labelW + textRectW + (stmHoriGapSm * 3)
		h += (stmVertGapSm * 2)
		drawRoundRect(dc, x, y, w, h, stmBackground, stmBackgroundPen)
		labelRender.Draw(dc.Image, x+stmHoriGapSm, y+stmVertGapSm, color.Black, 0)
		x += labelW + (stmHoriGapSm * 2)
		y += stmVertGapSm
		drawRoundRect(dc, x, y, textRectW, textH, int(p.color), stmTextRectPen)
		textRender.Draw(dc.Image, x+((textRectW-textW)>>1), y, color.White, 0)
	}
}

type rectKey struct {
	x, y, w, h, clr, clrPen int
}

var (
	rcMap = make(map[rectKey]*ebiten.Image)
)

func drawRoundRect(dc drawContext, x, y, w, h int, clr, clrPen int) {
	key := rectKey{x, y, w, h, clr, clrPen}
	if i, ok := rcMap[key]; ok {
		dc.DrawImage(i, nil)
		return
	}
	img, err := getRoundRect(dc, x, y, w, h, clr, clrPen)
	if err != nil {
		panic(err)
	}
	rcMap[key] = ebiten.NewImageFromImage(img)
}

func getRoundRect(dc drawContext, x, y, w, h int, clr, clrPen int) (image.Image, error) {
	varTable := []string{
		"$x", strconv.Itoa(x),
		"$y2", strconv.Itoa(y + stmCornerSize),
		"$w2", strconv.Itoa(w - stmCornerSize*2),
		"$h2", strconv.Itoa(h - stmCornerSize*2),
	}
	glyphTpl := "M $x $y2 s 0 -2 2 -2 h $w2 s 2 0 2 2 v $h2 s 0 2 -2 2 h -$w2 s -2 0 -2 -2 z"
	glyph := strings.NewReplacer(varTable...).Replace(glyphTpl)

	style := fmt.Sprintf(
		"fill:rgb(%d, %d, %d);stroke-width:0.7;stroke:rgb(%d, %d, %d)",
		(clr>>16)&0xff, (clr>>8)&0xff, clr&0xff,
		(clrPen>>16)&0xff, (clrPen>>8)&0xff, clrPen&0xff)

	cx, cy := dc.Size()
	svg := gdi.NewSVG(cx, cy)
	svg.Path(glyph, style)
	svg.End()
	return svg.ToImage()
}

func (p *stageMonitor) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}

// -------------------------------------------------------------------------------------
