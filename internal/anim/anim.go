package anim

import (
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/goplus/spx/internal/math32"
	"github.com/goplus/spx/internal/tools"
)

const (
	ANIMATIONTYPE_INT     = 0
	ANIMATIONTYPE_FLOAT   = 1
	ANIMATIONTYPE_VECTOR2 = 2

	//Repeat the animation incrementing using key value gradients.
	ANIMATIONLOOPMODE_RELATIVE = 0
	ANIMATIONLOOPMODE_CYCLE    = 1 //Restart the animation from initial value
	ANIMATIONLOOPMODE_CONSTANT = 2 //Pause the animation at the final value
)

type IAnimatable interface {
	GetTarget() IAnimationTarget
	Animate() bool
}

type IAnimation interface {
	GetAnimId() int64
	Animate(target IAnimationTarget, delay float64, from int, to int, loop bool, speedRatio float64) bool
}

type IAnimationTarget interface {
	GetAnimations() []IAnimation
	GetAnimatables() []IAnimatable
}

type Animatable struct {
	target IAnimationTarget

	FromFrame int
	ToFrame   int

	LoopAnimation        bool
	AnimationStartedDate int

	SpeedRatio float64
}

func NewAnimatable(target IAnimationTarget, from int, to int, loop bool, speedRatio float64) *Animatable {

	this := &Animatable{
		SpeedRatio: 1.0,
	}
	this.Init(target, from, to, loop, speedRatio)
	return this
}

func (this *Animatable) Init(target IAnimationTarget, from int, to int, loop bool, speedRatio float64) {

	this.target = target
	this.FromFrame = from
	this.ToFrame = to
	this.LoopAnimation = loop
	this.SpeedRatio = speedRatio

	this.AnimationStartedDate = tools.GetCurrentTimeMs()

}

//interface
/*
type IAnimatable interface {
	GetTarget() IAnimationTarget
	Animate() bool
}
*/
func (this *Animatable) GetTarget() IAnimationTarget {
	return this.target
}

func (this *Animatable) Animate(delay float64) bool {

	// //Getting time
	// var delay float64
	// delay = (float64)(tools.GetCurrentTimeMs() - this.AnimationStartedDate)

	// Animating
	running := false
	animations := this.target.GetAnimations()
	for i := 0; i < len(animations); i++ {
		animation, ok := animations[i].(IAnimation)
		if ok {
			isRunning := animation.Animate(this.target, delay, this.FromFrame, this.ToFrame, this.LoopAnimation, this.SpeedRatio)
			running = running || isRunning
		}

	}

	return running
}

type AnimationKeyFrame struct {
	Frame int
	Value interface{} // Vector2 or Int or Float
}

type Animation struct {
	Name string
	Id   int64

	FramePerSecond float64
	DataType       int
	LoopMode       int

	currentFrame int
	preFrame     int

	//playing
	playingCallback func(*Animation, int, interface{})
	//stop
	stopCallback func(*Animation)
	//error
	errorCallback func(*Animation, error)

	//tween
	easingFunction tools.IEasingFunction

	keys            []*AnimationKeyFrame
	offsetsCache    map[string]interface{}
	highLimitsCache map[string]interface{}
}

var globalAnimId int64 = 1

//loopmodel = -1
func NewAnimation(name string, framePerSecond float64, dataType int, loopMode int) *Animation {
	this := &Animation{}
	this.Init(name, framePerSecond, dataType, loopMode)
	return this
}

func (this *Animation) GetAnimId() int64 {
	return this.Id
}

func (this *Animation) SetEasingFunction(easingFunc tools.IEasingFunction) {
	this.easingFunction = easingFunc
}

func (this *Animation) SetOnPlayingListener(playfuc func(*Animation, int, interface{})) {
	this.playingCallback = playfuc
}

func (this *Animation) SetOnStopingListener(stopfuc func(*Animation)) {
	this.stopCallback = stopfuc
}

func (this *Animation) SetOnErrorListener(errorfuc func(*Animation, error)) {
	this.errorCallback = errorfuc
}

func (this *Animation) Init(name string, framePerSecond float64, dataType int, loopMode int) {
	this.Name = name
	this.Id = globalAnimId
	this.FramePerSecond = framePerSecond
	this.DataType = dataType
	this.currentFrame = math.MaxInt64
	this.preFrame = math.MinInt64

	globalAnimId++

	if loopMode == -1 {
		this.LoopMode = ANIMATIONLOOPMODE_CYCLE
	} else {
		this.LoopMode = loopMode
	}

	this.keys = make([]*AnimationKeyFrame, 0)

	// Cache
	this.offsetsCache = map[string]interface{}{}
	this.highLimitsCache = map[string]interface{}{}
}

