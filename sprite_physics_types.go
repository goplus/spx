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

package spx

type PhysicsMode = int64

const (
	NoPhysics        PhysicsMode = 0 // Pure visual, no collision, best performance (current default) eg: decorators
	KinematicPhysics PhysicsMode = 1 // Code-controlled movement with collision detection eg: player
	DynamicPhysics   PhysicsMode = 2 // Affected by physics, automatic gravity and collision eg: items
	StaticPhysics    PhysicsMode = 3 // Static immovable, but has collision, affects other objects : eg: walls
)

type ColliderShapeType = int64

const (
	RectCollider      ColliderShapeType = ColliderShapeType(physicsColliderRect)
	CircleCollider    ColliderShapeType = ColliderShapeType(physicsColliderCircle)
	CapsuleCollider   ColliderShapeType = ColliderShapeType(physicsColliderCapsule)
	PolygonCollider   ColliderShapeType = ColliderShapeType(physicsColliderPolygon)
	TriggerExtraPixel float64           = 2.0
)

// toPhysicsMode converts string to PhysicsMode.
func toPhysicsMode(mode string) PhysicsMode {
	if mode == "" {
		return NoPhysics
	}
	switch mode {
	case "kinematic":
		return KinematicPhysics
	case "dynamic":
		return DynamicPhysics
	case "static":
		return StaticPhysics
	case "no":
		return NoPhysics
	}
	println("config error: unknown physics mode ", mode)
	return NoPhysics
}
