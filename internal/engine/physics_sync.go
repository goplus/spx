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

package engine

const (
	// PhysicsCmdFields is the number of float32 fields per physics command.
	// [cmd, spriteIdLowBits, spriteIdHighBits, a, b, reserved0]
	PhysicsCmdFields = 6

	PhysicsCmdVelocity = 1 + iota
	PhysicsCmdGravity
	PhysicsCmdMass
	PhysicsCmdMode
	PhysicsCmdUseGravity
	PhysicsCmdGravityScale
	PhysicsCmdDrag
	PhysicsCmdFriction
	PhysicsCmdCollisionLayer
	PhysicsCmdCollisionMask
	PhysicsCmdTriggerLayer
	PhysicsCmdTriggerMask
	PhysicsCmdCollisionEnabled
	PhysicsCmdTriggerEnabled
)

type physicsCmd struct {
	kind     int32
	spriteID int64
	a        float32
	b        float32
}

// PhysicsSyncBuffer collects sprite physics config commands for batch processing.
type PhysicsSyncBuffer struct {
	cmds       []physicsCmd
	serialized []float32
}

func (b *PhysicsSyncBuffer) SetVelocity(id int64, x, y float64) {
	b.add(PhysicsCmdVelocity, id, x, y)
}

func (b *PhysicsSyncBuffer) SetGravity(id int64, gravity float64) {
	b.add1(PhysicsCmdGravity, id, gravity)
}

func (b *PhysicsSyncBuffer) SetMass(id int64, mass float64) {
	b.add1(PhysicsCmdMass, id, mass)
}

func (b *PhysicsSyncBuffer) SetPhysicsMode(id int64, mode int64) {
	b.addBits(PhysicsCmdMode, id, uint32(mode))
}

func (b *PhysicsSyncBuffer) SetUseGravity(id int64, enabled bool) {
	b.addBool(PhysicsCmdUseGravity, id, enabled)
}

func (b *PhysicsSyncBuffer) SetGravityScale(id int64, scale float64) {
	b.add1(PhysicsCmdGravityScale, id, scale)
}

func (b *PhysicsSyncBuffer) SetDrag(id int64, drag float64) {
	b.add1(PhysicsCmdDrag, id, drag)
}

func (b *PhysicsSyncBuffer) SetFriction(id int64, friction float64) {
	b.add1(PhysicsCmdFriction, id, friction)
}

func (b *PhysicsSyncBuffer) SetCollisionLayer(id int64, layer int64) {
	b.addBits(PhysicsCmdCollisionLayer, id, uint32(layer))
}

func (b *PhysicsSyncBuffer) SetCollisionMask(id int64, mask int64) {
	b.addBits(PhysicsCmdCollisionMask, id, uint32(mask))
}

func (b *PhysicsSyncBuffer) SetTriggerLayer(id int64, layer int64) {
	b.addBits(PhysicsCmdTriggerLayer, id, uint32(layer))
}

func (b *PhysicsSyncBuffer) SetTriggerMask(id int64, mask int64) {
	b.addBits(PhysicsCmdTriggerMask, id, uint32(mask))
}

func (b *PhysicsSyncBuffer) SetCollisionEnabled(id int64, enabled bool) {
	b.addBool(PhysicsCmdCollisionEnabled, id, enabled)
}

func (b *PhysicsSyncBuffer) SetTriggerEnabled(id int64, enabled bool) {
	b.addBool(PhysicsCmdTriggerEnabled, id, enabled)
}

func (b *PhysicsSyncBuffer) Clear() {
	b.cmds = b.cmds[:0]
}

func (b *PhysicsSyncBuffer) Count() int {
	return len(b.cmds)
}

// Serialize converts the command buffer to:
// [count, cmd0, spriteIdLowBits0, spriteIdHighBits0, a0, b0, reserved0, ...]
func (b *PhysicsSyncBuffer) Serialize() []float32 {
	count := len(b.cmds)
	if count == 0 {
		return nil
	}

	totalSize := 1 + count*PhysicsCmdFields
	b.serialized = ensureFloat32BufferSize(b.serialized, totalSize)
	result := b.serialized
	result[0] = float32(count)

	idx := 1
	for _, cmd := range b.cmds {
		idLow, idHigh := splitInt64BitsToFloat32(cmd.spriteID)
		result[idx] = float32(cmd.kind)
		result[idx+1] = idLow
		result[idx+2] = idHigh
		result[idx+3] = cmd.a
		result[idx+4] = cmd.b
		result[idx+5] = 0
		idx += PhysicsCmdFields
	}

	return result[:totalSize:totalSize]
}

func NewPhysicsSyncBuffer(capacity int) *PhysicsSyncBuffer {
	return &PhysicsSyncBuffer{
		cmds:       make([]physicsCmd, 0, capacity),
		serialized: make([]float32, 0, 1+capacity*PhysicsCmdFields),
	}
}

func (b *PhysicsSyncBuffer) add(kind int, id int64, a, bval float64) {
	b.cmds = append(b.cmds, physicsCmd{
		kind:     int32(kind),
		spriteID: id,
		a:        float32(a),
		b:        float32(bval),
	})
}

func (b *PhysicsSyncBuffer) add1(kind int, id int64, value float64) {
	b.add(kind, id, value, 0)
}

func (b *PhysicsSyncBuffer) addBits(kind int, id int64, bits uint32) {
	b.cmds = append(b.cmds, physicsCmd{
		kind:     int32(kind),
		spriteID: id,
		a:        uint32BitsToFloat32(bits),
	})
}

func (b *PhysicsSyncBuffer) addBool(kind int, id int64, value bool) {
	b.add1(kind, id, boolArg(value))
}
