package anim

import (
	"log"

	"github.com/goplus/spx/internal/math32"
	"github.com/goplus/spx/internal/tools"
)

type AnimationKeyFrame struct {
	Frame int
	Value interface{} // Vector2 or Int or Float
}

type AnimChannel struct {
	Name     string
	DataType int
	//tween
	easingFunction tools.IEasingFunction
	keys           []*AnimationKeyFrame
}

func (this *AnimChannel) SetEasingFunction(easingFunc tools.IEasingFunction) {
	this.easingFunction = easingFunc
}

// loopmodel = -1
func NewAnimChannel(name string, dataType int, easingFunc tools.IEasingFunction, keys []*AnimationKeyFrame) *AnimChannel {
	this := &AnimChannel{}
	this.Name = name
	this.DataType = dataType
	this.easingFunction = easingFunc

	this.keys = keys
	return this
}

func (this *AnimChannel) interpolate(currentFrame int) interface{} {
	// only one key
	if len(this.keys) == 1 {
		return this.keys[0].Value
	}

	// frame less than first key
	if this.keys[0].Frame >= currentFrame {
		return this.keys[0].Value
	}

	// frame between two keys
	for key := 0; key < len(this.keys)-1; key++ {
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

				return startValue + int(float64(endValue-startValue)*gradient)

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

				return startValue + (endValue-startValue)*gradient

			// Vector2
			case ANIMATIONTYPE_VECTOR2:

				startValue, ok := startValue_obj.(*math32.Vector2)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request Vector2 ")
					return math32.NewVector2Zero()
				}
				endValue, ok := endValue_obj.(*math32.Vector2)
				if !ok {
					log.Printf("_interpolate The interface type is incorrect, request Vector2 ")
					return math32.NewVector2Zero()
				}
				return startValue.Lerp(endValue, gradient)
			default:
				break
			}
			break

		}
	}
	// >= last key frame
	return this.keys[len(this.keys)-1].Value
}
