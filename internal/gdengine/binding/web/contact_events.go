//go:build js && wasm

/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webffi

import (
	"encoding/binary"
	"syscall/js"
)

const (
	contactCollisionEnter = 1
	contactCollisionStay  = 2
	contactCollisionExit  = 3
	contactTriggerEnter   = 4
	contactTriggerStay    = 5
	contactTriggerExit    = 6
	contactEventBytes     = 4 + 8 + 8 // kind, self ID, other ID
)

var contactEventsHandle js.Func
var contactEventScratch []byte

func registerContactEventQueue() {
	if contactEventsHandle.Type() != js.TypeUndefined {
		return
	}
	contactEventsHandle = js.FuncOf(gdspxContactEvents)
	js.Global().Set("gdspx_on_contact_events", contactEventsHandle)
}

func gdspxContactEvents(this js.Value, args []js.Value) any {
	if len(args) == 0 || !isByteArray(args[0]) {
		return nil
	}

	events := args[0]
	length := events.Length()
	if length < contactEventBytes {
		return nil
	}

	if cap(contactEventScratch) < length {
		contactEventScratch = make([]byte, length)
	}
	buf := contactEventScratch[:length]
	js.CopyBytesToGo(buf, events)

	for i := 0; i+contactEventBytes <= len(buf); i += contactEventBytes {
		kind := int(binary.LittleEndian.Uint32(buf[i : i+4]))
		self := int64(binary.LittleEndian.Uint64(buf[i+4 : i+12]))
		other := int64(binary.LittleEndian.Uint64(buf[i+12 : i+20]))
		dispatchContactEvent(kind, self, other)
	}
	return nil
}

func dispatchContactEvent(kind int, self, other int64) {
	var cb func(int64, int64)
	switch kind {
	case contactCollisionEnter:
		cb = callbacks.OnCollisionEnter
	case contactCollisionStay:
		cb = callbacks.OnCollisionStay
	case contactCollisionExit:
		cb = callbacks.OnCollisionExit
	case contactTriggerEnter:
		cb = callbacks.OnTriggerEnter
	case contactTriggerStay:
		cb = callbacks.OnTriggerStay
	case contactTriggerExit:
		cb = callbacks.OnTriggerExit
	}
	if cb != nil {
		cb(self, other)
	}
}
