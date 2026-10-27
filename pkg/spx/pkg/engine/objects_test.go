//go:build !pure_engine

package engine

import (
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
)

type objectTestSpriteMgr struct {
	ISpriteMgr
	ids                       []Object
	position, velocity, scale mathf.Vec2
	physics                   [4]int64
	animationName             string
	animationSpeed            float64
	animationLoop, reverse    bool
	destroyed                 bool
}

type objectTestUiMgr struct {
	IUiMgr
	ids       []Object
	size      mathf.Vec2
	rotation  float64
	destroyed bool
}

func TestSpriteEvents(t *testing.T) {
	first, second := &Sprite{}, &Sprite{}
	first.onCreate()
	second.onCreate()
	for _, tt := range []struct {
		name    string
		first   *Event0
		second  *Event0
		trigger func()
	}{
		{"screen exited", first.OnScreenExitedEvent, second.OnScreenExitedEvent, first.V_OnScreenExited},
		{"screen entered", first.OnScreenEnteredEvent, second.OnScreenEnteredEvent, first.V_OnScreenEntered},
		{"frames set changed", first.OnFramesSetChangedEvent, second.OnFramesSetChangedEvent, first.V_OnFramesSetChanged},
		{"animation changed", first.OnAnimationChangedEvent, second.OnAnimationChangedEvent, first.V_OnAnimationChanged},
		{"frame changed", first.OnFrameChangedEvent, second.OnFrameChangedEvent, first.V_OnFrameChanged},
		{"animation looped", first.OnAnimationLoopedEvent, second.OnAnimationLoopedEvent, first.V_OnAnimationLooped},
		{"animation finished", first.OnAnimationFinishedEvent, second.OnAnimationFinishedEvent, first.V_OnAnimationFinished},
		{"vfx finished", first.OnVfxFinishedEvent, second.OnVfxFinishedEvent, first.V_OnVfxFinished},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertObjectEvent(t, tt.first, tt.second, tt.trigger)
		})
	}
	for _, tt := range []struct {
		name    string
		first   *Event1[ISpriter]
		second  *Event1[ISpriter]
		trigger func(ISpriter)
	}{
		{"trigger entered", first.OnTriggerEnterEvent, second.OnTriggerEnterEvent, first.V_OnTriggerEnter},
		{"trigger exited", first.OnTriggerExitEvent, second.OnTriggerExitEvent, first.V_OnTriggerExit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertObjectEventPayloads(t, tt.first, tt.second, tt.trigger, []ISpriter{second, nil})
		})
	}
}

func TestUiNodeEvents(t *testing.T) {
	first, second := &UiNode{}, &UiNode{}
	first.onCreate()
	second.onCreate()
	for _, tt := range []struct {
		name    string
		first   *Event0
		second  *Event0
		trigger func()
	}{
		{"click", first.OnUiClickEvent, second.OnUiClickEvent, first.V_OnUiClick},
		{"pressed", first.OnUiPressedEvent, second.OnUiPressedEvent, first.V_OnUiPressed},
		{"released", first.OnUiReleasedEvent, second.OnUiReleasedEvent, first.V_OnUiReleased},
		{"hovered", first.OnUiHoveredEvent, second.OnUiHoveredEvent, first.V_OnUiHovered},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertObjectEvent(t, tt.first, tt.second, tt.trigger)
		})
	}
	t.Run("toggle", func(t *testing.T) {
		assertObjectEventPayloads(t, first.OnUiToggleEvent, second.OnUiToggleEvent, first.V_OnUiToggle, []bool{true, false})
	})
	t.Run("text changed", func(t *testing.T) {
		assertObjectEventPayloads(t, first.OnUiTextChangedEvent, second.OnUiTextChangedEvent, first.V_OnUiTextChanged, []string{"你好, player", ""})
	})
}

