// export by github.com/goplus/igop/cmd/qexp

package ai

import (
	q "github.com/goplus/builder/tools/ai"

	"go/constant"
	"reflect"

	"github.com/goplus/igop"
)

func init() {
	igop.RegisterPackage(&igop.Package{
		Name: "ai",
		Path: "github.com/goplus/builder/tools/ai",
		Deps: map[string]string{
			"context":      "context",
			"errors":       "errors",
			"fmt":          "fmt",
			"log":          "log",
			"math":         "math",
			"math/rand/v2": "rand",
			"reflect":      "reflect",
			"slices":       "slices",
			"sync":         "sync",
			"time":         "time",
		},
		Interfaces: map[string]reflect.Type{
			"Transport": reflect.TypeOf((*q.Transport)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{
			"CommandParamSpec": reflect.TypeOf((*q.CommandParamSpec)(nil)).Elem(),
			"CommandResult":    reflect.TypeOf((*q.CommandResult)(nil)).Elem(),
			"CommandSpec":      reflect.TypeOf((*q.CommandSpec)(nil)).Elem(),
			"Player":           reflect.TypeOf((*q.Player)(nil)).Elem(),
			"Request":          reflect.TypeOf((*q.Request)(nil)).Elem(),
			"Response":         reflect.TypeOf((*q.Response)(nil)).Elem(),
			"Turn":             reflect.TypeOf((*q.Turn)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars: map[string]reflect.Value{
			"Break":              reflect.ValueOf(&q.Break),
			"ErrTransportNotSet": reflect.ValueOf(&q.ErrTransportNotSet),
		},
		Funcs: map[string]reflect.Value{
			"DefaultKnowledgeBase":    reflect.ValueOf(q.DefaultKnowledgeBase),
			"DefaultTransport":        reflect.ValueOf(q.DefaultTransport),
			"PlayerOnCmd_":            reflect.ValueOf(q.PlayerOnCmd_),
			"SetDefaultKnowledgeBase": reflect.ValueOf(q.SetDefaultKnowledgeBase),
			"SetDefaultTransport":     reflect.ValueOf(q.SetDefaultTransport),
		},
		TypedConsts: map[string]igop.TypedConst{},
		UntypedConsts: map[string]igop.UntypedConst{
			"GopPackage": {"untyped bool", constant.MakeBool(bool(q.GopPackage))},
		},
	})
}
