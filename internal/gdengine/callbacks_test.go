package gdengine

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/gdengine/binding/facade"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type callbackTestRuntime struct {
	engine.RuntimeBridge
	sprites map[engine.Object]engine.ISpriter
	calls   *[]string
}

func (r *callbackTestRuntime) Sprites() map[engine.Object]engine.ISpriter {
	return r.sprites
}

func (r *callbackTestRuntime) AdvanceTimeSinceGameStart(delta float64) float64 {
	*r.calls = append(*r.calls, fmt.Sprintf("advance:%g", delta))
	return delta
}

func (r *callbackTestRuntime) InternalUpdateEngine(delta float64) {
	*r.calls = append(*r.calls, fmt.Sprintf("runtime:%g", delta))
}

type callbackTestSprite struct {
	engine.ISpriter
	calls *[]string
}

func (s *callbackTestSprite) OnUpdate(delta float64) {
	*s.calls = append(*s.calls, fmt.Sprintf("sprite-update:%g", delta))
}

func (s *callbackTestSprite) OnFixedUpdate(delta float64) {
	*s.calls = append(*s.calls, fmt.Sprintf("sprite-fixed:%g", delta))
}

func (s *callbackTestSprite) OnDestroy() {
	*s.calls = append(*s.calls, "sprite-destroy")
}

func TestEngineFrameCallbackOwnsRuntimeUpdates(t *testing.T) {
	previousCallbacks, previousSprites := coreCallbacks, sprites
	previousDelta, _ := itime.FixedDeltaTime()
	t.Cleanup(func() {
		coreCallbacks, sprites = previousCallbacks, previousSprites
		itime.SetFixedDeltaTime(previousDelta)
		engine.SetRuntimeBridge(nil)
	})

	for _, fixed := range []float64{0, 0.1} {
		for _, runUpdate := range []bool{false, true} {
			t.Run(fmt.Sprintf("fixed=%g/update=%v", fixed, runUpdate), func(t *testing.T) {
				var calls []string
				sprite := &callbackTestSprite{calls: &calls}
				engine.SetRuntimeBridge(&callbackTestRuntime{
					sprites: map[engine.Object]engine.ISpriter{1: sprite}, calls: &calls,
				})
				itime.SetFixedDeltaTime(fixed)
				coreCallbacks = engine.CoreCallbackInfo{
					OnEngineUpdate: func(delta float64) {
						calls = append(calls, fmt.Sprintf("frame-begin:%g", delta))
						if runUpdate {
							engine.InternalUpdateEngine(delta)
						}
						calls = append(calls, "frame-end")
					},
				}
				bindCallbacks().OnEngineUpdate(0.25)

				logical := 0.25
				if fixed > 0 {
					logical = fixed
				}
				want := []string{fmt.Sprintf("advance:%g", logical), fmt.Sprintf("frame-begin:%g", logical)}
				if runUpdate {
					want = append(want, fmt.Sprintf("runtime:%g", logical))
				}
				want = append(want, "frame-end")
				if !slices.Equal(calls, want) {
					t.Fatalf("frame calls = %v, want %v", calls, want)
				}
			})
		}
	}
}