func TestSpriteAxisHelpers(t *testing.T) {
	for _, name := range []string{"position", "velocity", "scale"} {
		t.Run(name, func(t *testing.T) {
			m := objectTestInstallSpriteMgr(t)
			s := &Sprite{Id: 73}
			var value *mathf.Vec2
			var add func(float64, float64)
			var addX, addY, setX, setY func(float64)
			var getX, getY func() float64
			switch name {
			case "position":
				value, add, addX, addY, setX, setY, getX, getY = &m.position, s.AddPos, s.AddPosX, s.AddPosY, s.SetPosX, s.SetPosY, s.GetPosX, s.GetPosY
			case "velocity":
				value, add, addX, addY, setX, setY, getX, getY = &m.velocity, s.AddVel, s.AddVelX, s.AddVelY, s.SetVelX, s.SetVelY, s.GetVelX, s.GetVelY
			case "scale":
				value, add, addX, addY, setX, setY, getX, getY = &m.scale, s.AddScale, s.AddScaleX, s.AddScaleY, s.SetScaleX, s.SetScaleY, s.GetScaleX, s.GetScaleY
			}
			*value = mathf.Vec2{X: 3, Y: -4}
			for _, step := range []struct {
				name string
				act  func()
				want mathf.Vec2
			}{
				{"add both axes", func() { add(1, 2) }, mathf.Vec2{X: 4, Y: -2}},
				{"add X preserves Y", func() { addX(-5) }, mathf.Vec2{X: -1, Y: -2}},
				{"add Y preserves X", func() { addY(9) }, mathf.Vec2{X: -1, Y: 7}},
				{"set X preserves Y", func() { setX(8) }, mathf.Vec2{X: 8, Y: 7}},
				{"set Y preserves X", func() { setY(-6) }, mathf.Vec2{X: 8, Y: -6}},
			} {
				step.act()
				if *value != step.want || getX() != step.want.X || getY() != step.want.Y {
					t.Fatalf("%s: manager value = %v, getters = (%v, %v), want %v", step.name, *value, getX(), getY(), step.want)
				}
			}
			objectTestAssertIDs(t, m.ids, s.Id)
		})
	}
}

func TestSpriteAnimationPhysicsAndDestroy(t *testing.T) {
	m := objectTestInstallSpriteMgr(t)
	s := &Sprite{}
	s.SetId(92)
	if got := s.GetId(); got != 92 {
		t.Fatalf("GetId() = %d, want 92", got)
	}
	m.animationSpeed, m.animationLoop, m.reverse = 3, true, true
	s.PlayAnimation("walk")
	if m.animationName != "walk" || m.animationSpeed != 1 || m.animationLoop || m.reverse {
		t.Errorf("PlayAnimation defaults = (%q, %v, %v, %v), want (walk, 1, false, false)", m.animationName, m.animationSpeed, m.animationLoop, m.reverse)
	}
	m.physics = [4]int64{1, 2, 4, 8}
	s.DisablePhysic()
	if m.physics != ([4]int64{}) {
		t.Errorf("DisablePhysic left layers/masks enabled: %v", m.physics)
	}
	for _, want := range []bool{true, false} {
		m.destroyed = want
		if got := s.Destroy(); got != want {
			t.Errorf("Destroy() = %v, want %v", got, want)
		}
	}
	objectTestAssertIDs(t, m.ids, 92)
}

func TestUiNodePropertiesAndDestroy(t *testing.T) {
	previous := UiMgr
	m := &objectTestUiMgr{}
	UiMgr = m
	t.Cleanup(func() { UiMgr = previous })
	node := &UiNode{}
	node.SetId(65)
	if got := node.GetId(); got != 65 {
		t.Fatalf("GetId() = %d, want 65", got)
	}
	wantSize := mathf.Vec2{X: 320, Y: 180}
	node.SetSize(wantSize)
	if got := node.GetSize(); got != wantSize || m.size != wantSize {
		t.Errorf("size = %v, manager size = %v, want %v", got, m.size, wantSize)
	}
	node.SetRotation(-45)
	if got := node.GetRotation(); got != -45 || m.rotation != -45 {
		t.Errorf("rotation = %v, manager rotation = %v, want -45", got, m.rotation)
	}
	for _, want := range []bool{true, false} {
		m.destroyed = want
		if got := node.Destroy(); got != want {
			t.Errorf("Destroy() = %v, want %v", got, want)
		}
	}
	objectTestAssertIDs(t, m.ids, 65)
}

func (m *objectTestSpriteMgr) GetPosition(id Object) mathf.Vec2 {
	m.ids = append(m.ids, id)
	return m.position
}

