# SPX Physics API Design

## Design principles

The SPX physics API should remain approachable for Scratch users while exposing enough control for platform games, projectiles, triggers, tile maps, and simple simulation. Defaults must preserve existing non-physics projects. Advanced behavior should be opt-in and use terminology that maps predictably to the engine.

## API overview

The design is organized into four groups:

1. physics-body control;
2. collider configuration;
3. spatial queries and contacts;
4. tile-map operations.

The current implementation is authoritative; some items in this design may represent planned phases rather than shipped API. The examples below are design notes, not a compile-tested Go API reference.

The current sprite-level signatures are:

```go
type PhysicsMode = int64
type ColliderShapeType = int64

func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode)
func (p *SpriteImpl) PhysicsMode() PhysicsMode
func (p *SpriteImpl) Velocity() (velocityX, velocityY float64)
func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64)
func (p *SpriteImpl) Gravity() float64
func (p *SpriteImpl) SetGravity(gravity float64)
func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64)
func (p *SpriteImpl) IsOnFloor() bool

func (p *SpriteImpl) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error
func (p *SpriteImpl) ColliderShape(isTrigger bool) (ColliderShapeType, []float64)
func (p *SpriteImpl) SetColliderPivot(isTrigger bool, offsetX, offsetY float64)
func (p *SpriteImpl) ColliderPivot(isTrigger bool) (offsetX, offsetY float64)
```

The `isTrigger` parameter distinguishes body colliders from trigger shapes, and the getter is `ColliderShape` (singular). XGo overloads may be exposed as multiple generated Go methods; default-argument syntax in a design example is not valid Go syntax.

## Delivery plan

### Phase 1

Provide body modes, gravity/velocity control, basic colliders, and collision/trigger callbacks while preserving existing `touching` behavior.

### Phase 2

Add practical queries, layers/masks, material properties, and better control of kinematic and dynamic bodies.

### Phase 3

Add tile-map collision and bulk operations suitable for platform and grid games.

### Phase 4

Evaluate joints, richer shapes, continuous collision, and advanced engine features based on real projects and performance data.

## Compatibility

- Existing sprites remain non-physics or use legacy behavior unless configured.
- Existing collision APIs keep their documented coordinate and naming semantics.
- New defaults must not make static decorations fall or move.
- Serialization accepts older project assets without requiring new fields.
- Web and native implementations expose equivalent game behavior where possible.

## Core concepts

### Physics control

A sprite selects an appropriate body behavior: static for immovable geometry, kinematic/character-style for code-driven motion, dynamic for simulation-driven motion, or no physics for decorative elements. Velocity, gravity scale, forces/impulses, and body activation are applied only where meaningful for that mode.

### Colliders

Collider shape and trigger/sensor behavior are independent from visual appearance. Shapes are defined in sprite-local coordinates and transformed once by the engine. Collision layers describe what a body is; masks describe what it interacts with.

### Queries

Queries cover common needs such as checking contacts, ray/shape tests, finding nearby objects, and obtaining collision details. Query results must use SPX world coordinates and stable sprite identities rather than leaking Godot node pointers.

### Tile maps

Tile-map APIs should support reading/writing tiles, converting between cell and world coordinates, configuring collision, and applying bulk edits efficiently. Web builds should batch large edits rather than making one bridge call per tile.

## Usage guidance

### Choosing a mode

- Use no physics for backgrounds and UI-like decorations.
- Use static bodies for floors, walls, and fixed obstacles.
- Use kinematic/character motion for player-controlled platform movement.
- Use dynamic bodies for boxes, debris, and objects driven by forces.
- Use triggers for checkpoints, pickups, water regions, and detection zones.

### Performance

Prefer simple collision shapes, reuse objects, use layers/masks to avoid irrelevant pairs, and avoid polling every sprite when an event or spatial query is sufficient. Batch tile-map updates. Do not create and destroy large numbers of physics bodies every frame.

### Common problems

If visuals and collisions disagree, inspect costume anchors and coordinate conversion. If objects pass through thin geometry, reduce per-tick movement or use the supported continuous/shape-cast approach. If a body does not collide, compare both objects' layers and masks and confirm that a trigger was not used accidentally.

## Example scenarios

### Patrol enemy

Use code-driven horizontal velocity, a floor/edge query, and collision events to reverse direction.

### Platform game

Use character-style movement, gravity, grounded-state queries, and static tile or platform colliders.

### Shooting game

Use pooled projectile sprites, a simple collider or ray query, collision layers, and a lifetime/edge cleanup rule.

### Pushable box

Use a dynamic body with mass and friction, colliding with a character body and static level geometry.

### Flying effect

Use a non-physics or kinematic sprite when the path is fully scripted; do not pay for dynamic simulation unnecessarily.

### Background decoration

Disable physics and collision completely.

### Underwater region

Use a trigger volume to change gravity, damping, movement, and visual/audio effects while a sprite is inside.

### Simple AI

Combine proximity/ray queries with collision events. Keep sensing frequency proportional to gameplay needs.

### Minesweeper

Use tile-map cell data and coordinate conversion; physics bodies are not required for purely grid-based interaction.
