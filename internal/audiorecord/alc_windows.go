package audiorecord

import (
	"syscall"
	"unsafe"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -systemdll=false -output zalc_windows.go alc_windows.go

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

type ALCdevice struct{}
type ALCcontext struct{}

type Device struct {
	handle *ALCdevice
}

// windows api calls

//sys alcOpenDevice(devicename *byte) (r *ALCdevice) = OpenAL32.alcOpenDevice
//sys alcGetError(device *ALCdevice) (r uint32) = OpenAL32.alcGetError
//sys alcCloseDevice(device *ALCdevice) (r bool) = OpenAL32.alcCloseDevice
//sys alcCreateContext(device *ALCdevice, attrlist *int32) (r *ALCcontext) = OpenAL32.alcCreateContext
//sys alcCaptureOpenDevice(devicename *byte, frequency uint32, format uint32, buffersize int32) (r *ALCdevice) = OpenAL32.alcCaptureOpenDevice
//sys alcCaptureCloseDevice(device *ALCdevice) (r bool) = OpenAL32.alcCaptureCloseDevice
//sys alcCaptureStart(device *ALCdevice) = OpenAL32.alcCaptureStart
//sys alcCaptureStop(device *ALCdevice) = OpenAL32.alcCaptureStop
//sys alcCaptureSamples(device *ALCdevice, buffer unsafe.Pointer, samples int32) = OpenAL32.alcCaptureSamples

// GetError() returns the most recent error generated
// in the AL state machine.
func (self *Device) GetError() uint32 {
	return alcGetError(self.handle)
}

func OpenDevice(name string) *Device {
	h := alcOpenDevice(syscall.StringBytePtr(name))
	return &Device{h}
}

func (self *Device) CloseDevice() bool {
	//TODO: really a method? or not?
	return alcCloseDevice(self.handle)
}

func (self *Device) CreateContext() *Context {
	// TODO: really a method?
	// TODO: attrlist support
	return &Context{alcCreateContext(self.handle, nil)}
}

// func (self *Device) GetIntegerv(param uint32, size uint32) (result []int32) {
// 	result = make([]int32, size)
// 	walcGetIntegerv(self.handle, ALCenum(param), ALCsizei(size), unsafe.Pointer(&result[0]))
// 	return
// }

// func (self *Device) GetInteger(param uint32) int32 {
// 	return int32(walcGetInteger(self.handle, ALCenum(param)))
// }

type CaptureDevice struct {
	Device
	sampleSize uint32
}

func CaptureOpenDevice(name string, freq uint32, format uint32, size uint32) *CaptureDevice {
	// TODO: turn empty string into nil?
	// TODO: what about an error return?
	h := alcCaptureOpenDevice(syscall.StringBytePtr(name), freq, format, int32(size))
	s := map[uint32]uint32{FormatMono8: 1, FormatMono16: 2, FormatStereo8: 2, FormatStereo16: 4}[format]
	return &CaptureDevice{Device{h}, s}
}

// XXX: Override Device.CloseDevice to make sure the correct
// C function is called even if someone decides to use this
// behind an interface.
func (self *CaptureDevice) CloseDevice() bool {
	return alcCaptureCloseDevice(self.handle)
}

func (self *CaptureDevice) CaptureCloseDevice() bool {
	return self.CloseDevice()
}

func (self *CaptureDevice) CaptureStart() {
	alcCaptureStart(self.handle)
}

func (self *CaptureDevice) CaptureStop() {
	alcCaptureStop(self.handle)
}

func (self *CaptureDevice) CaptureSamples(size uint32) (data []byte) {
	data = make([]byte, size*self.sampleSize)
	alcCaptureSamples(self.handle, unsafe.Pointer(&data[0]), int32(size))
	return
}

///// Context ///////////////////////////////////////////////////////

// Context encapsulates the state of a given instance
// of the OpenAL state machine. Only one context can
// be active in a given process.
type Context struct {
	handle *ALCcontext
}

// A context that doesn't exist, useful for certain
// context operations (see OpenAL documentation for
// details).
//var NullContext Context

// Renamed, was MakeContextCurrent.
// func (self *Context) Activate() bool {
// 	return alcMakeContextCurrent(self.handle) != alcFalse
// }

// Renamed, was ProcessContext.
// func (self *Context) Process() {
// 	alcProcessContext(self.handle)
// }

// Renamed, was SuspendContext.
// func (self *Context) Suspend() {
// 	alcSuspendContext(self.handle)
// }

// Renamed, was DestroyContext.
// func (self *Context) Destroy() {
// 	alcDestroyContext(self.handle)
// 	self.handle = nil
// }

// Renamed, was GetContextsDevice.
// func (self *Context) GetDevice() *Device {
// 	return &Device{alcGetContextsDevice(self.handle)}
// }

// Renamed, was GetCurrentContext.
// func CurrentContext() *Context {
// 	return &Context{alcGetCurrentContext()}
// }
