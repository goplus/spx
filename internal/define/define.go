package define

var isWebIntepreterMode bool
var IsMainThread bool
var HasInit bool

func Init(isWeb bool) {
	isWebIntepreterMode = isWeb
}

func IsWebMode() bool {
	return isWebIntepreterMode
}

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}
