package platform

const (
	PlatformTypeUnknown = 0
	PlatformTypeWeb     = 1
	PlatformTypeDesktop = 2
	PlatformTypeAndroid = 3
	PlatformTypeIos     = 4
	PlatformTypeServer  = 5
)

func IsWeb() bool {
	return GetPlatformType() == PlatformTypeWeb
}

func IsDesktop() bool {
	return GetPlatformType() == PlatformTypeDesktop
}

func IsAndroid() bool {
	return GetPlatformType() == PlatformTypeAndroid
}

func IsIos() bool {
	return GetPlatformType() == PlatformTypeIos
}

func IsServer() bool {
	return GetPlatformType() == PlatformTypeServer
}
