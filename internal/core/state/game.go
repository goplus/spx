package state

import (
	"sync"
	"time"

	coreevent "github.com/goplus/spx/v2/internal/core/event"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/ui"
)

type GameLifecycleState struct {
	StartFlag      sync.Once
	RunOnce        sync.Once
	OncePathFinder sync.Once
	IsLoaded       bool
	IsRunned       bool
}

type GameDisplayState struct {
	WorldWidth   int
	WorldHeight  int
	MinWorldX    int
	MinWorldY    int
	MapMode      int
	WindowWidth  int
	WindowHeight int
	WindowScale  float64
	StretchMode  bool
}

type GameDialogState struct {
	AskPanel  *ui.UiAsk
	AnswerVal string
}

type GameDebugState struct {
	Debug      bool
	DebugPanel *ui.UiDebug
	DebugInstr bool
	DebugEvent bool
	DebugPerf  bool
}

type GameRuntimeState struct {
	EnabledPhysics   bool
	IsSchedInMain    bool
	MainSchedTime    time.Time
	ImageSizeCache   sync.Map
	EventQueuePolicy coreevent.QueuePolicy
	EventQueueStats  coreevent.QueueStats
}

type GamePathfindingState struct {
	PathCellSizeX int
	PathCellSizeY int
}

type GameAudioState struct {
	AudioAttenuation float64
	AudioMaxDistance float64
	SoundObj         engine.Object
}
