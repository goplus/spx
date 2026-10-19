package engine

import (
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func runtimeBridgeTestInstall(t *testing.T, bridge RuntimeBridge) {
	t.Helper()
	previous := runtimeBridge
	SetRuntimeBridge(bridge)
	t.Cleanup(func() { SetRuntimeBridge(previous) })
}

type runtimeBridgeTestConstruction struct {
	operation string
	typ       reflect.Type
	position  mathf.Vec2
	parent    Object
	name      string
	engineUI  bool
}

type runtimeBridgeTestConstructor struct {
	RuntimeBridge
	requests   []runtimeBridgeTestConstruction
	result     reflect.Value
	registered []reflect.Type
}

func (b *runtimeBridgeTestConstructor) construct(request runtimeBridgeTestConstruction) reflect.Value {
	b.requests = append(b.requests, request)
	return b.result
}

func (b *runtimeBridgeTestConstructor) RegisterSpriteType(typ reflect.Type) {
	b.registered = append(b.registered, typ)
}

func (b *runtimeBridgeTestConstructor) CreateSprite(typ reflect.Type, position mathf.Vec2) reflect.Value {
	return b.construct(runtimeBridgeTestConstruction{operation: "sprite", typ: typ, position: position})
}

func (b *runtimeBridgeTestConstructor) CreateEmptySprite(typ reflect.Type, position mathf.Vec2) reflect.Value {
	return b.construct(runtimeBridgeTestConstruction{operation: "empty sprite", typ: typ, position: position})
}

func (b *runtimeBridgeTestConstructor) CreateBackdrop(typ reflect.Type) reflect.Value {
	return b.construct(runtimeBridgeTestConstruction{operation: "backdrop", typ: typ})
}

func (b *runtimeBridgeTestConstructor) CreateUI(typ reflect.Type, name string, engineUI bool) reflect.Value {
	return b.construct(runtimeBridgeTestConstruction{operation: "ui", typ: typ, name: name, engineUI: engineUI})
}

func (b *runtimeBridgeTestConstructor) BindUI(typ reflect.Type, parent Object, path string) reflect.Value {
	return b.construct(runtimeBridgeTestConstruction{operation: "bind ui", typ: typ, parent: parent, name: path})
}

type runtimeBridgeTestObject struct{ value int }

func TestRuntimeBridgeConstructionPreservesTypesAndIdentity(t *testing.T) {
	position := mathf.Vec2{X: 12, Y: -34}
	for _, test := range []struct {
		name    string
		create  func() *runtimeBridgeTestObject
		request runtimeBridgeTestConstruction
	}{
		{"sprite", func() *runtimeBridgeTestObject { return CreateSprite[runtimeBridgeTestObject](position) }, runtimeBridgeTestConstruction{operation: "sprite", position: position}},
		{"empty sprite", func() *runtimeBridgeTestObject { return CreateEmptySprite[runtimeBridgeTestObject](position) }, runtimeBridgeTestConstruction{operation: "empty sprite", position: position}},
		{"backdrop", func() *runtimeBridgeTestObject { return CreateBackdrop[runtimeBridgeTestObject]() }, runtimeBridgeTestConstruction{operation: "backdrop"}},
		{"ui", func() *runtimeBridgeTestObject { return CreateUI[runtimeBridgeTestObject]("HUD") }, runtimeBridgeTestConstruction{operation: "ui", name: "HUD"}},
		{"engine ui", func() *runtimeBridgeTestObject { return CreateEngineUI[runtimeBridgeTestObject]("Debugger") }, runtimeBridgeTestConstruction{operation: "ui", name: "Debugger", engineUI: true}},
		{"bind ui", func() *runtimeBridgeTestObject { return BindUI[runtimeBridgeTestObject](91, "Panel/Label") }, runtimeBridgeTestConstruction{operation: "bind ui", parent: 91, name: "Panel/Label"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			object := &runtimeBridgeTestObject{value: 17}
			bridge := &runtimeBridgeTestConstructor{result: reflect.ValueOf(object).Elem()}
			runtimeBridgeTestInstall(t, bridge)
			got := test.create()
			if got != object {
				t.Fatalf("constructed object = %p, want bridge-owned object %p", got, object)
			}
			got.value = 42
			if bridge.result.FieldByName("value").Int() != 42 {
				t.Fatal("constructed object does not share the runtime's storage")
			}
			test.request.typ = reflect.TypeFor[runtimeBridgeTestObject]()
			if !reflect.DeepEqual(bridge.requests, []runtimeBridgeTestConstruction{test.request}) {
				t.Errorf("construction requests = %#v, want %#v", bridge.requests, test.request)
			}
		})
	}
}

func TestRuntimeBridgeRegistrationAndMissingUI(t *testing.T) {
	bridge := &runtimeBridgeTestConstructor{}
	runtimeBridgeTestInstall(t, bridge)
	RegisterSpriteType[runtimeBridgeTestObject]()
	RegisterSpriteType[*runtimeBridgeTestObject]()
	RegisterSpriteType[ISpriter]()
	want := []reflect.Type{reflect.TypeFor[runtimeBridgeTestObject](), reflect.TypeFor[*runtimeBridgeTestObject](), reflect.TypeFor[ISpriter]()}
	if !reflect.DeepEqual(bridge.registered, want) {
		t.Errorf("registered types = %v, want %v", bridge.registered, want)
	}
	if got := BindUI[runtimeBridgeTestObject](23, "Missing/Label"); got != nil {
		t.Errorf("missing UI = %v, want nil", got)
	}
	wantRequest := runtimeBridgeTestConstruction{operation: "bind ui", typ: reflect.TypeFor[runtimeBridgeTestObject](), parent: 23, name: "Missing/Label"}
	if !reflect.DeepEqual(bridge.requests, []runtimeBridgeTestConstruction{wantRequest}) {
		t.Errorf("missing UI request = %#v, want %#v", bridge.requests, wantRequest)
	}
}

type runtimeBridgeTestAccess struct {
	RuntimeBridge
	sprites   map[Object]ISpriter
	nodes     map[Object]IUiNode
	updates   []float64
	now       float64
	boundID   Object
	boundType string
}

func (b *runtimeBridgeTestAccess) InternalUpdateEngine(delta float64) {
	b.updates = append(b.updates, delta)
}
func (b *runtimeBridgeTestAccess) ClearAllSprites()              { clear(b.sprites) }
func (b *runtimeBridgeTestAccess) GetSprite(id Object) ISpriter  { return b.sprites[id] }
func (b *runtimeBridgeTestAccess) GetUINode(id Object) IUiNode   { return b.nodes[id] }
func (b *runtimeBridgeTestAccess) Sprites() map[Object]ISpriter  { return b.sprites }
func (b *runtimeBridgeTestAccess) UiNodes() map[Object]IUiNode   { return b.nodes }
func (b *runtimeBridgeTestAccess) DeleteSprite(id Object)        { delete(b.sprites, id) }
func (b *runtimeBridgeTestAccess) DeleteUINode(id Object)        { delete(b.nodes, id) }
func (b *runtimeBridgeTestAccess) TimeSinceGameStarted() float64 { return b.now }
func (b *runtimeBridgeTestAccess) AdvanceTimeSinceGameStart(delta float64) float64 {
	b.now += delta
	return b.now
}
func (b *runtimeBridgeTestAccess) BindSceneInstantiatedSprite(id Object, typeName string) {
	b.boundID, b.boundType = id, typeName
}

func TestRuntimeBridgeAccessorsPreserveRuntimeState(t *testing.T) {
	sprite, ui := &Sprite{Id: 7}, &UiNode{Id: 11}
	bridge := &runtimeBridgeTestAccess{
		sprites: map[Object]ISpriter{7: sprite}, nodes: map[Object]IUiNode{11: ui}, now: 12.5,
	}
	runtimeBridgeTestInstall(t, bridge)
	if GetSprite(7) != sprite || GetUINode(11) != ui || GetSprite(99) != nil || GetUINode(99) != nil {
		t.Fatal("runtime lookup lost object identity or missing-object result")
	}
	sprites, nodes := Sprites(), UiNodes()
	sprites[17], nodes[21] = sprite, ui
	if bridge.sprites[17] != sprite || bridge.nodes[21] != ui {
		t.Fatal("accessors returned copies of the runtime registries")
	}
	DeleteSprite(7)
	DeleteUINode(11)
	if len(sprites) != 1 || sprites[17] != sprite || len(nodes) != 1 || nodes[21] != ui {
		t.Fatal("deleting one object changed the wrong registry entry")
	}
	ClearAllSprites()
	if len(sprites) != 0 || len(nodes) != 1 {
		t.Fatal("clearing sprites did not preserve the UI registry")
	}
	InternalUpdateEngine(0.125)
	InternalUpdateEngine(0.25)
	if !reflect.DeepEqual(bridge.updates, []float64{0.125, 0.25}) {
		t.Errorf("engine deltas = %v", bridge.updates)
	}
	if got := AdvanceTimeSinceGameStart(0.75); got != 13.25 || TimeSinceGameStarted() != got {
		t.Errorf("game clock = %v, want 13.25", got)
	}
	BindSceneInstantiatedSprite(31, "Enemy")
	if bridge.boundID != 31 || bridge.boundType != "Enemy" {
		t.Errorf("bound scene sprite = (%d, %q)", bridge.boundID, bridge.boundType)
	}
}

type runtimeBridgeTestScheduledCall struct {
	operation           string
	node                ISpriter
	position, position2 mathf.Vec2
	duration, duration2 float64
}

type runtimeBridgeTestScheduler struct {
	RuntimeBridge
	calls     []runtimeBridgeTestScheduledCall
	callbacks []func()
}

func (b *runtimeBridgeTestScheduler) schedule(call runtimeBridgeTestScheduledCall, callback func()) {
	b.calls = append(b.calls, call)
	b.callbacks = append(b.callbacks, callback)
}
func (b *runtimeBridgeTestScheduler) DelayCall(delay float64, callback func()) {
	b.schedule(runtimeBridgeTestScheduledCall{operation: "delay", duration: delay}, callback)
}
func (b *runtimeBridgeTestScheduler) DelaySpriteCall(delay float64, node ISpriter, callback func()) {
	b.schedule(runtimeBridgeTestScheduledCall{operation: "sprite delay", node: node, duration: delay}, callback)
}
func (b *runtimeBridgeTestScheduler) TweenPos(node ISpriter, position mathf.Vec2, duration float64, callback func()) {
	b.schedule(runtimeBridgeTestScheduledCall{operation: "tween", node: node, position: position, duration: duration}, callback)
}
func (b *runtimeBridgeTestScheduler) TweenPos2(node ISpriter, position mathf.Vec2, duration float64, position2 mathf.Vec2, duration2 float64, callback func()) {
	b.schedule(runtimeBridgeTestScheduledCall{operation: "two-stage tween", node: node, position: position, duration: duration, position2: position2, duration2: duration2}, callback)
}

func TestRuntimeBridgeSchedulingPreservesArgumentsAndCallbacks(t *testing.T) {
	node := &Sprite{Id: 19}
	position, position2 := mathf.Vec2{X: 10, Y: -20}, mathf.Vec2{X: -30, Y: 40}
	for _, test := range []struct {
		schedule func(func())
		want     runtimeBridgeTestScheduledCall
	}{
		{func(callback func()) { DelayCall(0.5, callback) }, runtimeBridgeTestScheduledCall{operation: "delay", duration: 0.5}},
		{func(callback func()) { DelaySpriteCall(1.5, node, callback) }, runtimeBridgeTestScheduledCall{operation: "sprite delay", node: node, duration: 1.5}},
		{func(callback func()) { TweenPos(node, position, 2.5, callback) }, runtimeBridgeTestScheduledCall{operation: "tween", node: node, position: position, duration: 2.5}},
		{func(callback func()) { TweenPos2(node, position, 3.5, position2, 4.5, callback) }, runtimeBridgeTestScheduledCall{operation: "two-stage tween", node: node, position: position, duration: 3.5, position2: position2, duration2: 4.5}},
	} {
		t.Run(test.want.operation, func(t *testing.T) {
			bridge := &runtimeBridgeTestScheduler{}
			runtimeBridgeTestInstall(t, bridge)
			callbacks := 0
			test.schedule(func() { callbacks++ })
			if !reflect.DeepEqual(bridge.calls, []runtimeBridgeTestScheduledCall{test.want}) {
				t.Fatalf("scheduled calls = %#v, want %#v", bridge.calls, test.want)
			}
			if callbacks != 0 || len(bridge.callbacks) != 1 || bridge.callbacks[0] == nil {
				t.Fatal("callback was lost or invoked before the runtime scheduled it")
			}
			bridge.callbacks[0]()
			if callbacks != 1 {
				t.Errorf("callback invocations = %d, want 1", callbacks)
			}
			test.schedule(nil)
			if len(bridge.callbacks) != 2 || bridge.callbacks[1] != nil {
				t.Fatal("nil callback was not preserved")
			}
		})
	}
}

func TestRuntimeBridgeMissingPanicsClearly(t *testing.T) {
	runtimeBridgeTestInstall(t, nil)
	for name, call := range map[string]func(){
		"update":       func() { InternalUpdateEngine(0.1) },
		"construction": func() { CreateSprite[runtimeBridgeTestObject](mathf.Vec2{}) },
		"timer":        func() { DelayCall(1, nil) },
		"tween":        func() { TweenPos(nil, mathf.Vec2{}, 1, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != "engine runtime bridge is not initialized" {
					t.Errorf("panic = %v, want uninitialized bridge message", got)
				}
			}()
			call()
		})
	}
}

