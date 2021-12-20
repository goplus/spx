package audiorecord

/*
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include <jni.h>
#include <limits.h>
#include <AL/al.h>
#include <AL/alc.h>

void al_init(uintptr_t java_vm, uintptr_t jni_env, jobject context, void** handle) {
  JavaVM* vm = (JavaVM*)java_vm;
  JNIEnv* env = (JNIEnv*)jni_env;

  jclass android_content_Context = (*env)->FindClass(env, "android/content/Context");
  jmethodID get_application_info = (*env)->GetMethodID(env, android_content_Context, "getApplicationInfo", "()Landroid/content/pm/ApplicationInfo;");
  jclass android_content_pm_ApplicationInfo = (*env)->FindClass(env, "android/content/pm/ApplicationInfo");
  jfieldID native_library_dir = (*env)->GetFieldID(env, android_content_pm_ApplicationInfo, "nativeLibraryDir", "Ljava/lang/String;");
  jobject app_info = (*env)->CallObjectMethod(env, context, get_application_info);
  jstring native_dir = (*env)->GetObjectField(env, app_info, native_library_dir);
  const char *cnative_dir = (*env)->GetStringUTFChars(env, native_dir, 0);

  char lib_path[PATH_MAX] = "";
  strlcat(lib_path, cnative_dir, sizeof(lib_path));
  strlcat(lib_path, "/libopenal.so", sizeof(lib_path));
  *handle = dlopen(lib_path, RTLD_LAZY);
  (*env)->ReleaseStringUTFChars(env, native_dir, cnative_dir);
}
#include "wrappers.h"
*/
import "C"
import (
	"unsafe"
)

// Format of sound samples passed to Buffer.SetData().
const (
	FormatMono8    = 0x1100
	FormatMono16   = 0x1101
	FormatStereo8  = 0x1102
	FormatStereo16 = 0x1103
)

const (
	alcFalse = 0
	alcTrue  = 1
)

// Error codes returned by Device.GetError().
const (
	NoError        = 0
	InvalidDevice  = 0xA001
	InvalidContext = 0xA002
	InvalidEnum    = 0xA003
	InvalidValue   = 0xA004
	OutOfMemory    = 0xA005
)

const (
	Frequency     = 0x1007 // int Hz
	Refresh       = 0x1008 // int Hz
	Sync          = 0x1009 // bool
	MonoSources   = 0x1010 // int
	StereoSources = 0x1011 // int
)

// The Specifier string for default device?
const (
	DefaultDeviceSpecifier = 0x1004
	DeviceSpecifier        = 0x1005
	Extensions             = 0x1006
)

// ?
const (
	MajorVersion = 0x1000
	MinorVersion = 0x1001
)

// ?
const (
	AttributesSize = 0x1002
	AllAttributes  = 0x1003
)

// Capture extension
const (
	CaptureDeviceSpecifier        = 0x310
	CaptureDefaultDeviceSpecifier = 0x311
	CaptureSamples                = 0x312
)

func init() {
	err := mobileinit.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.al_init(C.uintptr_t(vm), C.uintptr_t(env), C.jobject(ctx), &alHandle)
		if alHandle == nil {
			return errors.New("al: cannot load libopenal.so")
		}
		return nil
	})
	if err != nil {
		log.Fatalf("al: %v", err)
	}

}

type Device struct {
	handle *C.ALCdevice
}

// GetError() returns the most recent error generated
// in the AL state machine.
func (self *Device) GetError() uint32 {
	return uint32(C.alcGetError(self.handle))
}

func OpenDevice(name string) *Device {
	// TODO: turn empty string into nil?
	// TODO: what about an error return?
	p := C.CString(name)
	h := C.walcOpenDevice(p)
	C.free(unsafe.Pointer(p))
	return &Device{h}
}

func (self *Device) CloseDevice() bool {
	//TODO: really a method? or not?
	return C.alcCloseDevice(self.handle) != 0
}

func (self *Device) CreateContext() *Context {
	// TODO: really a method?
	// TODO: attrlist support
	return &Context{C.alcCreateContext(self.handle, nil)}
}

func (self *Device) GetIntegerv(param uint32, size uint32) (result []int32) {
	result = make([]int32, size)
	C.walcGetIntegerv(self.handle, C.ALCenum(param), C.ALCsizei(size), unsafe.Pointer(&result[0]))
	return
}

func (self *Device) GetInteger(param uint32) int32 {
	return int32(C.walcGetInteger(self.handle, C.ALCenum(param)))
}

type CaptureDevice struct {
	Device
	sampleSize uint32
}

func CaptureOpenDevice(name string, freq uint32, format uint32, size uint32) *CaptureDevice {
	// TODO: turn empty string into nil?
	// TODO: what about an error return?
	p := C.CString(name)
	h := C.walcCaptureOpenDevice(p, C.ALCuint(freq), C.ALCenum(format), C.ALCsizei(size))
	C.free(unsafe.Pointer(p))
	s := map[uint32]uint32{FormatMono8: 1, FormatMono16: 2, FormatStereo8: 2, FormatStereo16: 4}[format]
	return &CaptureDevice{Device{h}, s}
}

// XXX: Override Device.CloseDevice to make sure the correct
// C function is called even if someone decides to use this
// behind an interface.
func (self *CaptureDevice) CloseDevice() bool {
	return C.alcCaptureCloseDevice(self.handle) != 0
}

func (self *CaptureDevice) CaptureCloseDevice() bool {
	return self.CloseDevice()
}

func (self *CaptureDevice) CaptureStart() {
	C.alcCaptureStart(self.handle)
}

func (self *CaptureDevice) CaptureStop() {
	C.alcCaptureStop(self.handle)
}

func (self *CaptureDevice) CaptureSamples(size uint32) (data []byte) {
	data = make([]byte, size*self.sampleSize)
	C.alcCaptureSamples(self.handle, unsafe.Pointer(&data[0]), C.ALCsizei(size))
	return
}

///// Context ///////////////////////////////////////////////////////

// Context encapsulates the state of a given instance
// of the OpenAL state machine. Only one context can
// be active in a given process.
type Context struct {
	handle *C.ALCcontext
}

// A context that doesn't exist, useful for certain
// context operations (see OpenAL documentation for
// details).
var NullContext Context

// Renamed, was MakeContextCurrent.
func (self *Context) Activate() bool {
	return C.alcMakeContextCurrent(self.handle) != alcFalse
}

// Renamed, was ProcessContext.
func (self *Context) Process() {
	C.alcProcessContext(self.handle)
}

// Renamed, was SuspendContext.
func (self *Context) Suspend() {
	C.alcSuspendContext(self.handle)
}

// Renamed, was DestroyContext.
func (self *Context) Destroy() {
	C.alcDestroyContext(self.handle)
	self.handle = nil
}

// Renamed, was GetContextsDevice.
func (self *Context) GetDevice() *Device {
	return &Device{C.alcGetContextsDevice(self.handle)}
}

// Renamed, was GetCurrentContext.
func CurrentContext() *Context {
	return &Context{C.alcGetCurrentContext()}
}