func TestEngineCallbacksPreserveRuntimeLifecycle(t *testing.T) {
	previousCallbacks, previousSprites := coreCallbacks, sprites
	previousLink, previousInterpreter := activeLink, isWebIntepreterMode
	previousAudio, previousCamera, previousDebug := engine.AudioMgr, engine.CameraMgr, engine.DebugMgr
	previousExt, previousInput, previousNavigation := engine.ExtMgr, engine.InputMgr, engine.NavigationMgr
	previousPen, previousPhysics, previousPlatform := engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr
	previousRes, previousScene, previousSprite := engine.ResMgr, engine.SceneMgr, engine.SpriteMgr
	previousTilemap, previousTilemapparser, previousUI := engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr
	previousDelta, _ := itime.FixedDeltaTime()
	t.Cleanup(func() {
		coreCallbacks, sprites = previousCallbacks, previousSprites
		activeLink, isWebIntepreterMode = previousLink, previousInterpreter
		engine.AudioMgr, engine.CameraMgr, engine.DebugMgr = previousAudio, previousCamera, previousDebug
		engine.ExtMgr, engine.InputMgr, engine.NavigationMgr = previousExt, previousInput, previousNavigation
		engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr = previousPen, previousPhysics, previousPlatform
		engine.ResMgr, engine.SceneMgr, engine.SpriteMgr = previousRes, previousScene, previousSprite
		engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr = previousTilemap, previousTilemapparser, previousUI
		if runtime.GOOS != "js" {
			facade.RegisterCallbacks(bindCallbacks())
		}
		itime.SetFixedDeltaTime(previousDelta)
		engine.SetRuntimeBridge(nil)
	})
	if activeLink != nil {
		t.Fatal("lifecycle fixture requires no active link")
	}

	for _, fixed := range []float64{0, 0.1} {
		for _, withCore := range []bool{false, true} {
			t.Run(fmt.Sprintf("fixed=%g/core=%v", fixed, withCore), func(t *testing.T) {
				var calls, want []string
				sprite := &callbackTestSprite{calls: &calls}
				engine.SetRuntimeBridge(&callbackTestRuntime{
					sprites: map[engine.Object]engine.ISpriter{1: sprite}, calls: &calls,
				})
				itime.SetFixedDeltaTime(fixed)
				core := engine.CoreCallbackInfo{}
				if withCore {
					core = engine.CoreCallbackInfo{
						OnEngineStart:       func() { calls = append(calls, "core-start") },
						OnEngineUpdate:      func(delta float64) { calls = append(calls, fmt.Sprintf("core-update:%g", delta)) },
						OnEngineFixedUpdate: func(delta float64) { calls = append(calls, fmt.Sprintf("core-fixed:%g", delta)) },
						OnEnginePause:       func(paused bool) { calls = append(calls, fmt.Sprintf("core-pause:%t", paused)) },
						OnEngineDestroy:     func() { calls = append(calls, "core-destroy") },
					}
					want = append(want, "core-start")
				}
				logical := 0.25
				if fixed > 0 {
					logical = fixed
				}
				want = append(want, fmt.Sprintf("advance:%g", logical), fmt.Sprintf("sprite-update:%g", logical))
				if withCore {
					want = append(want, fmt.Sprintf("core-update:%g", logical))
				}
				want = append(want, fmt.Sprintf("runtime:%g", logical), "sprite-fixed:0.5")
				if withCore {
					want = append(want, "core-fixed:0.5", "core-pause:true", "core-destroy")
				}
				want = append(want, "sprite-destroy")

				if runtime.GOOS == "js" {
					// Web linking requires the host's full JS API; exercise callbacks directly.
					coreCallbacks = standaloneCallbacks(core)
				} else {
					engine.AudioMgr, engine.CameraMgr, engine.DebugMgr = nil, nil, nil
					engine.ExtMgr, engine.InputMgr, engine.NavigationMgr = nil, nil, nil
					engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr = nil, nil, nil
					engine.ResMgr, engine.SceneMgr, engine.SpriteMgr = nil, nil, nil
					engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr = nil, nil, nil
					session := PrepareLink(standaloneCallbacks(core))
					t.Cleanup(func() {
						session.Unlink()
						if activeLink != nil || isWebIntepreterMode || !reflect.ValueOf(coreCallbacks).IsZero() {
							t.Error("Unlink retained the active session or callbacks")
						}
					})
					if activeLink != session || session.backend == nil {
						t.Fatal("PrepareLink did not install its backend session")
					}
					readyCalls := 0
					session.Run(func() { readyCalls++ })
					session.Run(func() { readyCalls++ })
					if readyCalls != 2 || IsWebIntepreterMode() != (linkerBridge{}).IsWebIntepreterMode() {
						t.Fatal("link startup did not signal readiness consistently")
					}
					func() {
						defer func() {
							if recover() == nil {
								t.Error("duplicate link was accepted")
							}
						}()
						PrepareLink(core)
					}()

					for i, manager := range []any{
						engine.AudioMgr, engine.CameraMgr, engine.DebugMgr, engine.ExtMgr, engine.InputMgr,
						engine.NavigationMgr, engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr, engine.ResMgr,
						engine.SceneMgr, engine.SpriteMgr, engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr,
					} {
						if manager == nil {
							t.Fatalf("PrepareLink did not bind manager %d", i)
						}
					}
				}
				callbacks := bindCallbacks()
				callbacks.OnEngineStart()
				callbacks.OnEngineUpdate(0.25)
				callbacks.OnEngineFixedUpdate(0.5)
				callbacks.OnEnginePause(true)
				callbacks.OnEngineDestroy()
				if !slices.Equal(calls, want) {
					t.Fatalf("lifecycle calls = %v, want %v", calls, want)
				}
			})
		}
	}
	if runtime.GOOS != "js" {
		bridge := linkerBridge{}
		bridge.Link(engine.CoreCallbackInfo{})
		if activeLink == nil {
			t.Fatal("public link did not install an active session")
		}
		bridge.Unlink()
		bridge.Unlink()
		if activeLink != nil || IsWebIntepreterMode() {
			t.Fatal("public unlink retained session state")
		}
	}
}

