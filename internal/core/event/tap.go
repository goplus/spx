package event

func If0(enabled func() bool, call func()) func() {
	if call == nil {
		return func() {}
	}
	if enabled == nil {
		return call
	}
	return func() {
		if enabled() {
			call()
		}
	}
}

func If1[T any](enabled func() bool, call func(T)) func(T) {
	if call == nil {
		return func(T) {}
	}
	if enabled == nil {
		return call
	}
	return func(v T) {
		if enabled() {
			call(v)
		}
	}
}

func If2[A, B any](enabled func() bool, call func(A, B)) func(A, B) {
	if call == nil {
		return func(A, B) {}
	}
	if enabled == nil {
		return call
	}
	return func(a A, b B) {
		if enabled() {
			call(a, b)
		}
	}
}

func Ignore1[T any](call func()) func(T) {
	if call == nil {
		return func(T) {}
	}
	return func(T) {
		call()
	}
}

func Ignore2[A, B any](call func()) func(A, B) {
	if call == nil {
		return func(A, B) {}
	}
	return func(A, B) {
		call()
	}
}

func Tap0(call func(), tap func()) func() {
	if tap == nil {
		return call
	}
	if call == nil {
		return func() {
			tap()
		}
	}
	return func() {
		tap()
		call()
	}
}

func Tap1[T any](call func(T), tap func(T)) func(T) {
	if tap == nil {
		return call
	}
	if call == nil {
		return func(v T) {
			tap(v)
		}
	}
	return func(v T) {
		tap(v)
		call(v)
	}
}

func Tap2[A, B any](call func(A, B), tap func(A, B)) func(A, B) {
	if tap == nil {
		return call
	}
	if call == nil {
		return func(a A, b B) {
			tap(a, b)
		}
	}
	return func(a A, b B) {
		tap(a, b)
		call(a, b)
	}
}

func TapVoid1[T any](call func(), tap func(T)) func(T) {
	if tap == nil {
		return func(T) {
			if call != nil {
				call()
			}
		}
	}
	return func(v T) {
		tap(v)
		if call != nil {
			call()
		}
	}
}

func TapVoid2[A, B any](call func(), tap func(A, B)) func(A, B) {
	if tap == nil {
		return func(A, B) {
			if call != nil {
				call()
			}
		}
	}
	return func(a A, b B) {
		tap(a, b)
		if call != nil {
			call()
		}
	}
}