func (this *Animation) SetKeys(values []*AnimationKeyFrame) {
	this.keys = values
	this.offsetsCache = map[string]interface{}{}
	this.highLimitsCache = map[string]interface{}{}
}

func (this *Animation) _interpolate(currentFrame int, repeatCount int, loopMode int, offsetValue_Obj interface{}, highLimitValue_Obj interface{}) interface{} {

	if loopMode == ANIMATIONLOOPMODE_CONSTANT && repeatCount > 0 {
		return highLimitValue_Obj
	}
	for key := 0; key < len(this.keys); key++ {
		if this.keys[key+1].Frame >= currentFrame {
			startValue_obj := this.keys[key].Value
			endValue_obj := this.keys[key+1].Value
			gradient := (float64)(currentFrame-this.keys[key].Frame) / (float64)(this.keys[key+1].Frame-this.keys[key].Frame)

			//log.Printf("pregradient %g  currentFrame %d, this.keys[key].Frame %d, this.keys[key+1].Frame %d ", gradient, currentFrame, this.keys[key].Frame, this.keys[key+1].Frame)
			if this.easingFunction != nil {
				gradient = this.easingFunction.Ease(this.easingFunction, gradient)
				//log.Printf("gradient %g ", gradient)
			}

			switch this.DataType {

			case ANIMATIONTYPE_INT:
				startValue, ok := tools.GetInt(startValue_obj)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request int ")
					return 0
				}
				endValue, ok := tools.GetInt(endValue_obj)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request int ")
					return 0.0
				}

				var offsetValue int
				if offsetValue_Obj != nil {
					offsetValue, _ = tools.GetInt(offsetValue_Obj)
				} else {
					offsetValue = 0.0
				}

				//log.Printf("startValue %d, endValue %d, gradient %g , val %d", startValue, endValue, gradient, startValue+int(float64(endValue-startValue)*gradient))

				switch loopMode {
				case ANIMATIONLOOPMODE_CYCLE:
					return startValue + int(float64(endValue-startValue)*gradient)
				case ANIMATIONLOOPMODE_CONSTANT:
					return startValue + int(float64(endValue-startValue)*gradient)
				case ANIMATIONLOOPMODE_RELATIVE:
					return int(float64(offsetValue)*float64(repeatCount) + (float64(startValue) + float64(endValue-startValue)*gradient))
				}

				break
			// Float
			case ANIMATIONTYPE_FLOAT:
				startValue, ok := tools.GetFloat(startValue_obj)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request int ")
					return 0
				}
				endValue, ok := tools.GetFloat(endValue_obj)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request int ")
					return 0.0
				}

				var offsetValue float64
				if offsetValue_Obj != nil {
					offsetValue, _ = tools.GetFloat(offsetValue_Obj)
				} else {
					offsetValue = 0.0
				}

				switch loopMode {
				case ANIMATIONLOOPMODE_CYCLE:
					return startValue + (endValue-startValue)*gradient
				case ANIMATIONLOOPMODE_CONSTANT:
					return startValue + (endValue-startValue)*gradient
				case ANIMATIONLOOPMODE_RELATIVE:
					return offsetValue*float64(repeatCount) + (startValue + (endValue-startValue)*gradient)
				}
				break

			// Vector3
			case ANIMATIONTYPE_VECTOR2:

				startValue, ok := startValue_obj.(*math32.Vector2)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request Quaternion ")
					return math32.NewVector2Zero()
				}
				endValue, ok := endValue_obj.(*math32.Vector2)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request Quaternion ")
					return math32.NewVector2Zero()
				}

				var offsetValue *math32.Vector2
				if offsetValue_Obj != nil {
					offsetValue, _ = offsetValue_Obj.(*math32.Vector2)
				} else {
					offsetValue = math32.NewVector2(0, 0)
				}
				switch loopMode {
				case ANIMATIONLOOPMODE_CYCLE:
					return startValue.Lerp(endValue, gradient)
				case ANIMATIONLOOPMODE_CONSTANT:
					return startValue.Lerp(endValue, gradient)
				case ANIMATIONLOOPMODE_RELATIVE:
					return startValue.Lerp(endValue, gradient).Add(offsetValue.Scale(float64(repeatCount)))
				}
			default:
				break
			}
			break

		}
	}

	return this.keys[len(this.keys)-1].Value
}

