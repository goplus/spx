package ffi

/*
#include "gdextension_spx_interface.h"
*/
import "C"

import (
	"unsafe"

	"github.com/goplus/spx/pkg/gdspx/internal/platform"
	"github.com/goplus/spx/pkg/gdspx/pkg/engine"
	"github.com/realdream-ai/mathf"
)

type Uint64T C.uint64_t
type Uint32T C.uint32_t
type Uint16T C.uint16_t
type Uint8T C.uint8_t
type Int32T C.int32_t
type Int16T C.int16_t
type Int8T C.int8_t
type Char C.char
type WcharT C.wchar_t
type Char32T C.char32_t
type Char16T C.char16_t

type GdString C.GdString
type GdInt C.GdInt
type GdBool C.GdBool
type GdFloat C.GdFloat
type GdVec4 C.GdVec4
type GdVec3 C.GdVec3
type GdVec2 C.GdVec2
type GdColor C.GdColor
type GdRect2 C.GdRect2
type GdObj C.GdObj

func ToGdBool(val bool) GdBool {
	if val {
		return GdBool(1)
	} else {
		return GdBool(0)
	}
}
func ToBool(val GdBool) bool {
	return val != 0
}
func ToGdVec2(val mathf.Vec2) GdVec2 {
	return GdVec2{C.GdFloat(val.X), C.GdFloat(val.Y)}
}
func ToVec2(val GdVec2) mathf.Vec2 {
	return mathf.NewVec2(float64(val.X), float64(val.Y))
}
func ToGdVec4(val mathf.Vec4) GdVec4 {
	return GdVec4{C.GdFloat(val.X), C.GdFloat(val.Y), C.GdFloat(val.Z), C.GdFloat(val.W)}
}
func ToVec4(val GdVec4) mathf.Vec4 {
	return mathf.NewVec4(float64(val.X), float64(val.Y), float64(val.Z), float64(val.W))
}
func ToGdColor(val mathf.Color) GdColor {
	return GdColor{C.float(val.R), C.float(val.G), C.float(val.B), C.float(val.A)}
}
func ToColor(val GdColor) mathf.Color {
	return mathf.NewColor(float64(val.R), float64(val.G), float64(val.B), float64(val.A))
}
func ToGdRect2(val mathf.Rect2) GdRect2 {
	position := ToGdVec2(val.Position)
	size := ToGdVec2(val.Size)
	ret := GdRect2{}
	ret.Position = C.GdVec2(position)
	ret.Size = C.GdVec2(size)
	return ret
}
func ToRect2(val GdRect2) mathf.Rect2 {
	ret := mathf.Rect2{}
	ret.Position = ToVec2(GdVec2(val.Position))
	ret.Size = ToVec2(GdVec2(val.Size))
	return ret
}
func ToGdObj(val engine.Object) GdObj {
	return GdObj(val)
}
func ToObject(val GdObj) engine.Object {
	return engine.Object(val)
}
func ToGdInt(val int64) GdInt {
	return GdInt(val)
}
func ToInt(val GdInt) int64 {
	return int64(val)
}
func ToInt64(val GdInt) int64 {
	return int64(val)
}
func ToGdFloat(val float64) GdFloat {
	return GdFloat(val)
}
func ToFloat64(val GdFloat) float64 {
	return float64(val)
}
func ToFloat32(val GdFloat) float64 {
	return float64(val)
}
func ToFloat(val GdFloat) float64 {
	return float64(val)
}

func ToString(val GdString) string {
	cstrPtr := (*C.char)(unsafe.Pointer(val))
	str := C.GoString(cstrPtr)
	// free the memory allocated in c++
	// Warning!: Using Go's C.free(unsafe.Pointer(cstrPtr)) to free memory allocated in C++ can cause a crash
	CallResFreeStr(val)
	return str
}

type GDExtensionSpxCallbackInfoPtr C.GDExtensionSpxCallbackInfoPtr
type SpxCallbackInfo C.SpxCallbackInfo

