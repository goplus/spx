//go:build !js && !ios && !android
// +build !js,!ios,!android

package platform

func GetPlatformType() int {
	return PlatformTypeDesktop
}
