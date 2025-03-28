package engine

var (
	KeyCode KeyCodeEnum
)

type KeyCodeEnum struct {
	None         int64
	Special      int64
	Escape       int64
	Tab          int64
	Backtab      int64
	Backspace    int64
	Enter        int64
	KPEnter      int64
	Insert       int64
	Delete       int64
	Pause        int64
	Print        int64
	SysReq       int64
	Clear        int64
	Home         int64
	End          int64
	Left         int64
	Up           int64
	Right        int64
	Down         int64
	PageUp       int64
	PageDown     int64
	Shift        int64
	Ctrl         int64
	Meta         int64
	CmdOrCtrl    int64
	Alt          int64
	CapsLock     int64
	NumLock      int64
	ScrollLock   int64
	F1           int64
	F2           int64
	F3           int64
	F4           int64
	F5           int64
	F6           int64
	F7           int64
	F8           int64
	F9           int64
	F10          int64
	F11          int64
	F12          int64
	F13          int64
	F14          int64
	F15          int64
	F16          int64
	F17          int64
	F18          int64
	F19          int64
	F20          int64
	F21          int64
	F22          int64
	F23          int64
	F24          int64
	F25          int64
	F26          int64
	F27          int64
	F28          int64
	F29          int64
	F30          int64
	F31          int64
	F32          int64
	F33          int64
	F34          int64
	F35          int64
	KPMultiply   int64
	KPDivide     int64
	KPSubtract   int64
	KPPeriod     int64
	KPAdd        int64
	KP0          int64
	KP1          int64
	KP2          int64
	KP3          int64
	KP4          int64
	KP5          int64
	KP6          int64
	KP7          int64
	KP8          int64
	KP9          int64
	Menu         int64
	Hyper        int64
	Help         int64
	Back         int64
	Forward      int64
	Stop         int64
	Refresh      int64
	VolumeDown   int64
	VolumeMute   int64
	VolumeUp     int64
	MediaPlay    int64
	MediaStop    int64
	MediaPrev    int64
	MediaNext    int64
	MediaRecord  int64
	HomePage     int64
	Favorites    int64
	Search       int64
	Standby      int64
	OpenURL      int64
	LaunchMail   int64
	LaunchMedia  int64
	Launch0      int64
	Launch1      int64
	Launch2      int64
	Launch3      int64
	Launch4      int64
	Launch5      int64
	Launch6      int64
	Launch7      int64
	Launch8      int64
	Launch9      int64
	LaunchA      int64
	LaunchB      int64
	LaunchC      int64
	LaunchD      int64
	LaunchE      int64
	LaunchF      int64
	Globe        int64
	Keyboard     int64
	JISEisu      int64
	JISKana      int64
	Unknown      int64
	Space        int64
	Exclam       int64
	QuoteDbl     int64
	NumberSign   int64
	Dollar       int64
	Percent      int64
	Ampersand    int64
	Apostrophe   int64
	ParenLeft    int64
	ParenRight   int64
	Asterisk     int64
	Plus         int64
	Comma        int64
	Minus        int64
	Period       int64
	Slash        int64
	Key0         int64
	Key1         int64
	Key2         int64
	Key3         int64
	Key4         int64
	Key5         int64
	Key6         int64
	Key7         int64
	Key8         int64
	Key9         int64
	Colon        int64
	Semicolon    int64
	Less         int64
	Equal        int64
	Greater      int64
	Question     int64
	At           int64
	A            int64
	B            int64
	C            int64
	D            int64
	E            int64
	F            int64
	G            int64
	H            int64
	I            int64
	J            int64
	K            int64
	L            int64
	M            int64
	N            int64
	O            int64
	P            int64
	Q            int64
	R            int64
	S            int64
	T            int64
	U            int64
	V            int64
	W            int64
	X            int64
	Y            int64
	Z            int64
	BracketLeft  int64
	Backslash    int64
	BracketRight int64
	AsciiCircum  int64
	Underscore   int64
	QuoteLeft    int64
	BraceLeft    int64
	Bar          int64
	BraceRight   int64
	AsciiTilde   int64
	Yen          int64
	Section      int64
}

