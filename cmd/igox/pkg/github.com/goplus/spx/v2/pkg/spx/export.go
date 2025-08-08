// export by github.com/goplus/ixgo/cmd/qexp

package spx

import (
	q "github.com/goplus/spx/v2/pkg/spx"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "spx",
		Path: "github.com/goplus/spx/v2/pkg/spx",
		Deps: map[string]string{
			"github.com/goplus/spx/v2/internal/engine": "engine",
			"sync/atomic": "atomic",
			"time":        "time",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{},
		AliasTypes: map[string]reflect.Type{},
		Vars:       map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"Execute":            reflect.ValueOf(q.Execute),
			"ExecuteNative":      reflect.ValueOf(q.ExecuteNative),
			"Go":                 reflect.ValueOf(q.Go),
			"IsAbortThreadError": reflect.ValueOf(q.IsAbortThreadError),
			"IsInCoroutine":      reflect.ValueOf(q.IsInCoroutine),
			"Wait":               reflect.ValueOf(q.Wait),
			"WaitNextFrame":      reflect.ValueOf(q.WaitNextFrame),
		},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
