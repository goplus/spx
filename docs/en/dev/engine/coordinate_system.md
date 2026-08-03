# SPX Coordinate Systems and the Godot Boundary

SPX exposes Scratch-style coordinates while Godot uses a canvas coordinate system. Coordinate conversion must happen at explicit ownership boundaries; mixing spaces inside gameplay or physics code causes doubled offsets, inverted axes, and collision mismatches.

## 1. Coordinate spaces

### 1.1 SPX world coordinates

The stage center is the origin. Positive X points right and positive Y points up. Public sprite positions, movement APIs, stage bounds, and game logic use this space.

### 1.2 Asset coordinates

Costumes, SVG content, collision paths, and editor-authored metadata may use top-left origins or asset-local pixel units. Asset import is responsible for normalizing these values.

### 1.3 SPX local render coordinates

Render-local values describe geometry relative to a sprite origin or costume anchor. They must not include the sprite's world translation a second time.

### 1.4 Godot canvas coordinates

Godot canvas coordinates normally have positive Y pointing down. The SPX/Godot bridge converts positions and vectors when data crosses the boundary.

## 2. Ownership rules

- SPX owns logical game positions and exposes them through the public API.
- Godot owns render nodes, physics bodies, and canvas-space transforms.
- Import code owns asset-space normalization.
- A conversion belongs at the boundary between two spaces, not in every caller.
- Store a value in one canonical space and convert it only for the consumer.

## 3. Main data flows

### 3.1 SPX-driven sprites

SPX updates a logical world position. The bridge converts it once and applies the corresponding Godot transform. Costume anchor and local render offsets are applied as local geometry, not folded into the logical position.

### 3.2 Physics feedback from Godot

When Godot physics moves a body, the bridge converts the resulting canvas position back to SPX world coordinates before updating public sprite state. Feedback synchronization must avoid treating the converted value as a new independent movement command.

### 3.3 Collision shapes

Collision geometry is imported from asset-local data, normalized around the sprite/costume origin, and then attached to the physics body. Body translation supplies world placement; collision vertices must not repeat it.

## 4. Other valid conversions

Screen input, camera transforms, UI layout, tile maps, and exported asset metadata can require their own spaces. Name helper functions by source and destination when the conversion is not obvious, and keep vector conversion separate from point conversion when translation is involved.

## 5. Change checklist

- Identify the coordinate space of every input and output.
- Verify Y-axis direction and origin.
- Check whether camera, stage-center, costume-anchor, or local offsets are already applied.
- Test both SPX-driven movement and physics-driven feedback.
- Compare rendered geometry with collision geometry.
- Test nonzero positions, rotations, scaling, and asymmetric costumes.
- Add a regression test at the boundary where the conversion occurs.
