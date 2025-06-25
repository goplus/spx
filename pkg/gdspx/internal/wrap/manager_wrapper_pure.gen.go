//go:build pure_engine

/*
------------------------------------------------------------------------------
//   This code was generated for pure engine mode (no FFI).
//
//   Changes to this file may cause incorrect behavior and will be lost if
//   the code is regenerated.
//----------------------------------------------------------------------------
*/
package wrap

import (
	"fmt"
	"reflect"

	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

func BindMgr(mgrs []IManager) {
	for _, mgr := range mgrs {
		switch v := mgr.(type) {
		case IAudioMgr:
			AudioMgr = v

		case ICameraMgr:
			CameraMgr = v

		case IExtMgr:
			ExtMgr = v

		case IInputMgr:
			InputMgr = v

		case IPhysicMgr:
			PhysicMgr = v

		case IPlatformMgr:
			PlatformMgr = v

		case IResMgr:
			ResMgr = v

		case ISceneMgr:
			SceneMgr = v

		case ISpriteMgr:
			SpriteMgr = v

		case IUiMgr:
			UiMgr = v

		default:
			panic(fmt.Sprintf("engine init error : unknown manager type %s", reflect.TypeOf(mgr).String()))
		}
	}
}

type audioMgr struct {
	baseMgr
}
type cameraMgr struct {
	baseMgr
}
type extMgr struct {
	baseMgr
}
type inputMgr struct {
	baseMgr
}
type physicMgr struct {
	baseMgr
}
type platformMgr struct {
	baseMgr
}
type resMgr struct {
	baseMgr
}
type sceneMgr struct {
	baseMgr
}
type spriteMgr struct {
	baseMgr
}
type uiMgr struct {
	baseMgr
}

func createMgrs() []IManager {
	addManager(&audioMgr{})
	addManager(&cameraMgr{})
	addManager(&extMgr{})
	addManager(&inputMgr{})
	addManager(&physicMgr{})
	addManager(&platformMgr{})
	addManager(&resMgr{})
	addManager(&sceneMgr{})
	addManager(&spriteMgr{})
	addManager(&uiMgr{})
	return mgrs
}

// Pure Go implementations (no FFI calls)

// Audio Manager
func (pself *audioMgr) StopAll() {
	// Pure implementation - no operation
}
func (pself *audioMgr) CreateAudio() Object {
	return Object(0)
}
func (pself *audioMgr) DestroyAudio(obj Object) {
	// Pure implementation - no operation
}
func (pself *audioMgr) SetPitch(obj Object, pitch float64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) GetPitch(obj Object) float64 {
	return 0.0
}
func (pself *audioMgr) SetPan(obj Object, pan float64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) GetPan(obj Object) float64 {
	return 0.0
}
func (pself *audioMgr) SetVolume(obj Object, volume float64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) GetVolume(obj Object) float64 {
	return 1.0
}
func (pself *audioMgr) Play(obj Object, path string) int64 {
	return 0
}
func (pself *audioMgr) Pause(aid int64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) Resume(aid int64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) Stop(aid int64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) SetLoop(aid int64, loop bool) {
	// Pure implementation - no operation
}
func (pself *audioMgr) GetLoop(aid int64) bool {
	return false
}
func (pself *audioMgr) GetTimer(aid int64) float64 {
	return 0.0
}
func (pself *audioMgr) SetTimer(aid int64, time float64) {
	// Pure implementation - no operation
}
func (pself *audioMgr) IsPlaying(aid int64) bool {
	return false
}

// Camera Manager
func (pself *cameraMgr) GetCameraPosition() Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *cameraMgr) SetCameraPosition(position Vec2) {
	// Pure implementation - no operation
}
func (pself *cameraMgr) GetCameraZoom() Vec2 {
	return Vec2{X: 1, Y: 1}
}
func (pself *cameraMgr) SetCameraZoom(size Vec2) {
	// Pure implementation - no operation
}
func (pself *cameraMgr) GetViewportRect() Rect2 {
	return Rect2{Position: Vec2{X: 0, Y: 0}, Size: Vec2{X: 800, Y: 600}}
}