func initKeyCode() {
	KeyCode.None = 0
	KeyCode.Special = 1 << 22
	KeyCode.Escape = KeyCode.Special | 0x01
	KeyCode.Tab = KeyCode.Special | 0x02
	KeyCode.Backtab = KeyCode.Special | 0x03
	KeyCode.Backspace = KeyCode.Special | 0x04
	KeyCode.Enter = KeyCode.Special | 0x05
	KeyCode.KPEnter = KeyCode.Special | 0x06
	KeyCode.Insert = KeyCode.Special | 0x07
	KeyCode.Delete = KeyCode.Special | 0x08
	KeyCode.Pause = KeyCode.Special | 0x09
	KeyCode.Print = KeyCode.Special | 0x0A
	KeyCode.SysReq = KeyCode.Special | 0x0B
	KeyCode.Clear = KeyCode.Special | 0x0C
	KeyCode.Home = KeyCode.Special | 0x0D
	KeyCode.End = KeyCode.Special | 0x0E
	KeyCode.Left = KeyCode.Special | 0x0F
	KeyCode.Up = KeyCode.Special | 0x10
	KeyCode.Right = KeyCode.Special | 0x11
	KeyCode.Down = KeyCode.Special | 0x12
	KeyCode.PageUp = KeyCode.Special | 0x13
	KeyCode.PageDown = KeyCode.Special | 0x14
	KeyCode.Shift = KeyCode.Special | 0x15
	KeyCode.Ctrl = KeyCode.Special | 0x16
	KeyCode.Meta = KeyCode.Special | 0x17
	KeyCode.CmdOrCtrl = KeyCode.Meta // 或者 KeyCode.Ctrl
	KeyCode.Alt = KeyCode.Special | 0x18
	KeyCode.CapsLock = KeyCode.Special | 0x19
	KeyCode.NumLock = KeyCode.Special | 0x1A
	KeyCode.ScrollLock = KeyCode.Special | 0x1B
	KeyCode.F1 = KeyCode.Special | 0x1C
	KeyCode.F2 = KeyCode.Special | 0x1D
	KeyCode.F3 = KeyCode.Special | 0x1E
	KeyCode.F4 = KeyCode.Special | 0x1F
	KeyCode.F5 = KeyCode.Special | 0x20
	KeyCode.F6 = KeyCode.Special | 0x21
	KeyCode.F7 = KeyCode.Special | 0x22
	KeyCode.F8 = KeyCode.Special | 0x23
	KeyCode.F9 = KeyCode.Special | 0x24
	KeyCode.F10 = KeyCode.Special | 0x25
	KeyCode.F11 = KeyCode.Special | 0x26
	KeyCode.F12 = KeyCode.Special | 0x27
	KeyCode.F13 = KeyCode.Special | 0x28
	KeyCode.F14 = KeyCode.Special | 0x29
	KeyCode.F15 = KeyCode.Special | 0x2A
	KeyCode.F16 = KeyCode.Special | 0x2B
	KeyCode.F17 = KeyCode.Special | 0x2C
	KeyCode.F18 = KeyCode.Special | 0x2D
	KeyCode.F19 = KeyCode.Special | 0x2E
	KeyCode.F20 = KeyCode.Special | 0x2F
	KeyCode.F21 = KeyCode.Special | 0x30
	KeyCode.F22 = KeyCode.Special | 0x31
	KeyCode.F23 = KeyCode.Special | 0x32
	KeyCode.F24 = KeyCode.Special | 0x33
	KeyCode.F25 = KeyCode.Special | 0x34
	KeyCode.F26 = KeyCode.Special | 0x35
	KeyCode.F27 = KeyCode.Special | 0x36
	KeyCode.F28 = KeyCode.Special | 0x37
	KeyCode.F29 = KeyCode.Special | 0x38
	KeyCode.F30 = KeyCode.Special | 0x39
	KeyCode.F31 = KeyCode.Special | 0x3A
	KeyCode.F32 = KeyCode.Special | 0x3B
	KeyCode.F33 = KeyCode.Special | 0x3C
	KeyCode.F34 = KeyCode.Special | 0x3D
	KeyCode.F35 = KeyCode.Special | 0x3E
	KeyCode.KPMultiply = KeyCode.Special | 0x81
	KeyCode.KPDivide = KeyCode.Special | 0x82
	KeyCode.KPSubtract = KeyCode.Special | 0x83
	KeyCode.KPPeriod = KeyCode.Special | 0x84
	KeyCode.KPAdd = KeyCode.Special | 0x85
	KeyCode.KP0 = KeyCode.Special | 0x86
	KeyCode.KP1 = KeyCode.Special | 0x87
	KeyCode.KP2 = KeyCode.Special | 0x88
	KeyCode.KP3 = KeyCode.Special | 0x89
	KeyCode.KP4 = KeyCode.Special | 0x8A
	KeyCode.KP5 = KeyCode.Special | 0x8B
	KeyCode.KP6 = KeyCode.Special | 0x8C
	KeyCode.KP7 = KeyCode.Special | 0x8D
	KeyCode.KP8 = KeyCode.Special | 0x8E
	KeyCode.KP9 = KeyCode.Special | 0x8F
	KeyCode.Menu = KeyCode.Special | 0x42
	KeyCode.Hyper = KeyCode.Special | 0x43
	KeyCode.Help = KeyCode.Special | 0x45
	KeyCode.Back = KeyCode.Special | 0x48
	KeyCode.Forward = KeyCode.Special | 0x49
	KeyCode.Stop = KeyCode.Special | 0x4A
	KeyCode.Refresh = KeyCode.Special | 0x4B
	KeyCode.VolumeDown = KeyCode.Special | 0x4C
	KeyCode.VolumeMute = KeyCode.Special | 0x4D
	KeyCode.VolumeUp = KeyCode.Special | 0x4E
	KeyCode.MediaPlay = KeyCode.Special | 0x54
	KeyCode.MediaStop = KeyCode.Special | 0x55
	KeyCode.MediaPrev = KeyCode.Special | 0x56
	KeyCode.MediaNext = KeyCode.Special | 0x57
	KeyCode.MediaRecord = KeyCode.Special | 0x58
	KeyCode.HomePage = KeyCode.Special | 0x59
	KeyCode.Favorites = KeyCode.Special | 0x5A
	KeyCode.Search = KeyCode.Special | 0x5B
	KeyCode.Standby = KeyCode.Special | 0x5C
	KeyCode.OpenURL = KeyCode.Special | 0x5D
	KeyCode.LaunchMail = KeyCode.Special | 0x5E
	KeyCode.LaunchMedia = KeyCode.Special | 0x5F
	KeyCode.Launch0 = KeyCode.Special | 0x60
	KeyCode.Launch1 = KeyCode.Special | 0x61
	KeyCode.Launch2 = KeyCode.Special | 0x62
	KeyCode.Launch3 = KeyCode.Special | 0x63
	KeyCode.Launch4 = KeyCode.Special | 0x64
	KeyCode.Launch5 = KeyCode.Special | 0x65
	KeyCode.Launch6 = KeyCode.Special | 0x66
	KeyCode.Launch7 = KeyCode.Special | 0x67
	KeyCode.Launch8 = KeyCode.Special | 0x68
	KeyCode.Launch9 = KeyCode.Special | 0x69
	KeyCode.LaunchA = KeyCode.Special | 0x6A
	KeyCode.LaunchB = KeyCode.Special | 0x6B
	KeyCode.LaunchC = KeyCode.Special | 0x6C
	KeyCode.LaunchD = KeyCode.Special | 0x6D
	KeyCode.LaunchE = KeyCode.Special | 0x6E
	KeyCode.LaunchF = KeyCode.Special | 0x6F
	KeyCode.Globe = KeyCode.Special | 0x70
	KeyCode.Keyboard = KeyCode.Special | 0x71
	KeyCode.JISEisu = KeyCode.Special | 0x72
	KeyCode.JISKana = KeyCode.Special | 0x73
	KeyCode.Unknown = KeyCode.Special | 0x7FFFFF

	// 可打印字符的键值
	KeyCode.Space = 0x0020
	KeyCode.Exclam = 0x0021
	KeyCode.QuoteDbl = 0x0022
	KeyCode.NumberSign = 0x0023
	KeyCode.Dollar = 0x0024
	KeyCode.Percent = 0x0025
	KeyCode.Ampersand = 0x0026
	KeyCode.Apostrophe = 0x0027
	KeyCode.ParenLeft = 0x0028
	KeyCode.ParenRight = 0x0029
	KeyCode.Asterisk = 0x002A
	KeyCode.Plus = 0x002B
	KeyCode.Comma = 0x002C
	KeyCode.Minus = 0x002D
	KeyCode.Period = 0x002E
	KeyCode.Slash = 0x002F
	KeyCode.Key0 = 0x0030
	KeyCode.Key1 = 0x0031
	KeyCode.Key2 = 0x0032
	KeyCode.Key3 = 0x0033
	KeyCode.Key4 = 0x0034
	KeyCode.Key5 = 0x0035
	KeyCode.Key6 = 0x0036
	KeyCode.Key7 = 0x0037
	KeyCode.Key8 = 0x0038
	KeyCode.Key9 = 0x0039
	KeyCode.Colon = 0x003A
	KeyCode.Semicolon = 0x003B
	KeyCode.Less = 0x003C
	KeyCode.Equal = 0x003D
	KeyCode.Greater = 0x003E
	KeyCode.Question = 0x003F
	KeyCode.At = 0x0040
	KeyCode.A = 0x0041
	KeyCode.B = 0x0042
	KeyCode.C = 0x0043
	KeyCode.D = 0x0044
	KeyCode.E = 0x0045
	KeyCode.F = 0x0046
	KeyCode.G = 0x0047
	KeyCode.H = 0x0048
	KeyCode.I = 0x0049
	KeyCode.J = 0x004A
	KeyCode.K = 0x004B
	KeyCode.L = 0x004C
	KeyCode.M = 0x004D
	KeyCode.N = 0x004E
	KeyCode.O = 0x004F
	KeyCode.P = 0x0050
	KeyCode.Q = 0x0051
	KeyCode.R = 0x0052
	KeyCode.S = 0x0053
	KeyCode.T = 0x0054
	KeyCode.U = 0x0055
	KeyCode.V = 0x0056
	KeyCode.W = 0x0057
	KeyCode.X = 0x0058
	KeyCode.Y = 0x0059
	KeyCode.Z = 0x005A
	KeyCode.BracketLeft = 0x005B
	KeyCode.Backslash = 0x005C
	KeyCode.BracketRight = 0x005D
	KeyCode.AsciiCircum = 0x005E
	KeyCode.Underscore = 0x005F
	KeyCode.QuoteLeft = 0x0060
	KeyCode.BraceLeft = 0x007B
	KeyCode.Bar = 0x007C
	KeyCode.BraceRight = 0x007D
	KeyCode.AsciiTilde = 0x007E
	KeyCode.Yen = 0x00A5
	KeyCode.Section = 0x00A7
}