type GDExtensionVariantPtr C.GDExtensionVariantPtr
type GDExtensionConstVariantPtr C.GDExtensionConstVariantPtr
type GDExtensionUninitializedVariantPtr C.GDExtensionUninitializedVariantPtr
type GDExtensionStringNamePtr C.GDExtensionStringNamePtr
type GDExtensionConstStringNamePtr C.GDExtensionConstStringNamePtr
type GDExtensionUninitializedStringNamePtr C.GDExtensionUninitializedStringNamePtr
type GDExtensionStringPtr C.GDExtensionStringPtr
type GDExtensionConstStringPtr C.GDExtensionConstStringPtr
type GDExtensionUninitializedStringPtr C.GDExtensionUninitializedStringPtr
type GDExtensionObjectPtr C.GDExtensionObjectPtr
type GDExtensionConstObjectPtr C.GDExtensionConstObjectPtr
type GDExtensionUninitializedObjectPtr C.GDExtensionUninitializedObjectPtr
type GDExtensionTypePtr C.GDExtensionTypePtr
type GDExtensionConstTypePtr C.GDExtensionConstTypePtr
type GDExtensionUninitializedTypePtr C.GDExtensionUninitializedTypePtr
type GDExtensionMethodBindPtr C.GDExtensionMethodBindPtr
type GDExtensionInt C.GDExtensionInt
type GDExtensionBool C.GDExtensionBool
type GDObjectInstanceID C.GDObjectInstanceID
type GDExtensionRefPtr C.GDExtensionRefPtr
type GDExtensionConstRefPtr C.GDExtensionConstRefPtr

type GDExtensionPtrConstructor C.GDExtensionPtrConstructor
type GDExtensionPtrDestructor C.GDExtensionPtrDestructor
type GDExtensionVariantType C.GDExtensionVariantType

const (
	GDEXTENSION_VARIANT_TYPE_NIL GDExtensionVariantType = iota
	GDEXTENSION_VARIANT_TYPE_BOOL
	GDEXTENSION_VARIANT_TYPE_INT
	GDEXTENSION_VARIANT_TYPE_FLOAT
	GDEXTENSION_VARIANT_TYPE_STRING
	GDEXTENSION_VARIANT_TYPE_VECTOR2
	GDEXTENSION_VARIANT_TYPE_VECTOR2I
	GDEXTENSION_VARIANT_TYPE_RECT2
	GDEXTENSION_VARIANT_TYPE_RECT2I
	GDEXTENSION_VARIANT_TYPE_VECTOR3
	GDEXTENSION_VARIANT_TYPE_VECTOR3I
	GDEXTENSION_VARIANT_TYPE_TRANSFORM2D
	GDEXTENSION_VARIANT_TYPE_VECTOR4
	GDEXTENSION_VARIANT_TYPE_VECTOR4I
	GDEXTENSION_VARIANT_TYPE_PLANE
	GDEXTENSION_VARIANT_TYPE_QUATERNION
	GDEXTENSION_VARIANT_TYPE_AABB
	GDEXTENSION_VARIANT_TYPE_BASIS
	GDEXTENSION_VARIANT_TYPE_TRANSFORM3D
	GDEXTENSION_VARIANT_TYPE_PROJECTION
	GDEXTENSION_VARIANT_TYPE_COLOR
	GDEXTENSION_VARIANT_TYPE_STRING_NAME
	GDEXTENSION_VARIANT_TYPE_NODE_PATH
	GDEXTENSION_VARIANT_TYPE_RID
	GDEXTENSION_VARIANT_TYPE_OBJECT
	GDEXTENSION_VARIANT_TYPE_CALLABLE
	GDEXTENSION_VARIANT_TYPE_SIGNAL
	GDEXTENSION_VARIANT_TYPE_DICTIONARY
	GDEXTENSION_VARIANT_TYPE_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_BYTE_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_INT32_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_INT64_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_FLOAT32_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_FLOAT64_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_STRING_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR2_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR3_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_COLOR_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR4_ARRAY
	GDEXTENSION_VARIANT_TYPE_VARIANT_MAX
)

type GDExtensionInitializationLevel int64

const (
	GDExtensionInitializationLevelCore    GDExtensionInitializationLevel = 0
	GDExtensionInitializationLevelServers GDExtensionInitializationLevel = 1
	GDExtensionInitializationLevelScene   GDExtensionInitializationLevel = 2
	GDExtensionInitializationLevelEditor  GDExtensionInitializationLevel = 3
)

type initialization = C.GDExtensionInitialization
type initializationLevel = C.GDExtensionInitializationLevel

func doInitialization(init *initialization) {
	stringInitConstructorBindings()
	C.initialization(init)
}
func getProcAddress(handle uintptr, name string) unsafe.Pointer {
	name = name + "\000"
	char := C.CString(name)
	defer C.free(unsafe.Pointer(char))
	return C.get_proc_address(C.pointer(handle), char)
}

func registerEngineCallback() {
	spx_global_register_callbacks := dlsymGD("spx_global_register_callbacks")
	C.spx_global_register_callbacks(
		C.pointer(uintptr(spx_global_register_callbacks)),
	)
}
func GDExtensionInterfaceObjectMethodBindPtrcall(
	p_method_bind GDExtensionMethodBindPtr,
	p_instance GDExtensionObjectPtr,
	p_args *GDExtensionConstTypePtr,
	r_ret GDExtensionTypePtr,
) {
}

