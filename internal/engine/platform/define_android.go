//go:build android
// +build android

package platform

func GetPlatformType() int {
	return PlatformTypeIos
}
