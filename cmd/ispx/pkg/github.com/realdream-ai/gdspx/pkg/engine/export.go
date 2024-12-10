// export by github.com/goplus/igop/cmd/qexp

package engine

import (
	q "github.com/realdream-ai/gdspx/pkg/engine"

	"reflect"

	"github.com/goplus/igop"
)

func init() {
	igop.RegisterPackage(&igop.Package{
		Name: "engine",
		Path: "github.com/realdream-ai/gdspx/pkg/engine",
		Deps: map[string]string{
			"fmt":                           "fmt",
			"github.com/realdream-ai/mathf": "mathf",
			"math":                          "math",
			"reflect":                       "reflect",
			"runtime/debug":                 "debug",
			"sort":                          "sort",
			"sync":                          "sync",
		},
		Interfaces: map[string]reflect.Type{
			"IAudioMgr":    reflect.TypeOf((*q.IAudioMgr)(nil)).Elem(),
			"ICameraMgr":   reflect.TypeOf((*q.ICameraMgr)(nil)).Elem(),
			"IInputMgr":    reflect.TypeOf((*q.IInputMgr)(nil)).Elem(),
			"ILifeCycle":   reflect.TypeOf((*q.ILifeCycle)(nil)).Elem(),
			"IManager":     reflect.TypeOf((*q.IManager)(nil)).Elem(),
			"IPhysicMgr":   reflect.TypeOf((*q.IPhysicMgr)(nil)).Elem(),
			"IPlatformMgr": reflect.TypeOf((*q.IPlatformMgr)(nil)).Elem(),
			"IResMgr":      reflect.TypeOf((*q.IResMgr)(nil)).Elem(),
			"ISceneMgr":    reflect.TypeOf((*q.ISceneMgr)(nil)).Elem(),
			"ISpriteMgr":   reflect.TypeOf((*q.ISpriteMgr)(nil)).Elem(),
			"ISpriter":     reflect.TypeOf((*q.ISpriter)(nil)).Elem(),
			"IUiMgr":       reflect.TypeOf((*q.IUiMgr)(nil)).Elem(),
			"IUiNode":      reflect.TypeOf((*q.IUiNode)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{
			"Action0":            reflect.TypeOf((*q.Action0)(nil)).Elem(),
			"CallbackInfo":       reflect.TypeOf((*q.CallbackInfo)(nil)).Elem(),
			"EngineCallbackInfo": reflect.TypeOf((*q.EngineCallbackInfo)(nil)).Elem(),
			"Event0":             reflect.TypeOf((*q.Event0)(nil)).Elem(),
			"KeyCodeEnum":        reflect.TypeOf((*q.KeyCodeEnum)(nil)).Elem(),
			"Node":               reflect.TypeOf((*q.Node)(nil)).Elem(),
			"Object":             reflect.TypeOf((*q.Object)(nil)).Elem(),
			"Sprite":             reflect.TypeOf((*q.Sprite)(nil)).Elem(),
			"UiNode":             reflect.TypeOf((*q.UiNode)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars: map[string]reflect.Value{
			"AudioMgr":           reflect.ValueOf(&q.AudioMgr),
			"CameraMgr":          reflect.ValueOf(&q.CameraMgr),
			"Id2Sprites":         reflect.ValueOf(&q.Id2Sprites),
			"Id2UiNodes":         reflect.ValueOf(&q.Id2UiNodes),
			"InputMgr":           reflect.ValueOf(&q.InputMgr),
			"KeyCode":            reflect.ValueOf(&q.KeyCode),
			"Math_PI":            reflect.ValueOf(&q.Math_PI),
			"PhysicMgr":          reflect.ValueOf(&q.PhysicMgr),
			"PlatformMgr":        reflect.ValueOf(&q.PlatformMgr),
			"ResMgr":             reflect.ValueOf(&q.ResMgr),
			"SceneMgr":           reflect.ValueOf(&q.SceneMgr),
			"SpriteMgr":          reflect.ValueOf(&q.SpriteMgr),
			"TimeSinceGameStart": reflect.ValueOf(&q.TimeSinceGameStart),
			"UiMgr":              reflect.ValueOf(&q.UiMgr),
		},
		Funcs: map[string]reflect.Value{
			"Abs":                         reflect.ValueOf(q.Abs),
			"Angle":                       reflect.ValueOf(q.Angle),
			"AngleToPoint":                reflect.ValueOf(q.AngleToPoint),
			"BindSceneInstantiatedSprite": reflect.ValueOf(q.BindSceneInstantiatedSprite),
			"ClearAllSprites":             reflect.ValueOf(q.ClearAllSprites),
			"DealySpriteCall":             reflect.ValueOf(q.DealySpriteCall),
			"DegToRad":                    reflect.ValueOf(q.DegToRad),
			"DelayCall":                   reflect.ValueOf(q.DelayCall),
			"GetSprite":                   reflect.ValueOf(q.GetSprite),
			"InternalInitEngine":          reflect.ValueOf(q.InternalInitEngine),
			"InternalUpdateEngine":        reflect.ValueOf(q.InternalUpdateEngine),
			"NewEvent0":                   reflect.ValueOf(q.NewEvent0),
			"PrintStack":                  reflect.ValueOf(q.PrintStack),
			"RadToDeg":                    reflect.ValueOf(q.RadToDeg),
			"Sign":                        reflect.ValueOf(q.Sign),
			"TweenPos":                    reflect.ValueOf(q.TweenPos),
			"TweenPos2":                   reflect.ValueOf(q.TweenPos2),
		},
		TypedConsts:   map[string]igop.TypedConst{},
		UntypedConsts: map[string]igop.UntypedConst{},
	})
}
