package engine

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"
)

// =============== factory ===================
func SyncCreateUiNode[T any](path string) *T {
	var __ret *T
	done := make(chan struct{})
	job := func() {
		__ret = CreateUI[T](path)
		done <- struct{}{}
	}
	updateJobQueue <- job
	<-done
	return __ret
}
func SyncCreateEngineUiNode[T any](path string) *T {
	var __ret *T
	done := make(chan struct{})
	job := func() {
		__ret = CreateEngineUI[T](path)
		done <- struct{}{}
	}
	updateJobQueue <- job
	<-done
	return __ret
}

func SyncCreateSprite[T any]() *T {
	var __ret *T
	done := make(chan struct{})
	job := func() {
		__ret = CreateSprite[T]()
		done <- struct{}{}
	}
	updateJobQueue <- job
	<-done
	return __ret
}

func SyncCreateEmptySprite[T any]() *T {
	var __ret *T
	done := make(chan struct{})
	job := func() {
		__ret = CreateEmptySprite[T]()
		done <- struct{}{}
	}
	updateJobQueue <- job
	<-done
	return __ret
}

func SyncNewBackdropProxy(obj interface{}, path string, renderScale float64) *ProxySprite {
	var __ret *ProxySprite
	done := make(chan struct{})
	job := func() {
		__ret = newBackdropProxy(obj, path, renderScale)
		done <- struct{}{}
	}
	updateJobQueue <- job
	<-done
	return __ret
}

func newBackdropProxy(obj interface{}, path string, renderScale float64) *ProxySprite {
	__ret := CreateEmptySprite[ProxySprite]()
	__ret.Target = obj
	__ret.SetZIndex(-1)
	__ret.DisablePhysic()
	__ret.UpdateTexture(path, renderScale)
	return __ret
}

// =============== input ===================
func SyncInputMousePressed() bool {
	return SyncInputGetMouseState(0) || SyncInputGetMouseState(1)
}

// =============== time ===================
func SyncGetCurrentTPS() float64 {
	return 30 // TODO(tanjp) use engine api
}

// =============== window ===================
func SyncSetRunnableOnUnfocused(flag bool) {
	println("TODO tanjp SyncSetRunnableOnUnfocused")
}

func SyncReadAllText(path string) string {
	return SyncResReadAllText(path)
}

// =============== setting ===================

func SyncSetDebugMode(isDebug bool) {
	SyncPlatformSetDebugMode(isDebug)
}

// =============== setting ===================
func SyncGetCameraLocalPosition(x, y float64) (float64, float64) {
	posX, posY := SyncGetCameraPosition()
	x -= posX
	y -= posY
	return x, y
}
func SyncGetCameraPosition() (float64, float64) {
	pos := SyncCameraGetCameraPosition()
	return float64(pos.X), -float64(pos.Y)
}
func SyncSetCameraPosition(x, y float64) {
	SyncCameraSetCameraPosition(NewVec2(float64(x), -float64(y)))
}
