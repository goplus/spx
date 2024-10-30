package timeline

import (
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/goplus/spx/internal/math32"
	"github.com/qiniu/x/log"
)

func Assert(condition bool, args ...string) {
	if !condition {
		for _, arg := range args {
			log.Error("assert fail" + arg)
		}
		panic("assert fail")
	}
}

func AssertApprox(f1 float64, f2 float64, args ...string) {
	equal := math.Abs(float64(f1)-float64(f2)) < 0.0001
	if !equal {
		for _, arg := range args {
			log.Error(arg)
		}
		panic(fmt.Sprintf("\n%f != %f\n", f1, f2))
	}
}

type testSpriteImpl struct {
	angle float64
	x, y  float64
}

func TestTweener1(t *testing.T) {
	fmt.Println("TestTweener1")
	s := &testSpriteImpl{}
	tn := &Tweener{}
	getter := func() interface{} {
		return s.angle
	}
	setter := func(a interface{}) {
		s.angle = a.(float64)
	}

	tn.Init__0(getter, setter, 100.0, 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.angle, 50)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == nil)
	AssertApprox(s.angle, 100)
}

func TestTweener2(t *testing.T) {
	s := &testSpriteImpl{}
	tn := &Tweener{}
	setter := func(a interface{}) {
		s.angle = a.(float64)
	}
	tn.Init__1(setter, 0.0, 100.0, 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.angle, 50)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == nil)
	AssertApprox(s.angle, 100)
}

func TestTweener3(t *testing.T) {
	s := &testSpriteImpl{}
	s.angle = 100.0
	tn := &Tweener{}
	getter := func() interface{} {
		return s.angle
	}
	setter := func(a interface{}) {
		s.angle = a.(float64)
	}
	tn.Init__2(getter, setter, 0.0, 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.angle, 50)
	time = 0.5
	running = tn.Step(&time)
	Assert(running == nil)
	AssertApprox(s.angle, 100)
}

func TestFrom(t *testing.T) {
	s := &testSpriteImpl{}
	s.angle = 0.0
	to := &Tweener{}
	getter := func() interface{} {
		return s.angle
	}
	setter := func(a interface{}) {
		s.angle = a.(float64)
	}
	to.Init__0(getter, setter, 100.0, 1.0)

	from := to.From()
	AssertApprox(s.angle, 0.0)
	var time float64 = 0.0
	running := from.Step(&time)
	Assert(running == from)
	time = 0.5
	running = from.Step(&time)
	Assert(running == from)
	AssertApprox(s.angle, 50.0)
	AssertApprox(float64(time), 0.0)
	time = 0.5
	running = from.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == nil)
	AssertApprox(s.angle, 0.0)
}

func TestSyncMap(t *testing.T) {
	var m sync.Map
	for i := 1; i < 100; i++ {
		s := &testSpriteImpl{}
		s.angle = 0.0
		to := &Tweener{}
		getter := func() interface{} {
			return s.angle
		}
		setter := func(a interface{}) {
			s.angle = a.(float64)
		}
		to.Init__0(getter, setter, 100.0, 1.0)
		m.Store(to.ID, to)
	}
}

func TestTweenerV2(t *testing.T) {
	fmt.Println("TestTweenerV2")
	s := &testSpriteImpl{}
	tn := &Tweener{}
	getter := func() interface{} {
		return *math32.NewVector2(s.x, s.y)
	}
	setter := func(obj interface{}) {
		if v, ok := obj.(math32.Vector2); ok {
			s.x, s.y = v.X, v.Y
		}
	}
	tn.Init__0(getter, setter, *math32.NewVector2(100.0, 100.0), 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.1
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.x, 10.0)
	AssertApprox(s.y, 10.0)
	time = 0.4
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == tn)
	AssertApprox(s.x, 50.0)
	AssertApprox(s.y, 50.0)
	time = 0.5
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == nil)
	AssertApprox(s.x, 100.0)
	AssertApprox(s.y, 100.0)
}

func TestTweenerV2_2(t *testing.T) {
	fmt.Println("TestTweenerV2")
	s := &testSpriteImpl{}
	tn := &Tweener{}
	setter := func(obj interface{}) {
		if v, ok := obj.(math32.Vector2); ok {
			s.x, s.y = v.X, v.Y
		}
	}
	tn.Init__1(setter, *math32.NewVector2(0.0, 0.0), *math32.NewVector2(100.0, 100.0), 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.1
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.x, 10.0)
	AssertApprox(s.y, 10.0)
	time = 0.4
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == tn)
	AssertApprox(s.x, 50.0)
	AssertApprox(s.y, 50.0)
	time = 0.5
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == nil)
	AssertApprox(s.x, 100.0)
	AssertApprox(s.y, 100.0)
}

func TestTweenerV2_3(t *testing.T) {
	fmt.Println("TestTweenerV2")
	s := &testSpriteImpl{}
	s.x = 100.0
	s.y = 100.0
	tn := &Tweener{}
	getter := func() interface{} {
		return *math32.NewVector2(s.x, s.y)
	}
	setter := func(obj interface{}) {
		if v, ok := obj.(math32.Vector2); ok {
			s.x, s.y = v.X, v.Y
		}
	}
	tn.Init__2(getter, setter, *math32.NewVector2(0.0, 0.0), 1.0)
	var time float64 = 0.0
	running := tn.Step(&time)
	Assert(running == tn)
	time = 0.1
	running = tn.Step(&time)
	Assert(running == tn)
	AssertApprox(s.x, 10.0)
	AssertApprox(s.y, 10.0)
	time = 0.4
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == tn)
	AssertApprox(s.x, 50.0)
	AssertApprox(s.y, 50.0)
	time = 0.5
	running = tn.Step(&time)
	AssertApprox(float64(time), 0.0)
	Assert(running == nil)
	AssertApprox(s.x, 100.0)
	AssertApprox(s.y, 100.0)
}

/*
func (p *SpriteImpl) TestTweenScale(targetScale float64, secs float64) {
	fmt.Println("TestTweenScale")
	tn := &timeline.Tweener{}
	tn.Init1(func() interface{} {
		return p.scale
	}, func(obj interface{}) {
		p.scale = obj.(float64)
	}, targetScale, timeline.float64(secs))
	p.g.timelines.AddTimeline(tn)
}

func (p *SpriteImpl) TestTweenV2(x, y float64, secs float64) {
	fmt.Println("TestTweenV2")
	tn := &timeline.Tweener{}
	tn.Init1(func() interface{} {
		return *math32.NewVector2(p.x, p.y)
	}, func(obj interface{}) {
		if v2, ok := obj.(math32.Vector2); ok {
			p.x, p.y = v2.X, v2.Y
		}
	}, *math32.NewVector2(x, y), timeline.float64(secs))
	p.g.timelines.AddTimeline(tn)
}
*/