// Extension Manager
func (pself *extMgr) RequestExit(exit_code int64) {
	// Pure implementation - no operation
}
func (pself *extMgr) OnRuntimePanic(msg string) {
	// Pure implementation - no operation
}
func (pself *extMgr) DestroyAllPens() {
	// Pure implementation - no operation
}
func (pself *extMgr) CreatePen() Object {
	return Object(0)
}
func (pself *extMgr) DestroyPen(obj Object) {
	// Pure implementation - no operation
}
func (pself *extMgr) PenStamp(obj Object) {
	// Pure implementation - no operation
}
func (pself *extMgr) MovePenTo(obj Object, position Vec2) {
	// Pure implementation - no operation
}
func (pself *extMgr) PenDown(obj Object, move_by_mouse bool) {
	// Pure implementation - no operation
}
func (pself *extMgr) PenUp(obj Object) {
	// Pure implementation - no operation
}
func (pself *extMgr) SetPenColorTo(obj Object, color Color) {
	// Pure implementation - no operation
}
func (pself *extMgr) ChangePenBy(obj Object, property int64, amount float64) {
	// Pure implementation - no operation
}
func (pself *extMgr) SetPenTo(obj Object, property int64, value float64) {
	// Pure implementation - no operation
}
func (pself *extMgr) ChangePenSizeBy(obj Object, amount float64) {
	// Pure implementation - no operation
}
func (pself *extMgr) SetPenSizeTo(obj Object, size float64) {
	// Pure implementation - no operation
}
func (pself *extMgr) SetPenStampTexture(obj Object, texture_path string) {
	// Pure implementation - no operation
}

// Input Manager
func (pself *inputMgr) GetMousePos() Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *inputMgr) GetKey(key int64) bool {
	return false
}
func (pself *inputMgr) GetMouseState(mouse_id int64) bool {
	return false
}
func (pself *inputMgr) GetKeyState(key int64) int64 {
	return 0
}
func (pself *inputMgr) GetAxis(neg_action string, pos_action string) float64 {
	return 0.0
}
func (pself *inputMgr) IsActionPressed(action string) bool {
	return false
}
func (pself *inputMgr) IsActionJustPressed(action string) bool {
	return false
}
func (pself *inputMgr) IsActionJustReleased(action string) bool {
	return false
}

// Physics Manager
func (pself *physicMgr) Raycast(from Vec2, to Vec2, collision_mask int64) Object {
	return Object(0)
}
func (pself *physicMgr) CheckCollision(from Vec2, to Vec2, collision_mask int64, collide_with_areas bool, collide_with_bodies bool) bool {
	return false
}
func (pself *physicMgr) CheckTouchedCameraBoundaries(obj Object) int64 {
	return 0
}
func (pself *physicMgr) CheckTouchedCameraBoundary(obj Object, board_type int64) bool {
	return false
}
func (pself *physicMgr) SetCollisionSystemType(is_collision_by_alpha bool) {
	// Pure implementation - no operation
}

// Platform Manager
func (pself *platformMgr) SetWindowPosition(pos Vec2) {
	// Pure implementation - no operation
}
func (pself *platformMgr) GetWindowPosition() Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *platformMgr) SetWindowSize(width int64, height int64) {
	// Pure implementation - no operation
}
func (pself *platformMgr) GetWindowSize() Vec2 {
	return Vec2{X: 800, Y: 600}
}
func (pself *platformMgr) SetWindowTitle(title string) {
	// Pure implementation - no operation
}
func (pself *platformMgr) GetWindowTitle() string {
	return "SPX Pure Engine"
}
func (pself *platformMgr) SetWindowFullscreen(enable bool) {
	// Pure implementation - no operation
}
func (pself *platformMgr) IsWindowFullscreen() bool {
	return false
}
func (pself *platformMgr) SetDebugMode(enable bool) {
	// Pure implementation - no operation
}
func (pself *platformMgr) IsDebugMode() bool {
	return false
}
func (pself *platformMgr) GetTimeScale() float64 {
	return 1.0
}
func (pself *platformMgr) SetTimeScale(time_scale float64) {
	// Pure implementation - no operation
}
func (pself *platformMgr) GetPersistantDataDir() string {
	return "/tmp"
}
func (pself *platformMgr) SetPersistantDataDir(path string) {
	// Pure implementation - no operation
}
func (pself *platformMgr) IsInPersistantDataDir(path string) bool {
	return false
}