func (m *objectTestSpriteMgr) SetPosition(id Object, value mathf.Vec2) {
	m.ids = append(m.ids, id)
	m.position = value
}

func (m *objectTestSpriteMgr) GetVelocity(id Object) mathf.Vec2 {
	m.ids = append(m.ids, id)
	return m.velocity
}

func (m *objectTestSpriteMgr) SetVelocity(id Object, value mathf.Vec2) {
	m.ids = append(m.ids, id)
	m.velocity = value
}

func (m *objectTestSpriteMgr) GetScale(id Object) mathf.Vec2 {
	m.ids = append(m.ids, id)
	return m.scale
}

func (m *objectTestSpriteMgr) SetScale(id Object, value mathf.Vec2) {
	m.ids = append(m.ids, id)
	m.scale = value
}

func (m *objectTestSpriteMgr) SetTriggerLayer(id Object, value int64) {
	m.ids = append(m.ids, id)
	m.physics[0] = value
}

func (m *objectTestSpriteMgr) SetCollisionLayer(id Object, value int64) {
	m.ids = append(m.ids, id)
	m.physics[1] = value
}

func (m *objectTestSpriteMgr) SetCollisionMask(id Object, value int64) {
	m.ids = append(m.ids, id)
	m.physics[2] = value
}

func (m *objectTestSpriteMgr) SetTriggerMask(id Object, value int64) {
	m.ids = append(m.ids, id)
	m.physics[3] = value
}

func (m *objectTestSpriteMgr) PlayAnim(id Object, name string, speed float64, loop, reverse bool) {
	m.ids = append(m.ids, id)
	m.animationName, m.animationSpeed, m.animationLoop, m.reverse = name, speed, loop, reverse
}

func (m *objectTestSpriteMgr) DestroySprite(id Object) bool {
	m.ids = append(m.ids, id)
	return m.destroyed
}

func (m *objectTestUiMgr) GetSize(id Object) mathf.Vec2 {
	m.ids = append(m.ids, id)
	return m.size
}

func (m *objectTestUiMgr) SetSize(id Object, value mathf.Vec2) {
	m.ids = append(m.ids, id)
	m.size = value
}

func (m *objectTestUiMgr) GetRotation(id Object) float64 {
	m.ids = append(m.ids, id)
	return m.rotation
}

func (m *objectTestUiMgr) SetRotation(id Object, value float64) {
	m.ids = append(m.ids, id)
	m.rotation = value
}

func (m *objectTestUiMgr) DestroyNode(id Object) bool {
	m.ids = append(m.ids, id)
	return m.destroyed
}

func assertObjectEvent(t *testing.T, first, second *Event0, trigger func()) {
	t.Helper()
	if first == nil || second == nil || first == second {
		t.Fatal("onCreate must create an independent event per object")
	}
	var firstCalls, secondCalls int
	first.Subscribe(func() { firstCalls++ })
	second.Subscribe(func() { secondCalls++ })
	trigger()
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("event calls = (%d, %d), want (1, 0)", firstCalls, secondCalls)
	}
}

func assertObjectEventPayloads[T any](t *testing.T, first, second *Event1[T], trigger func(T), want []T) {
	t.Helper()
	if first == nil || second == nil || first == second {
		t.Fatal("onCreate must create an independent event per object")
	}
	var got []T
	first.Subscribe(func(value T) { got = append(got, value) })
	second.Subscribe(func(T) { t.Error("event leaked to another object") })
	for _, value := range want {
		trigger(value)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event payloads = %v, want %v", got, want)
	}
}

func objectTestInstallSpriteMgr(t *testing.T) *objectTestSpriteMgr {
	t.Helper()
	previous := SpriteMgr
	m := &objectTestSpriteMgr{}
	SpriteMgr = m
	t.Cleanup(func() { SpriteMgr = previous })
	return m
}

func objectTestAssertIDs(t *testing.T, ids []Object, want Object) {
	t.Helper()
	if len(ids) == 0 {
		t.Fatal("manager received no calls")
	}
	for _, id := range ids {
		if id != want {
			t.Fatalf("manager received object %d, want %d", id, want)
		}
	}
}
