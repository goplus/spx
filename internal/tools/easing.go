//by sqr

package tools

import "math"

type IEasingCoreFunction interface {
	EaseInCore(gradient float64) float64
}

type IEasingFunction interface {
	IEasingCoreFunction
	/**
	 * Given an input gradient between 0 and 1, this returns the corresponding value
	 * of the easing function.
	 */
	Ease(corefunc IEasingCoreFunction, gradient float64) float64
}

type EasingMode uint8

const (
	/**
	 * Interpolation follows the mathematical formula associated with the easing function.
	 */
	EASINGMODE_EASEIN EasingMode = iota

	/**
	 * Interpolation follows 100% interpolation minus the output of the formula associated with the easing function.
	 */
	EASINGMODE_EASEOUT

	/**
	 * Interpolation uses EaseIn for the first half of the animation and EaseOut for the second half.
	 */
	EASINGMODE_EASEINOUT
)

type EasingFunction struct {
	easingMode EasingMode
}

/**
* Sets the easing mode of the current function.
 */
func (this *EasingFunction) setEasingMode(easingMode EasingMode) {
	this.easingMode = easingMode
}

/**
* Gets the current easing mode.
 */
func (this *EasingFunction) getEasingMode() EasingMode {
	return this.easingMode
}

func (this *EasingFunction) EaseInCore(gradient float64) float64 {
	panic("You must implement this method")
}

func (this *EasingFunction) Ease(core IEasingCoreFunction, gradient float64) float64 {
	switch this.easingMode {
	case EASINGMODE_EASEIN:
		return core.EaseInCore(gradient)
	case EASINGMODE_EASEOUT:
		return (1 - core.EaseInCore(1-gradient))
	}

	if gradient >= 0.5 {
		return (((1 - core.EaseInCore((1-gradient)*2)) * 0.5) + 0.5)
	}

	return (core.EaseInCore(gradient*2) * 0.5)
}

//------------

/**
 * Easing function with a circle shape (see link below).
 * @see https://easings.net/#easeInCirc
 */
type CircleEase struct {
	EasingFunction
}

func NewCircleEase() *CircleEase {
	return &CircleEase{}
}

func (this *CircleEase) EaseInCore(gradient float64) float64 {
	gradient = math.Max(0, math.Min(1, gradient))
	return (1.0 - math.Sqrt(1.0-(gradient*gradient)))
}

/**
 * Easing function with a ease back shape (see link below).
 * @see https://easings.net/#easeInBack
 */
type BackEase struct {
	EasingFunction

	Amplitude float64
}

func NewBackEase() *BackEase {
	return &BackEase{
		Amplitude: 1.0,
	}
}

/** @hidden */
func (this *BackEase) EaseInCore(gradient float64) float64 {
	var num = math.Max(0, this.Amplitude)
	return (math.Pow(gradient, 3.0) - ((gradient * num) * math.Sin(3.1415926535897931*gradient)))
}

/**
 * Easing function with a bouncing shape (see link below).
 * @see https://easings.net/#easeInBounce
 */

type BounceEase struct {
	EasingFunction

	/** Defines the number of bounces */
	Bounces float64

	/** Defines the amplitude of the bounce*/
	Bounciness float64
}

func NewBounceEase() *BounceEase {
	return &BounceEase{
		Bounces:    3.0,
		Bounciness: 2.0,
	}
}
func (this *BounceEase) EaseInCore(gradient float64) float64 {
	var y = math.Max(0.0, this.Bounces)
	var bounciness = this.Bounciness
	if bounciness <= 1.0 {
		bounciness = 1.001
	}
	var num9 = math.Pow(bounciness, y)
	var num5 = 1.0 - bounciness
	var num4 = ((1.0 - num9) / num5) + (num9 * 0.5)
	var num15 = gradient * num4
	var num65 = math.Log((-num15*(1.0-bounciness))+1.0) / math.Log(bounciness)
	var num3 = math.Floor(num65)
	var num13 = num3 + 1.0
	var num8 = (1.0 - math.Pow(bounciness, num3)) / (num5 * num4)
	var num12 = (1.0 - math.Pow(bounciness, num13)) / (num5 * num4)
	var num7 = (num8 + num12) * 0.5
	var num6 = gradient - num7
	var num2 = num7 - num8
	return (((-math.Pow(1.0/bounciness, y-num3) / (num2 * num2)) * (num6 - num2)) * (num6 + num2))
}