//export initialize
func initialize(_ unsafe.Pointer, level initializationLevel) {
	platform.Init()
	if level == 2 {
		main()
	}
}

//export deinitialize
func deinitialize(_ unsafe.Pointer, level initializationLevel) {
	if level == 0 {
	}
}

//export func_on_engine_start
func func_on_engine_start() {
	if callbacks.OnEngineStart != nil {
		callbacks.OnEngineStart()
	}
}

//export func_on_engine_update
func func_on_engine_update(delta C.GDReal) {
	if callbacks.OnEngineUpdate != nil {
		callbacks.OnEngineUpdate(float64(delta))
	}
}

//export func_on_engine_fixed_update
func func_on_engine_fixed_update(delta C.GDReal) {
	if callbacks.OnEngineFixedUpdate != nil {
		callbacks.OnEngineFixedUpdate(float64(delta))
	}
}

//export func_on_engine_destroy
func func_on_engine_destroy() {
	if callbacks.OnEngineDestroy != nil {
		callbacks.OnEngineDestroy()
	}
}

//export func_on_scene_sprite_instantiated
func func_on_scene_sprite_instantiated(id C.GDExtensionInt, typeName C.GdString) {
	name := ToString(GdString(typeName))
	if callbacks.OnSceneSpriteInstantiated != nil {
		callbacks.OnSceneSpriteInstantiated(int64(id), name)
	}
}

//export func_on_sprite_ready
func func_on_sprite_ready(id C.GDExtensionInt) {
	if callbacks.OnSpriteReady != nil {
		callbacks.OnSpriteReady(int64(id))
	}
}

//export func_on_sprite_updated
func func_on_sprite_updated(delta C.GDReal) {
	if callbacks.OnSpriteUpdated != nil {
		callbacks.OnSpriteUpdated(float64(delta))
	}
}

//export func_on_sprite_fixed_updated
func func_on_sprite_fixed_updated(delta C.GDReal) {
	if callbacks.OnSpriteFixedUpdated != nil {
		callbacks.OnSpriteFixedUpdated(float64(delta))
	}
}

//export func_on_sprite_destroyed
func func_on_sprite_destroyed(id C.GDExtensionInt) {
	if callbacks.OnSpriteDestroyed != nil {
		callbacks.OnSpriteDestroyed(int64(id))
	}
}

//export func_on_action_pressed
func func_on_action_pressed(actionName C.GdString) {
	name := ToString(GdString(actionName))
	if callbacks.OnSpriteReady != nil {
		callbacks.OnActionPressed(name)
	}
}

//export func_on_mouse_pressed
func func_on_mouse_pressed(keyid C.GDExtensionInt) {
	if callbacks.OnMousePressed != nil {
		callbacks.OnMousePressed(int64(keyid))
	}
}

//export func_on_mouse_released
func func_on_mouse_released(keyid C.GDExtensionInt) {
	if callbacks.OnMouseReleased != nil {
		callbacks.OnMouseReleased(int64(keyid))
	}
}

//export func_on_key_pressed
func func_on_key_pressed(keyid C.GDExtensionInt) {
	if callbacks.OnKeyPressed != nil {
		callbacks.OnKeyPressed(int64(keyid))
	}
}

//export func_on_key_released
func func_on_key_released(keyid C.GDExtensionInt) {
	if callbacks.OnKeyReleased != nil {
		callbacks.OnKeyReleased(int64(keyid))
	}
}

//export func_on_action_just_pressed
func func_on_action_just_pressed(actionName C.GdString) {
	name := ToString(GdString(actionName))
	if callbacks.OnActionJustPressed != nil {
		callbacks.OnActionJustPressed(name)
	}
}

//export func_on_action_just_released
func func_on_action_just_released(actionName C.GdString) {
	name := ToString(GdString(actionName))
	if callbacks.OnActionJustReleased != nil {
		callbacks.OnActionJustReleased(name)
	}
}

//export func_on_axis_changed
func func_on_axis_changed(actionName C.GdString, value C.GDReal) {
	name := ToString(GdString(actionName))
	if callbacks.OnAxisChanged != nil {
		callbacks.OnAxisChanged(name, float64(value))
	}
}

//export func_on_collision_enter
func func_on_collision_enter(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnCollisionEnter != nil {
		callbacks.OnCollisionEnter(int64(selfId), int64(otherId))
	}
}

//export func_on_collision_stay
func func_on_collision_stay(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnCollisionStay != nil {
		callbacks.OnCollisionStay(int64(selfId), int64(otherId))
	}
}

//export func_on_collision_exit
func func_on_collision_exit(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnCollisionExit != nil {
		callbacks.OnCollisionExit(int64(selfId), int64(otherId))
	}
}

