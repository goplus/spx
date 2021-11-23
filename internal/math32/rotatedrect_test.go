package math32_test

import (
	"fmt"
	"testing"

	"github.com/goplus/spx/internal/math32"
)

func TestRotatedRect(t *testing.T) {
	v0 := &math32.Vector2{
		X: 0,
		Y: 0,
	}
	v1 := &math32.Vector2{
		X: 0,
		Y: 5,
	}
	v2 := &math32.Vector2{
		X: 5,
		Y: 5,
	}

	rect := math32.NewRotatedRect3(v0, v1, v2)
	fmt.Printf("rect %s", rect.String())

	if rect.Size.Width != 5 {
		t.Fatalf("rect.Size.Width != 5")
	}
}

func TestRotatedRect2(t *testing.T) {
	v0 := &math32.Vector2{
		X: -5.0,
		Y: 0,
	}
	v1 := &math32.Vector2{
		X: 0,
		Y: -5.0,
	}
	v2 := &math32.Vector2{
		X: 5,
		Y: 0,
	}

	rect := math32.NewRotatedRect3(v0, v1, v2)
	fmt.Printf("rect %s", rect.String())

	if rect.Angle != -45 {
		t.Fatalf("rect.Angle != -45 ")
	}
}

func TestRotatedRect3(t *testing.T) {
	v0 := &math32.Vector2{
		X: -5.0,
		Y: 0,
	}
	v1 := &math32.Vector2{
		X: 0,
		Y: -5.0,
	}
	v2 := &math32.Vector2{
		X: 5,
		Y: 0,
	}

	rect1 := math32.NewRotatedRect3(v0, v1, v2)
	fmt.Printf("rect %s", rect1.String())

	vv0 := &math32.Vector2{
		X: 0,
		Y: 0,
	}
	vv1 := &math32.Vector2{
		X: 0,
		Y: 5,
	}
	vv2 := &math32.Vector2{
		X: 5,
		Y: 5,
	}

	rect2 := math32.NewRotatedRect3(vv0, vv1, vv2)
	fmt.Printf("rect %s", rect2.String())

	ret := rect1.IsCollision(rect2)

	if ret == false {
		t.Fatalf("rect IsCollision is false  ")
	}

}

func TestRotatedRect4(t *testing.T) {
	v0 := &math32.Vector2{
		X: 20.0,
		Y: 20,
	}
	v1 := &math32.Vector2{
		X: 40,
		Y: 20.0,
	}
	v2 := &math32.Vector2{
		X: 40,
		Y: 40,
	}

	rect1 := math32.NewRotatedRect3(v0, v1, v2)
	fmt.Printf("rect %s", rect1.String())

	vv0 := &math32.Vector2{
		X: 0,
		Y: 0,
	}
	vv1 := &math32.Vector2{
		X: 0,
		Y: 5,
	}
	vv2 := &math32.Vector2{
		X: 5,
		Y: 5,
	}

	rect2 := math32.NewRotatedRect3(vv0, vv1, vv2)
	fmt.Printf("rect %s", rect2.String())

	ret := rect1.IsCollision(rect2)

	if ret == true {
		t.Fatalf("rect  IsCollision is true ")
	}

}