// Resource Manager
func (pself *resMgr) CreateAnimation(sprite_type_name string, anim_name string, context string, fps int64, is_altas bool) {
	// Pure implementation - no operation
}
func (pself *resMgr) SetLoadMode(is_direct_mode bool) {
	// Pure implementation - no operation
}
func (pself *resMgr) GetLoadMode() bool {
	return true
}
func (pself *resMgr) GetBoundFromAlpha(p_path string) Rect2 {
	return Rect2{Position: Vec2{X: 0, Y: 0}, Size: Vec2{X: 100, Y: 100}}
}
func (pself *resMgr) GetImageSize(p_path string) Vec2 {
	return Vec2{X: 100, Y: 100}
}
func (pself *resMgr) ReadAllText(p_path string) string {
	return ""
}
func (pself *resMgr) HasFile(p_path string) bool {
	return false
}
func (pself *resMgr) ReloadTexture(path string) {
	// Pure implementation - no operation
}
func (pself *resMgr) FreeStr(str string) {
	// Pure implementation - no operation
}
func (pself *resMgr) SetDefaultFont(font_path string) {
	// Pure implementation - no operation
}

// Scene Manager
func (pself *sceneMgr) ChangeSceneToFile(path string) {
	// Pure implementation - no operation
}
func (pself *sceneMgr) DestroyAllSprites() {
	// Pure implementation - no operation
}
func (pself *sceneMgr) ReloadCurrentScene() int64 {
	return 0
}
func (pself *sceneMgr) UnloadCurrentScene() {
	// Pure implementation - no operation
}

