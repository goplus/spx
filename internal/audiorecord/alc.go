package audiorecord

// TODO(tanjp): implement this

// Format of sound samples passed to Buffer.SetData().
const (
	FormatMono8    = 0x1100
	FormatMono16   = 0x1101
	FormatStereo8  = 0x1102
	FormatStereo16 = 0x1103
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

type Device struct {
}

// GetError() returns the most recent error generated
// in the AL state machine.
func (pself *Device) GetError() uint32 {
	return 0
}

func OpenDevice(name string) *Device {
	return &Device{}
}

func (pself *Device) CloseDevice() bool {
	return true
}

func (pself *Device) CreateContext() *Context {
	return &Context{}
}

func (pself *Device) GetIntegerv(param uint32, size uint32) (result []int32) {
	result = make([]int32, size)
	return
}

func (pself *Device) GetInteger(param uint32) int32 {
	return 0
}

type CaptureDevice struct {
	Device
}

func CaptureOpenDevice(name string, freq uint32, format uint32, size uint32) *CaptureDevice {

	return &CaptureDevice{}
}

// Override Device.CloseDevice to make sure the correct
// C function is called even if someone decides to use this
// behind an interface.
func (pself *CaptureDevice) CloseDevice() bool {
	return true
}

func (pself *CaptureDevice) CaptureCloseDevice() bool {
	return true
}

func (pself *CaptureDevice) CaptureStart() {
}

func (pself *CaptureDevice) CaptureStop() {
}

func (pself *CaptureDevice) CaptureSamples(size uint32) (data []byte) {
	return
}

///// Context ///////////////////////////////////////////////////////

// Context encapsulates the state of a given instance
// of the OpenAL state machine. Only one context can
// be active in a given process.
type Context struct {
}

// A context that doesn't exist, useful for certain
// context operations (see OpenAL documentation for
// details).
var NullContext Context

// Renamed, was MakeContextCurrent.
func (pself *Context) Activate() bool {
	return true
}

// Renamed, was ProcessContext.
func (pself *Context) Process() {
}

// Renamed, was SuspendContext.
func (pself *Context) Suspend() {
}

// Renamed, was DestroyContext.
func (pself *Context) Destroy() {
}

// Renamed, was GetContextsDevice.
func (pself *Context) GetDevice() *Device {
	return &Device{}
}

// Renamed, was GetCurrentContext.
func CurrentContext() *Context {
	return &Context{}
}
