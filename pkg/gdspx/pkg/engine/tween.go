package engine

import (
	. "github.com/realdream-ai/mathf"
)

type posTweenInfo struct {
	value     Vec2
	duration  float64
	startTime float64
}

func (pself *posTweenInfo) getEndTime() float64 {
	return pself.startTime + pself.duration
}

type tweenCallInfo struct {
	id         Object
	startValue Vec2
	callback   func()
	curIndex   int64
	timer      float64
	infos      []posTweenInfo
}

func (pself *tweenCallInfo) getCount() int64 {
	return int64(len(pself.infos))
}
func (pself *tweenCallInfo) isDone() bool {
	return pself.curIndex >= pself.getCount()
}

func (pself *tweenCallInfo) updateStartInfo() {
	if pself.isDone() {
		return
	}
	id := pself.id
	if isNodeExist(id) {
		sprite := GetSprite(id)
		pself.startValue = sprite.GetPosition()
	}
}

func (pself *tweenCallInfo) update() {
	if pself.isDone() {
		return
	}
	curInfo := pself.infos[pself.curIndex]
	percent := Clamp01f((pself.timer - curInfo.startTime) / curInfo.duration)
	id := pself.id
	if isNodeExist(id) {
		sprite := GetSprite(id)
		pos := pself.startValue.Lerpf(curInfo.value, percent)
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
	for i := 0; i < count; i++ {
		tempTweenInfos = append(tempTweenInfos, tweenInfos[i])
	}
	tweenInfos = tweenInfos[:0]
	for i := 0; i < count; i++ {
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
	for i := 0; i < count; i++ {
		if tempTweenInfos[i].isDone() {
			id := tempTweenInfos[i].id
			if isNodeExist(id) {
				tempTweenInfos[i].callback()
			}
		}
	}
	tempTweenInfos = tempTweenInfos[:0]
}

func TweenPos(node ISpriter, pos Vec2, duration float64, callback func()) {
	info := &tweenCallInfo{}
	info.id = node.GetId()
	info.callback = callback
	info.curIndex = 0
	info.timer = 0
	info.infos = []posTweenInfo{{pos, duration, 0}}
	info.updateStartInfo()
	tweenInfos = append(tweenInfos, info)
}

func TweenPos2(node ISpriter, pos Vec2, duration float64, pos2 Vec2, duration2 float64, callback func()) {
	info := &tweenCallInfo{}
	info.id = node.GetId()
	info.callback = callback
	info.curIndex = 0
	info.timer = 0
	info.infos = []posTweenInfo{{pos, duration, 0}, {pos2, duration2, 0}}
	info.updateStartInfo()
	for i := 1; i < len(info.infos); i++ {
		lastInfo := info.infos[i-1]
		info.infos[i].startTime = lastInfo.startTime + lastInfo.duration
	}
	tweenInfos = append(tweenInfos, info)
}
