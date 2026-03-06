package engine

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type posTweenInfo struct {
	value     Vec2
	duration  float64
	startTime float64
}

func (info *posTweenInfo) getEndTime() float64 {
	return info.startTime + info.duration
}

type tweenCallInfo struct {
	id         Object
	startValue Vec2
	callback   func()
	curIndex   int64
	timer      float64
	infos      []posTweenInfo
}

func (info *tweenCallInfo) getCount() int64 {
	return int64(len(info.infos))
}

func (info *tweenCallInfo) isDone() bool {
	return info.curIndex >= info.getCount()
}

func (info *tweenCallInfo) updateStartInfo() {
	if info.isDone() {
		return
	}
	if sprite := lookupSprite(info.id); sprite != nil {
		info.startValue = sprite.GetPosition()
	}
}

func (info *tweenCallInfo) update() {
	if info.isDone() {
		return
	}
	curInfo := info.infos[info.curIndex]
	percent := Clamp01f((info.timer - curInfo.startTime) / curInfo.duration)
	if sprite := lookupSprite(info.id); sprite != nil {
		pos := info.startValue.Lerpf(curInfo.value, percent)
		sprite.SetPosition(pos)
	}
}

var (
	tweenInfos     = make([]*tweenCallInfo, 0)
	tempTweenInfos = make([]*tweenCallInfo, 0)
)

func updateTweens(delta float64) {
	tempTweenInfos = tempTweenInfos[:0]
	count := len(tweenInfos)
	tempTweenInfos = append(tempTweenInfos, tweenInfos...)
	tweenInfos = tweenInfos[:0]
	for i := range count {
		curTween := tempTweenInfos[i]
		curTween.timer += delta
		for curTween.timer >= curTween.infos[curTween.curIndex].getEndTime() {
			curTween.curIndex++
			curTween.updateStartInfo()
			if curTween.isDone() {
				break
			}
		}
		curTween.update()
		if !curTween.isDone() {
			tweenInfos = append(tweenInfos, curTween)
		}
	}
	for i := range count {
		if tempTweenInfos[i].isDone() && isNodeExist(tempTweenInfos[i].id) {
			tempTweenInfos[i].callback()
		}
	}
	tempTweenInfos = tempTweenInfos[:0]
}

func tweenPos(node gdx.ISpriter, pos Vec2, duration float64, callback func()) {
	info := &tweenCallInfo{
		id:       node.GetId(),
		callback: callback,
		infos:    []posTweenInfo{{value: pos, duration: duration}},
	}
	info.updateStartInfo()
	tweenInfos = append(tweenInfos, info)
}

func tweenPos2(node gdx.ISpriter, pos Vec2, duration float64, pos2 Vec2, duration2 float64, callback func()) {
	info := &tweenCallInfo{
		id:       node.GetId(),
		callback: callback,
		infos: []posTweenInfo{
			{value: pos, duration: duration},
			{value: pos2, duration: duration2},
		},
	}
	info.updateStartInfo()
	for i := 1; i < len(info.infos); i++ {
		lastInfo := info.infos[i-1]
		info.infos[i].startTime = lastInfo.startTime + lastInfo.duration
	}
	tweenInfos = append(tweenInfos, info)
}
