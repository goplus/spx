package wrap

import (
	"fmt"
)

type baseMgr struct {
}

func (pself *baseMgr) OnStart() {
}
func (pself *baseMgr) OnUpdate(delta float64) {
}
func (pself *baseMgr) OnFixedUpdate(delta float64) {
}
func (pself *baseMgr) OnDestroy() {
}

func (mgr *baseMgr) logf(format string, v ...any) (n int, err error) {
	return fmt.Printf(format, v...)
}
func (mgr *baseMgr) log(a ...any) (n int, err error) {
	return fmt.Println(a...)
}