type routedCallbackRuntime struct {
	engine.RuntimeBridge
	sprites   map[engine.Object]engine.ISpriter
	nodes     map[engine.Object]engine.IUiNode
	boundID   engine.Object
	boundType string
}

func (r *routedCallbackRuntime) GetSprite(id engine.Object) engine.ISpriter { return r.sprites[id] }
func (r *routedCallbackRuntime) GetUINode(id engine.Object) engine.IUiNode  { return r.nodes[id] }
func (r *routedCallbackRuntime) DeleteSprite(id engine.Object)              { delete(r.sprites, id) }
func (r *routedCallbackRuntime) DeleteUINode(id engine.Object)              { delete(r.nodes, id) }
func (r *routedCallbackRuntime) BindSceneInstantiatedSprite(id engine.Object, name string) {
	r.boundID, r.boundType = id, name
}

type routedCallbackSprite struct {
	engine.ISpriter
	calls []string
	other engine.ISpriter
}

func (s *routedCallbackSprite) OnStart()           { s.calls = append(s.calls, "start") }
func (s *routedCallbackSprite) V_OnScreenEntered() { s.calls = append(s.calls, "V_OnScreenEntered") }
func (s *routedCallbackSprite) OnScreenEntered()   { s.calls = append(s.calls, "OnScreenEntered") }
func (s *routedCallbackSprite) V_OnScreenExited()  { s.calls = append(s.calls, "V_OnScreenExited") }
func (s *routedCallbackSprite) OnScreenExited()    { s.calls = append(s.calls, "OnScreenExited") }
func (s *routedCallbackSprite) V_OnVfxFinished()   { s.calls = append(s.calls, "V_OnVfxFinished") }
func (s *routedCallbackSprite) OnVfxFinished()     { s.calls = append(s.calls, "OnVfxFinished") }
func (s *routedCallbackSprite) V_OnAnimationFinished() {
	s.calls = append(s.calls, "V_OnAnimationFinished")
}
func (s *routedCallbackSprite) OnAnimationFinished() {
	s.calls = append(s.calls, "OnAnimationFinished")
}
func (s *routedCallbackSprite) V_OnAnimationLooped() {
	s.calls = append(s.calls, "V_OnAnimationLooped")
}
func (s *routedCallbackSprite) OnAnimationLooped() { s.calls = append(s.calls, "OnAnimationLooped") }
func (s *routedCallbackSprite) V_OnFrameChanged()  { s.calls = append(s.calls, "V_OnFrameChanged") }
func (s *routedCallbackSprite) OnFrameChanged()    { s.calls = append(s.calls, "OnFrameChanged") }
func (s *routedCallbackSprite) V_OnAnimationChanged() {
	s.calls = append(s.calls, "V_OnAnimationChanged")
}
func (s *routedCallbackSprite) OnAnimationChanged() { s.calls = append(s.calls, "OnAnimationChanged") }
func (s *routedCallbackSprite) V_OnFramesSetChanged() {
	s.calls = append(s.calls, "V_OnFramesSetChanged")
}
func (s *routedCallbackSprite) OnFramesSetChanged() { s.calls = append(s.calls, "OnFramesSetChanged") }
func (s *routedCallbackSprite) V_OnTriggerEnter(other engine.ISpriter) {
	s.calls = append(s.calls, "V_OnTriggerEnter")
	s.other = other
}
func (s *routedCallbackSprite) OnTriggerEnter(other engine.ISpriter) {
	s.calls = append(s.calls, "OnTriggerEnter")
	s.other = other
}
func (s *routedCallbackSprite) V_OnTriggerExit(other engine.ISpriter) {
	s.calls = append(s.calls, "V_OnTriggerExit")
	s.other = other
}
func (s *routedCallbackSprite) OnTriggerExit(other engine.ISpriter) {
	s.calls = append(s.calls, "OnTriggerExit")
	s.other = other
}

