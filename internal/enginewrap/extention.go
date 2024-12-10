package enginewrap

var mainCallback func(call func())

func Init(call func(f func())) {
	mainCallback = call
}

func callInMainThread(call func()) {
	mainCallback(call)
}

// =============== input ===================
func (pself *inputMgrImpl) MousePressed() bool {
	return inputMgr.GetMouseState(0) || inputMgr.GetMouseState(1)
}

// =============== window ===================

func (pself *platformMgrImpl) SetRunnableOnUnfocused(flag bool) {
	if !flag {
		println("TODO tanjp SetRunnableOnUnfocused")
	}
}
