package engine

import (
	"reflect"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type runtimeState struct {
	sprites        map[Object]gdx.ISpriter
	uiNodes        map[Object]gdx.IUiNode
	spriteTypes    map[string]reflect.Type
	timeSinceStart float64
}

var state = runtimeState{
	sprites:     make(map[Object]gdx.ISpriter),
	uiNodes:     make(map[Object]gdx.IUiNode),
	spriteTypes: make(map[string]reflect.Type),
}

func init() {
	gdx.SetRuntimeBridge(runtimeBridge{})
}

type runtimeBridge struct{}

func (runtimeBridge) InternalUpdateEngine(delta float64) {
	updateTimers(delta)
	updateTweens(delta)
}

func (runtimeBridge) ClearAllSprites() {
	clearAllSprites()
}

func (runtimeBridge) RegisterSpriteType(t reflect.Type) {
	state.spriteTypes[t.Name()] = t
}

func (runtimeBridge) GetSprite(id Object) gdx.ISpriter {
	return lookupSprite(id)
}

func (runtimeBridge) BindSceneInstantiatedSprite(id Object, typeName string) {
	bindSceneInstantiatedSprite(id, typeName)
}

func (runtimeBridge) CreateSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	return createPrefabSprite(t, pos)
}

func (runtimeBridge) CreateEmptySprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	return createBareSprite(t, pos)
}

func (runtimeBridge) CreateBackdrop(t reflect.Type) reflect.Value {
	return createBackdrop(t)
}

func (runtimeBridge) CreateUI(t reflect.Type, prefabName string, isEngine bool) reflect.Value {
	return createUI(t, prefabName, isEngine)
}

func (runtimeBridge) BindUI(t reflect.Type, parentNode Object, path string) reflect.Value {
	return bindUI(t, parentNode, path)
}

func (runtimeBridge) DelayCall(delay float64, callback func()) {
	delayCall(delay, callback)
}

func (runtimeBridge) DelaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	delaySpriteCall(delay, sprite, callback)
}

func (runtimeBridge) TweenPos(node gdx.ISpriter, pos mathf.Vec2, duration float64, callback func()) {
	tweenPos(node, pos, duration, callback)
}

func (runtimeBridge) TweenPos2(node gdx.ISpriter, pos mathf.Vec2, duration float64, pos2 mathf.Vec2, duration2 float64, callback func()) {
	tweenPos2(node, pos, duration, pos2, duration2, callback)
}

func (runtimeBridge) Sprites() map[Object]gdx.ISpriter {
	return state.sprites
}

func (runtimeBridge) UiNodes() map[Object]gdx.IUiNode {
	return state.uiNodes
}

func (runtimeBridge) GetUINode(id Object) gdx.IUiNode {
	if node, ok := state.uiNodes[id]; ok {
		return node
	}
	return nil
}

func (runtimeBridge) DeleteSprite(id Object) {
	delete(state.sprites, id)
}

func (runtimeBridge) DeleteUINode(id Object) {
	delete(state.uiNodes, id)
}

func (runtimeBridge) AdvanceTimeSinceGameStart(delta float64) float64 {
	state.timeSinceStart += delta
	return state.timeSinceStart
}

func (runtimeBridge) TimeSinceGameStarted() float64 {
	return state.timeSinceStart
}

func IsNodeExist(id Object) bool {
	return isNodeExist(id)
}

func CreateBareSpriteForType[T any](pos mathf.Vec2) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createBareSprite(tType, pos)
	return value.Addr().Interface().(*T)
}

func CreateBackdropForType[T any]() *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createBackdrop(tType)
	return value.Addr().Interface().(*T)
}

func CreateEngineUIForType[T any](prefabName string) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createUI(tType, prefabName, true)
	return value.Addr().Interface().(*T)
}

func BindUIForType[T any](parentNode Object, path string) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := bindUI(tType, parentNode, path)
	if !value.IsValid() {
		return nil
	}
	return value.Addr().Interface().(*T)
}

func clearAllSprites() {
	for id, sprite := range state.sprites {
		sprite.Destroy()
		delete(state.sprites, id)
	}
	for id, node := range state.uiNodes {
		node.Destroy()
		delete(state.uiNodes, id)
	}
}

func lookupSprite(id Object) gdx.ISpriter {
	if sprite, ok := state.sprites[id]; ok {
		return sprite
	}
	return nil
}

func isNodeExist(id Object) bool {
	if _, ok := state.uiNodes[id]; ok {
		return true
	}
	if _, ok := state.sprites[id]; ok {
		return true
	}
	return false
}

func bindSceneInstantiatedSprite(id Object, typeName string) {
	if t, ok := state.spriteTypes[typeName]; ok {
		createSpriteValue(t, id)
		return
	}
	spxlog.Error("BindSceneInstantiatedSprite: type not found %s", typeName)
}

func createPrefabSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	id := Managers().SpriteMgr.CreateSprite(getPrefabPath(t.Name()), pos)
	return createSpriteValue(t, id)
}

func createBareSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	id := Managers().SpriteMgr.CreateBareSprite(pos)
	return createSpriteValue(t, id)
}

func createBackdrop(t reflect.Type) reflect.Value {
	id := Managers().SpriteMgr.CreateBackdrop("")
	return createSpriteValue(t, id)
}

func createUI(t reflect.Type, prefabName string, isEngine bool) reflect.Value {
	name := t.Name()
	if prefabName != "" {
		name = prefabName
	}
	nodeValue := reflect.New(t).Elem()
	id := Managers().UiMgr.CreateNode(getUIPath(name, isEngine))
	node := nodeValue.Addr().Interface().(gdx.IUiNode)
	gdx.InitUINodeInstance(id, node, func(id Object, node gdx.IUiNode) {
		state.uiNodes[id] = node
	})
	return nodeValue
}

func bindUI(t reflect.Type, parentNode Object, path string) reflect.Value {
	id := Managers().UiMgr.BindNode(parentNode, path)
	if id == 0 {
		spxlog.Error("BindUI failed: parentNode=%d path=%s", parentNode, path)
		return reflect.Value{}
	}
	nodeValue := reflect.New(t).Elem()
	node := nodeValue.Addr().Interface().(gdx.IUiNode)
	gdx.InitUINodeInstance(id, node, func(id Object, node gdx.IUiNode) {
		state.uiNodes[id] = node
	})
	return nodeValue
}

func createSpriteValue(t reflect.Type, id Object) reflect.Value {
	spriteValue := reflect.New(t).Elem()
	sprite := spriteValue.Addr().Interface().(gdx.ISpriter)
	gdx.InitSpriteInstance(id, sprite, func(id Object, sprite gdx.ISpriter) {
		state.sprites[id] = sprite
	})
	return spriteValue
}

func getPrefabPath(name string) string {
	return "res://assets/prefabs/" + name + ".tscn"
}

func getUIPath(name string, isEngine bool) string {
	if isEngine {
		return "res://engine/ui/" + name + ".tscn"
	}
	return "res://assets/ui/" + name + ".tscn"
}