type routedCallbackNode struct {
	engine.IUiNode
	calls []string
}

func (n *routedCallbackNode) V_OnUiPressed()  { n.calls = append(n.calls, "V_OnUiPressed") }
func (n *routedCallbackNode) OnUiPressed()    { n.calls = append(n.calls, "OnUiPressed") }
func (n *routedCallbackNode) V_OnUiReleased() { n.calls = append(n.calls, "V_OnUiReleased") }
func (n *routedCallbackNode) OnUiReleased()   { n.calls = append(n.calls, "OnUiReleased") }
func (n *routedCallbackNode) V_OnUiHovered()  { n.calls = append(n.calls, "V_OnUiHovered") }
func (n *routedCallbackNode) OnUiHovered()    { n.calls = append(n.calls, "OnUiHovered") }
func (n *routedCallbackNode) V_OnUiClick()    { n.calls = append(n.calls, "V_OnUiClick") }
func (n *routedCallbackNode) OnUiClick()      { n.calls = append(n.calls, "OnUiClick") }
func (n *routedCallbackNode) V_OnUiToggle(value bool) {
	n.calls = append(n.calls, fmt.Sprintf("V_OnUiToggle:%t", value))
}
func (n *routedCallbackNode) OnUiToggle(value bool) {
	n.calls = append(n.calls, fmt.Sprintf("OnUiToggle:%t", value))
}
func (n *routedCallbackNode) V_OnUiTextChanged(value string) {
	n.calls = append(n.calls, fmt.Sprintf("V_OnUiTextChanged:%s", value))
}
func (n *routedCallbackNode) OnUiTextChanged(value string) {
	n.calls = append(n.calls, fmt.Sprintf("OnUiTextChanged:%s", value))
}