//interface
/*
type IAnimation interface {
	Animate(target IAnimationTarget, delay float64, from float64, to float64, loop bool, speedRatio float64) bool
}
*/
func (this *Animation) Animate(target IAnimationTarget, delay float64, from int, to int, loop bool, speedRatio float64) bool {
	if len(this.keys) == 0 {
		if this.errorCallback != nil {
			this.errorCallback(this, errors.New("this keys is empty"))
		}
		return false
	}

	// Check limits
	if from < this.keys[0].Frame || from > this.keys[len(this.keys)-1].Frame {
		from = this.keys[0].Frame
	}
	if to < this.keys[0].Frame || to > this.keys[len(this.keys)-1].Frame {
		to = this.keys[len(this.keys)-1].Frame
	}

	if to == from {
		if this.stopCallback != nil {
			this.stopCallback(this)
		}
		return false
	}

	// Compute ratio
	rangeval := float64(to + 1 - from)
	ratio := delay * float64(this.FramePerSecond*speedRatio) / 1000.0

	if ratio >= rangeval && !loop { // If we are out of range and not looping get back to caller

		//add compete
		if this.playingCallback != nil && this.preFrame != to && len(this.keys) > 1 {
			this.playingCallback(this, to, this.keys[len(this.keys)-1].Value)
		}

		//stop callback
		if this.stopCallback != nil {
			this.stopCallback(this)
		}
		return false
	}
	repeatCount := int(ratio/rangeval) >> 0
	this.currentFrame = from
	if rangeval != 0 {
		this.currentFrame = from + int(ratio)%int(rangeval)
	}
	//\\log.Printf("this.currentFrame %d, val %d, rangeval %g, delay %g, this.FramePerSecond %g, speedRatio %f ratio %g", this.currentFrame, (int(ratio) % int(rangeval)), rangeval, delay, this.FramePerSecond, speedRatio, ratio)

	if this.currentFrame == this.preFrame {
		//anti not stop
		return true
	}
	this.preFrame = this.currentFrame

	var offsetValue interface{}
	var highLimitValue interface{}
	if this.LoopMode != ANIMATIONLOOPMODE_CYCLE {
		keyOffset := fmt.Sprintf("form[%d]to[%d]", from, to)
		_, ok := this.offsetsCache[keyOffset]
		if !ok {

			fromValue_obj := this._interpolate(from, 0, ANIMATIONLOOPMODE_CYCLE, nil, nil)
			toValue_obj := this._interpolate(to, 0, ANIMATIONLOOPMODE_CYCLE, nil, nil)

			switch this.DataType {
			// Float
			case ANIMATIONTYPE_INT:
				toValue, _ := toValue_obj.(int)
				fromValue, _ := fromValue_obj.(int)
				this.offsetsCache[keyOffset] = toValue - fromValue
				break
			// Float
			case ANIMATIONTYPE_FLOAT:
				toValue, _ := toValue_obj.(float64)
				fromValue, _ := fromValue_obj.(float64)
				this.offsetsCache[keyOffset] = toValue - fromValue
				break
			// Vector3
			case ANIMATIONTYPE_VECTOR2:
				toValue, _ := toValue_obj.(*math32.Vector2)
				fromValue, _ := fromValue_obj.(*math32.Vector2)
				this.offsetsCache[keyOffset] = toValue.Sub(fromValue)
			default:
				break
			}

			this.highLimitsCache[keyOffset] = toValue_obj

		}

		highLimitValue, _ = this.highLimitsCache[keyOffset]
		offsetValue, _ = this.offsetsCache[keyOffset]
	}
	// Compute value

	currentValue := this._interpolate(this.currentFrame, repeatCount, this.LoopMode, offsetValue, highLimitValue)

	if this.playingCallback != nil {
		this.playingCallback(this, this.currentFrame, currentValue)
	}

	// // Set value
	// if len(this._targetPropertyPath) > 1 {
	// 	property, err := reflections.GetField(target, this._targetPropertyPath[0])
	// 	if err != nil {
	// 		log.Printf("Animate reflections.GetField %s", property)
	// 		return false
	// 	}

	// 	for index := 1; index < len(this._targetPropertyPath)-1; index++ {
	// 		property, err = reflections.GetField(property, this._targetPropertyPath[index])
	// 		if err != nil {
	// 			log.Printf("Animate reflections.GetField %s", property)
	// 			return false
	// 		}
	// 	}

	// 	valname := this._targetPropertyPath[len(this._targetPropertyPath)-1]
	// 	reflections.SetField(property, valname, currentValue)
	// } else {
	// 	reflections.SetField(target, this._targetProperty, currentValue)
	// }

	return true
}