// Sprite Manager
func (pself *spriteMgr) SetDontDestroyOnLoad(obj Object) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) SetProcess(obj Object, is_on bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) SetPhysicProcess(obj Object, is_on bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) SetTypeName(obj Object, type_name string) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) SetChildPosition(obj Object, path string, pos Vec2) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetChildPosition(obj Object, path string) Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *spriteMgr) SetChildRotation(obj Object, path string, rot float64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetChildRotation(obj Object, path string) float64 {
	return 0.0
}
func (pself *spriteMgr) SetChildScale(obj Object, path string, scale Vec2) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetChildScale(obj Object, path string) Vec2 {
	return Vec2{X: 1, Y: 1}
}
func (pself *spriteMgr) CheckCollision(obj Object, target Object, is_src_trigger bool, is_dst_trigger bool) bool {
	return false
}
func (pself *spriteMgr) CheckCollisionWithPoint(obj Object, point Vec2, is_trigger bool) bool {
	return false
}
func (pself *spriteMgr) CreateBackdrop(path string) Object {
	return Object(0)
}
func (pself *spriteMgr) CreateSprite(path string) Object {
	return Object(0)
}
func (pself *spriteMgr) CloneSprite(obj Object) Object {
	return Object(0)
}
func (pself *spriteMgr) DestroySprite(obj Object) bool {
	return true
}
func (pself *spriteMgr) IsSpriteAlive(obj Object) bool {
	return false
}
func (pself *spriteMgr) SetPosition(obj Object, pos Vec2) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetPosition(obj Object) Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *spriteMgr) SetRotation(obj Object, rot float64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetRotation(obj Object) float64 {
	return 0.0
}
func (pself *spriteMgr) SetScale(obj Object, scale Vec2) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetScale(obj Object) Vec2 {
	return Vec2{X: 1, Y: 1}
}
func (pself *spriteMgr) SetRenderScale(obj Object, scale Vec2) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetRenderScale(obj Object) Vec2 {
	return Vec2{X: 1, Y: 1}
}
func (pself *spriteMgr) SetColor(obj Object, color Color) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetColor(obj Object) Color {
	return Color{R: 1, G: 1, B: 1, A: 1}
}
func (pself *spriteMgr) SetMaterialShader(obj Object, path string) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetMaterialShader(obj Object) string {
	return ""
}
func (pself *spriteMgr) SetMaterialParams(obj Object, effect string, amount float64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetMaterialParams(obj Object, effect string) float64 {
	return 0.0
}
func (pself *spriteMgr) SetMaterialParamsVec(obj Object, effect string, x float64, y float64, z float64, w float64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetMaterialParamsVec(obj Object, effect string) (float64, float64, float64, float64) {
	return 0.0, 0.0, 0.0, 0.0
}
func (pself *spriteMgr) SetVisible(obj Object, visible bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) IsVisible(obj Object) bool {
	return true
}
func (pself *spriteMgr) SetZIndex(obj Object, z_index int64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetZIndex(obj Object) int64 {
	return 0
}
func (pself *spriteMgr) SetTexture(obj Object, path string) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetTexture(obj Object) string {
	return ""
}
func (pself *spriteMgr) SetAnimation(obj Object, anim_name string) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetAnimation(obj Object) string {
	return ""
}
func (pself *spriteMgr) PlayAnimation(obj Object, anim_name string) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) StopAnimation(obj Object) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) IsAnimationPlaying(obj Object) bool {
	return false
}
func (pself *spriteMgr) SetAnimationSpeed(obj Object, speed float64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetAnimationSpeed(obj Object) float64 {
	return 1.0
}
func (pself *spriteMgr) SetFlip(obj Object, horizontal bool, is_flip bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetFlip(obj Object, horizontal bool) bool {
	return false
}
func (pself *spriteMgr) SetCollisionLayer(obj Object, layer int64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetCollisionLayer(obj Object) int64 {
	return 0
}
func (pself *spriteMgr) SetCollisionMask(obj Object, mask int64) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) GetCollisionMask(obj Object) int64 {
	return 0
}
func (pself *spriteMgr) SetCollisionEnabled(obj Object, enabled bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) IsCollisionEnabled(obj Object) bool {
	return false
}
func (pself *spriteMgr) SetTriggerEnabled(obj Object, enabled bool) {
	// Pure implementation - no operation
}
func (pself *spriteMgr) IsTriggerEnabled(obj Object) bool {
	return false
}

// UI Manager
func (pself *uiMgr) CreateControl(control_type int64) Object {
	return Object(0)
}
func (pself *uiMgr) DestroyControl(obj Object) {
	// Pure implementation - no operation
}
func (pself *uiMgr) SetText(obj Object, text string) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetText(obj Object) string {
	return ""
}
func (pself *uiMgr) SetFontSize(obj Object, size int64) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetFontSize(obj Object) int64 {
	return 12
}
func (pself *uiMgr) SetColor(obj Object, color Color) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetColor(obj Object) Color {
	return Color{R: 1, G: 1, B: 1, A: 1}
}
func (pself *uiMgr) SetVisible(obj Object, visible bool) {
	// Pure implementation - no operation
}
func (pself *uiMgr) IsVisible(obj Object) bool {
	return true
}
func (pself *uiMgr) SetLayoutMode(obj Object, mode int64) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetLayoutMode(obj Object) int64 {
	return 0
}
func (pself *uiMgr) GetAnchorsPreset(obj Object) int64 {
	return 0
}
func (pself *uiMgr) SetAnchorsPreset(obj Object, value int64) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetScale(obj Object) Vec2 {
	return Vec2{X: 1, Y: 1}
}
func (pself *uiMgr) SetScale(obj Object, value Vec2) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetPosition(obj Object) Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *uiMgr) SetPosition(obj Object, value Vec2) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetSize(obj Object) Vec2 {
	return Vec2{X: 100, Y: 100}
}
func (pself *uiMgr) SetSize(obj Object, value Vec2) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetGlobalPosition(obj Object) Vec2 {
	return Vec2{X: 0, Y: 0}
}
func (pself *uiMgr) SetGlobalPosition(obj Object, value Vec2) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetRotation(obj Object) float64 {
	return 0.0
}
func (pself *uiMgr) SetRotation(obj Object, value float64) {
	// Pure implementation - no operation
}
func (pself *uiMgr) GetFlip(obj Object, horizontal bool) bool {
	return false
}
func (pself *uiMgr) SetFlip(obj Object, horizontal bool, is_flip bool) {
	// Pure implementation - no operation
}
