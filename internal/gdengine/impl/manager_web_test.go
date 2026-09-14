//go:build js && wasm && !pure_engine

package impl

import (
	"bytes"
	"syscall/js"
	"testing"

	webffi "github.com/goplus/spx/v3/internal/gdengine/binding/web"
)

func TestBatchRetrievePositionsUpdatesCallerBuffer(t *testing.T) {
	previousBorrow := js.Global().Get("GdspxBorrowNativeArray")
	previousRead := webffi.API.SpxSpriteBatchRetrievePositions
	borrow := js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := js.Global().Get("Uint8Array").New(args[2].Int())
		data.Call("fill", 0xa5)
		return map[string]any{
			"__gdspx_array": true,
			"type":          args[0].Int(), "count": args[1].Int(),
			"data": data,
		}
	})
	success := true
	read := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 2 || args[0].Get("count").Int() != 1 || args[0].Get("type").Int() != 6 || args[1].Get("count").Int() != 2 {
			t.Error("position query has unexpected arguments")
			return nil
		}
		bits := make([]byte, 8)
		js.CopyBytesToGo(bits, args[0].Get("data"))
		if bits[0] != 1 || bits[2] != 0xc0 || bits[3] != 0x7f {
			t.Error("position query changed the ID's NaN bit pattern")
		}
		initial := make([]byte, 8)
		js.CopyBytesToGo(initial, args[1].Get("data"))
		if !bytes.Equal(initial, bytes.Repeat([]byte{0xa5}, 8)) {
			t.Error("output storage was copied or cleared before the call")
		}
		js.CopyBytesToJS(args[1].Get("data"), []byte{0, 0, 64, 64, 0, 0, 128, 192})
		return success
	})
	js.Global().Set("GdspxBorrowNativeArray", borrow)
	webffi.API.SpxSpriteBatchRetrievePositions = read.Value
	t.Cleanup(func() {
		js.Global().Set("GdspxBorrowNativeArray", previousBorrow)
		webffi.API.SpxSpriteBatchRetrievePositions = previousRead
		borrow.Release()
		read.Release()
	})
	ids := []int64{0x112233447fc00001}
	buffer := []float32{42, 42, 42}
	if !new(spriteMgr).BatchRetrievePositions(ids, buffer[:2]) {
		t.Fatal("successful query returned false")
	}
	if ids[0] != 0x112233447fc00001 || buffer[0] != 3 || buffer[1] != -4 || buffer[2] != 42 {
		t.Fatalf("position output = %v", buffer)
	}
	success = false
	buffer[0], buffer[1] = 11, 12
	if new(spriteMgr).BatchRetrievePositions(ids, buffer[:2]) {
		t.Fatal("failed query returned true")
	}
	if buffer[0] != 11 || buffer[1] != 12 || buffer[2] != 42 {
		t.Fatalf("failed query copied output back: %v", buffer)
	}
}

func TestWriteSnapshotUpdatesCallerArray(t *testing.T) {
	previousBorrow := js.Global().Get("GdspxBorrowNativeArray")
	previousWrite := webffi.API.SpxInputWriteSnapshot
	borrow := js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := js.Global().Get("Uint8Array").New(args[2].Int())
		data.Call("fill", 0xa5)
		return map[string]any{
			"__gdspx_array": true,
			"type":          args[0].Int(), "count": args[1].Int(),
			"data": data,
		}
	})
	calls := 0
	write := js.FuncOf(func(_ js.Value, args []js.Value) any {
		calls++
		if len(args) != 1 || args[0].Get("count").Int() != 3 {
			t.Errorf("unexpected snapshot arguments: %v", args)
			return nil
		}
		initial := make([]byte, 12)
		js.CopyBytesToGo(initial, args[0].Get("data"))
		if !bytes.Equal(initial, bytes.Repeat([]byte{0xa5}, 12)) {
			t.Error("snapshot output was copied or cleared before the call")
		}
		js.CopyBytesToJS(args[0].Get("data"), []byte{0, 0, 192, 63, 0, 0, 32, 192, 0, 0, 224, 64})
		return nil
	})
	js.Global().Set("GdspxBorrowNativeArray", borrow)
	webffi.API.SpxInputWriteSnapshot = write.Value
	t.Cleanup(func() {
		js.Global().Set("GdspxBorrowNativeArray", previousBorrow)
		webffi.API.SpxInputWriteSnapshot = previousWrite
		borrow.Release()
		write.Release()
	})

	out := [3]float32{42, 43, 44}
	mgr := new(inputMgr)
	mgr.WriteSnapshot(&out)
	if want := [3]float32{1.5, -2.5, 7}; out != want {
		t.Fatalf("snapshot = %v, want %v", out, want)
	}
	t.Run("nil output", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected nil output to panic")
			}
			if calls != 1 {
				t.Error("nil output reached the Web binding")
			}
		}()
		mgr.WriteSnapshot(nil)
	})
}