type runtimeBridgeTestLifecycleEvent struct {
	name string
	id   Object
}

type runtimeBridgeTestSprite struct {
	Sprite
	events       []runtimeBridgeTestLifecycleEvent
	readyOnStart bool
}

func (s *runtimeBridgeTestSprite) SetId(id Object) {
	s.Sprite.SetId(id)
	s.events = append(s.events, runtimeBridgeTestLifecycleEvent{"set id", s.Id})
}
func (s *runtimeBridgeTestSprite) onCreate() {
	s.Sprite.onCreate()
	s.events = append(s.events, runtimeBridgeTestLifecycleEvent{"create", s.Id})
}
func (s *runtimeBridgeTestSprite) OnStart() {
	s.readyOnStart = s.OnTriggerEnterEvent != nil && s.OnVfxFinishedEvent != nil
	s.events = append(s.events, runtimeBridgeTestLifecycleEvent{"start", s.Id})
}

type runtimeBridgeTestUI struct {
	UiNode
	events       []runtimeBridgeTestLifecycleEvent
	readyOnStart bool
}

func (n *runtimeBridgeTestUI) SetId(id Object) {
	n.UiNode.SetId(id)
	n.events = append(n.events, runtimeBridgeTestLifecycleEvent{"set id", n.Id})
}
func (n *runtimeBridgeTestUI) onCreate() {
	n.UiNode.onCreate()
	n.events = append(n.events, runtimeBridgeTestLifecycleEvent{"create", n.Id})
}
func (n *runtimeBridgeTestUI) OnStart() {
	n.readyOnStart = n.OnUiClickEvent != nil && n.OnUiTextChangedEvent != nil
	n.events = append(n.events, runtimeBridgeTestLifecycleEvent{"start", n.Id})
}

