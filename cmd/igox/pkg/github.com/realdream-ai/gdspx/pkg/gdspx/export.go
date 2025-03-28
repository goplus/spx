// export by github.com/goplus/igop/cmd/qexp

package gdspx

import (
	q "github.com/goplus/spx/pkg/gdspx/pkg/gdspx"

	"reflect"

	"github.com/goplus/igop"
)

func init() {
	igop.RegisterPackage(&igop.Package{
		Name: "gdspx",
		Path: "github.com/goplus/spx/pkg/gdspx/pkg/gdspx",
		Deps: map[string]string{
			"github.com/goplus/spx/pkg/gdspx/internal/engine": "engine",
			"github.com/goplus/spx/pkg/gdspx/pkg/engine":      "engine",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{},
		AliasTypes: map[string]reflect.Type{},
		Vars:       map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"IsWebIntepreterMode": reflect.ValueOf(q.IsWebIntepreterMode),
			"LinkEngine":          reflect.ValueOf(q.LinkEngine),
		},
		TypedConsts:   map[string]igop.TypedConst{},
		UntypedConsts: map[string]igop.UntypedConst{},
	})
}
