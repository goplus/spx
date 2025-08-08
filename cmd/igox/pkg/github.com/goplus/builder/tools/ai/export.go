// export by github.com/goplus/ixgo/cmd/qexp

package ai

import (
	q "github.com/goplus/builder/tools/ai"

	"go/constant"
	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "ai",
		Path: "github.com/goplus/builder/tools/ai",
		Deps: map[string]string{
			"context":                          "context",
			"errors":                           "errors",
			"fmt":                              "fmt",
			"github.com/goplus/spx/v2/pkg/spx": "spx",
			"log":                              "log",
			"math":                             "math",
			"math/rand/v2":                     "rand",
			"reflect":                          "reflect",
			"slices":                           "slices",
			"sync":                             "sync",
			"time":                             "time",
		},
		Interfaces: map[string]reflect.Type{
			"Transport": reflect.TypeOf((*q.Transport)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{
			"ArchivedHistory":  reflect.TypeOf((*q.ArchivedHistory)(nil)).Elem(),
			"CommandParamSpec": reflect.TypeOf((*q.CommandParamSpec)(nil)).Elem(),
			"CommandResult":    reflect.TypeOf((*q.CommandResult)(nil)).Elem(),
			"CommandSpec":      reflect.TypeOf((*q.CommandSpec)(nil)).Elem(),
			"Player":           reflect.TypeOf((*q.Player)(nil)).Elem(),
			"Request":          reflect.TypeOf((*q.Request)(nil)).Elem(),
			"Response":         reflect.TypeOf((*q.Response)(nil)).Elem(),
			"TaskRunner":       reflect.TypeOf((*q.TaskRunner)(nil)).Elem(),
			"Turn":             reflect.TypeOf((*q.Turn)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars: map[string]reflect.Value{
			"Break":              reflect.ValueOf(&q.Break),
			"ErrTransportNotSet": reflect.ValueOf(&q.ErrTransportNotSet),
		},
		Funcs: map[string]reflect.Value{
			"DefaultKnowledgeBase":    reflect.ValueOf(q.DefaultKnowledgeBase),
			"DefaultTaskRunner":       reflect.ValueOf(q.DefaultTaskRunner),
			"DefaultTransport":        reflect.ValueOf(q.DefaultTransport),
			"PlayerOnCmd_":            reflect.ValueOf(q.PlayerOnCmd_),
			"SetDefaultKnowledgeBase": reflect.ValueOf(q.SetDefaultKnowledgeBase),
			"SetDefaultTaskRunner":    reflect.ValueOf(q.SetDefaultTaskRunner),
			"SetDefaultTransport":     reflect.ValueOf(q.SetDefaultTransport),
		},
		TypedConsts: map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{
			"GopPackage": {"untyped bool", constant.MakeBool(bool(q.GopPackage))},
		},
	})
}