//export func_on_trigger_enter
func func_on_trigger_enter(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnTriggerEnter != nil {
		callbacks.OnTriggerEnter(int64(selfId), int64(otherId))
	}
}

//export func_on_trigger_stay
func func_on_trigger_stay(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnTriggerStay != nil {
		callbacks.OnTriggerStay(int64(selfId), int64(otherId))
	}
}

//export func_on_trigger_exit
func func_on_trigger_exit(selfId, otherId C.GDExtensionInt) {
	if callbacks.OnTriggerExit != nil {
		callbacks.OnTriggerExit(int64(selfId), int64(otherId))
	}
}

//export func_on_ui_ready
func func_on_ui_ready(id C.GDExtensionInt) {
	if callbacks.OnUiReady != nil {
		callbacks.OnUiReady(int64(id))
	}
}

//export func_on_ui_updated
func func_on_ui_updated(id C.GDExtensionInt) {
	if callbacks.OnUiUpdated != nil {
		callbacks.OnUiUpdated(int64(id))
	}
}

//export func_on_ui_destroyed
func func_on_ui_destroyed(id C.GDExtensionInt) {
	if callbacks.OnUiDestroyed != nil {
		callbacks.OnUiDestroyed(int64(id))
	}
}

//export func_on_ui_pressed
func func_on_ui_pressed(id C.GDExtensionInt) {
	if callbacks.OnUiPressed != nil {
		callbacks.OnUiPressed(int64(id))
	}
}

//export func_on_ui_released
func func_on_ui_released(id C.GDExtensionInt) {
	if callbacks.OnUiReleased != nil {
		callbacks.OnUiReleased(int64(id))
	}
}

//export func_on_ui_hovered
func func_on_ui_hovered(id C.GDExtensionInt) {
	if callbacks.OnUiHovered != nil {
		callbacks.OnUiHovered(int64(id))
	}
}

//export func_on_ui_clicked
func func_on_ui_clicked(id C.GDExtensionInt) {
	if callbacks.OnUiClicked != nil {
		callbacks.OnUiClicked(int64(id))
	}
}

//export func_on_ui_toggle
func func_on_ui_toggle(id C.GDExtensionInt, isOn C.GDExtensionBool) {
	if callbacks.OnUiToggle != nil {
		callbacks.OnUiToggle(int64(id), bool(isOn != 0))
	}
}

//export func_on_ui_text_changed
func func_on_ui_text_changed(id C.GDExtensionInt, text C.GdString) {
	str := ToString(GdString(text))
	if callbacks.OnUiTextChanged != nil {
		callbacks.OnUiTextChanged(int64(id), str)
	}
}

//export func_on_sprite_screen_entered
func func_on_sprite_screen_entered(id C.GDExtensionInt) {
	if callbacks.OnSpriteScreenEntered != nil {
		callbacks.OnSpriteScreenEntered(int64(id))
	}
}

//export func_on_sprite_screen_exited
func func_on_sprite_screen_exited(id C.GDExtensionInt) {
	if callbacks.OnSpriteScreenExited != nil {
		callbacks.OnSpriteScreenExited(int64(id))
	}
}

//export func_on_sprite_vfx_finished
func func_on_sprite_vfx_finished(id C.GDExtensionInt) {
	if callbacks.OnSpriteVfxFinished != nil {
		callbacks.OnSpriteVfxFinished(int64(id))
	}
}

//export func_on_sprite_animation_finished
func func_on_sprite_animation_finished(id C.GDExtensionInt) {
	if callbacks.OnSpriteAnimationFinished != nil {
		callbacks.OnSpriteAnimationFinished(int64(id))
	}
}

//export func_on_sprite_animation_looped
func func_on_sprite_animation_looped(id C.GDExtensionInt) {
	if callbacks.OnSpriteAnimationLooped != nil {
		callbacks.OnSpriteAnimationLooped(int64(id))
	}
}

//export func_on_sprite_frame_changed
func func_on_sprite_frame_changed(id C.GDExtensionInt) {
	if callbacks.OnSpriteFrameChanged != nil {
		callbacks.OnSpriteFrameChanged(int64(id))
	}
}

//export func_on_sprite_animation_changed
func func_on_sprite_animation_changed(id C.GDExtensionInt) {
	if callbacks.OnSpriteAnimationChanged != nil {
		callbacks.OnSpriteAnimationChanged(int64(id))
	}
}

//export func_on_sprite_frames_set_changed
func func_on_sprite_frames_set_changed(id C.GDExtensionInt) {
	if callbacks.OnSpriteFramesSetChanged != nil {
		callbacks.OnSpriteFramesSetChanged(int64(id))
	}
}
