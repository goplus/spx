//go:build js
// +build js

package platform

func GetPlatformType() int {
	return PlatformTypeWeb
}