func TestCallbacksRouteToLiveObjectsInOrder(t *testing.T) {
	sprite, other, node := &routedCallbackSprite{}, &routedCallbackSprite{}, &routedCallbackNode{}
	bridge := &routedCallbackRuntime{sprites: map[engine.Object]engine.ISpriter{1: sprite, 2: other}, nodes: map[engine.Object]engine.IUiNode{3: node}}
	engine.SetRuntimeBridge(bridge)
	t.Cleanup(func() { engine.SetRuntimeBridge(nil) })
	callbacks := bindCallbacks()
	callbacks.OnSceneSpriteInstantiated(41, "Cat")
	if bridge.boundID != 41 || bridge.boundType != "Cat" {
		t.Fatalf("scene binding=%d/%s", bridge.boundID, bridge.boundType)
	}
	callbacks.OnSpriteReady(1)
	callbacks.OnSpriteReady(999)
	if !slices.Equal(sprite.calls, []string{"start"}) {
		t.Fatalf("start callbacks=%v", sprite.calls)
	}
	for _, tc := range []struct {
		name string
		call func(int64)
	}{
		{"ScreenEntered", callbacks.OnSpriteScreenEntered},
		{"ScreenExited", callbacks.OnSpriteScreenExited},
		{"VfxFinished", callbacks.OnSpriteVfxFinished},
		{"AnimationFinished", callbacks.OnSpriteAnimationFinished},
		{"AnimationLooped", callbacks.OnSpriteAnimationLooped},
		{"FrameChanged", callbacks.OnSpriteFrameChanged},
		{"AnimationChanged", callbacks.OnSpriteAnimationChanged},
		{"FramesSetChanged", callbacks.OnSpriteFramesSetChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sprite.calls = nil
			tc.call(1)
			tc.call(999)
			want := []string{"V_On" + tc.name, "On" + tc.name}
			if !slices.Equal(sprite.calls, want) {
				t.Fatalf("calls=%v, want %v", sprite.calls, want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		call func(int64, int64)
	}{{"TriggerEnter", callbacks.OnTriggerEnter}, {"TriggerExit", callbacks.OnTriggerExit}} {
		sprite.calls = nil
		tc.call(999, 2)
		tc.call(1, 999)
		tc.call(1, 2)
		if !slices.Equal(sprite.calls, []string{"V_On" + tc.name, "On" + tc.name}) || sprite.other != other {
			t.Fatalf("%s calls=%v, other=%v", tc.name, sprite.calls, sprite.other)
		}
	}
	for _, tc := range []struct {
		name string
		call func(int64)
	}{{"Pressed", callbacks.OnUiPressed}, {"Released", callbacks.OnUiReleased}, {"Hovered", callbacks.OnUiHovered}, {"Click", callbacks.OnUiClicked}} {
		node.calls = nil
		tc.call(3)
		tc.call(999)
		if !slices.Equal(node.calls, []string{"V_OnUi" + tc.name, "OnUi" + tc.name}) {
			t.Fatalf("%s calls=%v", tc.name, node.calls)
		}
	}
	node.calls = nil
	callbacks.OnUiToggle(999, true)
	callbacks.OnUiTextChanged(999, "ignored")
	callbacks.OnUiToggle(3, true)
	callbacks.OnUiTextChanged(3, "hello")
	if !slices.Equal(node.calls, []string{"V_OnUiToggle:true", "OnUiToggle:true", "V_OnUiTextChanged:hello", "OnUiTextChanged:hello"}) {
		t.Fatalf("UI value callbacks=%v", node.calls)
	}
	callbacks.OnSpriteDestroyed(1)
	callbacks.OnUiDestroyed(3)
	if bridge.sprites[1] != nil || bridge.nodes[3] != nil || bridge.sprites[2] != other {
		t.Fatal("destroy callbacks removed wrong objects")
	}
}

func TestCoreInputCallbacksAndDiagnosticEvents(t *testing.T) {
	previous := coreCallbacks
	t.Cleanup(func() { coreCallbacks = previous; spxlog.SetLevel(spxlog.LevelInfo); spxlog.SetStdoutOutput(os.Stdout) })
	var output bytes.Buffer
	spxlog.SetLevel(spxlog.LevelDebug)
	spxlog.SetStdoutOutput(&output)
	callbacks := bindCallbacks()
	var calls []string
	core := engine.CoreCallbackInfo{
		OnEngineDestroyed: func() { calls = append(calls, "destroyed") }, OnEngineReset: func() { calls = append(calls, "reset") },
		OnMousePressed: func(id int64) { calls = append(calls, fmt.Sprintf("mouse+:%d", id)) }, OnMouseReleased: func(id int64) { calls = append(calls, fmt.Sprintf("mouse-:%d", id)) },
		OnKeyPressed: func(id int64) { calls = append(calls, fmt.Sprintf("key+:%d", id)) }, OnKeyReleased: func(id int64) { calls = append(calls, fmt.Sprintf("key-:%d", id)) },
	}
	for _, info := range []engine.CoreCallbackInfo{{}, core} {
		coreCallbacks = info
		callbacks.OnEngineDestroyed()
		callbacks.OnEngineReset()
		callbacks.OnMousePressed(1)
		callbacks.OnMouseReleased(2)
		callbacks.OnKeyPressed(3)
		callbacks.OnKeyReleased(4)
	}
	if !slices.Equal(calls, []string{"destroyed", "reset", "mouse+:1", "mouse-:2", "key+:3", "key-:4"}) {
		t.Fatalf("core callbacks=%v", calls)
	}
	callbacks.OnSpriteUpdated(.25)
	callbacks.OnSpriteFixedUpdated(.5)
	callbacks.OnActionPressed("jump")
	callbacks.OnActionJustPressed("jump")
	callbacks.OnActionJustReleased("jump")
	callbacks.OnAxisChanged("move", -.5)
	callbacks.OnCollisionEnter(1, 2)
	callbacks.OnCollisionStay(1, 2)
	callbacks.OnCollisionExit(1, 2)
	for _, want := range []string{"OnSpriteUpdated 0.250000", "OnSpriteFixedUpdated 0.500000", "OnActionPressed jump", "OnActionJustPressed jump", "OnActionJustReleased jump", "OnAxisChanged move -0.500000", "OnCollisionEnter 1 2", "OnCollisionStay 1 2", "OnCollisionExit 1 2"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing diagnostic %q", want)
		}
	}
}