func TestRuntimeBridgeLifecycleOrder(t *testing.T) {
	for _, register := range []bool{false, true} {
		name := "without registration"
		if register {
			name = "with registration"
		}
		t.Run(name, func(t *testing.T) {
			const id Object = 73
			want := []runtimeBridgeTestLifecycleEvent{{"set id", id}}
			if register {
				want = append(want, runtimeBridgeTestLifecycleEvent{"register", id})
			}
			want = append(want, runtimeBridgeTestLifecycleEvent{"create", id}, runtimeBridgeTestLifecycleEvent{"start", id})
			sprite, node := new(runtimeBridgeTestSprite), new(runtimeBridgeTestUI)
			var registerSprite func(Object, ISpriter)
			var registerUI func(Object, IUiNode)
			if register {
				registerSprite = func(gotID Object, got ISpriter) {
					if gotID != id || got != sprite || got.GetId() != id {
						t.Fatal("sprite registration did not receive the initialized instance")
					}
					sprite.events = append(sprite.events, runtimeBridgeTestLifecycleEvent{"register", gotID})
				}
				registerUI = func(gotID Object, got IUiNode) {
					if gotID != id || got != node || got.GetId() != id {
						t.Fatal("UI registration did not receive the initialized instance")
					}
					node.events = append(node.events, runtimeBridgeTestLifecycleEvent{"register", gotID})
				}
			}
			InitSpriteInstance(id, sprite, registerSprite)
			InitUINodeInstance(id, node, registerUI)
			if !sprite.readyOnStart || !node.readyOnStart {
				t.Error("OnStart ran before sprite/UI events were initialized")
			}
			if !reflect.DeepEqual(sprite.events, want) || !reflect.DeepEqual(node.events, want) {
				t.Errorf("lifecycle events: sprite = %v, UI = %v, want %v", sprite.events, node.events, want)
			}
		})
	}
}
