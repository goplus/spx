package enginewrap

// This file provides coordinate system adapters for all managers.
// SPX uses Y-axis pointing up, while Godot uses Y-axis pointing down.
// All coordinate conversions are handled here by overriding manager methods.

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

// flipY flips the Y coordinate to convert between SPX and Godot coordinate systems
func flipY(pos Vec2) Vec2 {
	return Vec2{X: pos.X, Y: -pos.Y}
}

// ============================================================================
// Sprite Coordinate Adapters
// ============================================================================

func (pself *Sprite) SetPosition(pos Vec2) {
	pself.Sprite.SetPosition(flipY(pos))
}

func (pself *Sprite) GetPosition() Vec2 {
	return flipY(pself.Sprite.GetPosition())
}

func (pself *Sprite) SetChildPosition(path string, pos Vec2) {
	pself.Sprite.SetChildPosition(path, flipY(pos))
}

func (pself *Sprite) GetChildPosition(path string) Vec2 {
	return flipY(pself.Sprite.GetChildPosition(path))
}

func (pself *Sprite) SetVelocity(velocity Vec2) {
	pself.Sprite.SetVelocity(flipY(velocity))
}

func (pself *Sprite) GetVelocity() Vec2 {
	return flipY(pself.Sprite.GetVelocity())
}

func (pself *Sprite) SetPivot(pivot Vec2) {
	pself.Sprite.SetPivot(flipY(pivot))
}

func (pself *Sprite) GetPivot() Vec2 {
	return flipY(pself.Sprite.GetPivot())
}

func (pself *Sprite) CheckCollisionWithPoint(point Vec2, isTrigger bool) bool {
	return pself.Sprite.CheckCollisionWithPoint(flipY(point), isTrigger)
}

// extra wrap functions
func (pself *Sprite) GetLastMotion() Vec2 {
	return flipY(pself.Sprite.GetLastMotion())
}

func (pself *Sprite) GetPositionDelta() Vec2 {
	return flipY(pself.Sprite.GetPositionDelta())
}

func (pself *Sprite) GetFloorNormal() Vec2 {
	return flipY(pself.Sprite.GetFloorNormal())
}

func (pself *Sprite) GetWallNormal() Vec2 {
	return flipY(pself.Sprite.GetWallNormal())
}

func (pself *Sprite) GetRealVelocity() Vec2 {
	return flipY(pself.Sprite.GetRealVelocity())
}

func (pself *Sprite) SetAnimOffset(offset Vec2) {
	pself.Sprite.SetAnimOffset(flipY(offset))
}

func (pself *Sprite) GetAnimOffset() Vec2 {
	return flipY(pself.Sprite.GetAnimOffset())
}

// ============================================================================
// SpriteMgr Coordinate Adapters
// ============================================================================

func (pself *SpriteMgrImpl) SetPosition(obj gdx.Object, pos Vec2) {
	pself.spriteMgrImpl.SetPosition(obj, flipY(pos))
}

func (pself *SpriteMgrImpl) GetPosition(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetPosition(obj))
}

func (pself *SpriteMgrImpl) SetChildPosition(obj gdx.Object, path string, pos Vec2) {
	pself.spriteMgrImpl.SetChildPosition(obj, path, flipY(pos))
}

func (pself *SpriteMgrImpl) GetChildPosition(obj gdx.Object, path string) Vec2 {
	return flipY(pself.spriteMgrImpl.GetChildPosition(obj, path))
}

func (pself *SpriteMgrImpl) SetVelocity(obj gdx.Object, velocity Vec2) {
	pself.spriteMgrImpl.SetVelocity(obj, flipY(velocity))
}

func (pself *SpriteMgrImpl) GetVelocity(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetVelocity(obj))
}

func (pself *SpriteMgrImpl) SetPivot(obj gdx.Object, pivot Vec2) {
	pself.spriteMgrImpl.SetPivot(obj, flipY(pivot))
}

func (pself *SpriteMgrImpl) GetPivot(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetPivot(obj))
}

func (pself *SpriteMgrImpl) CheckCollisionWithPoint(obj gdx.Object, point Vec2, isTrigger bool) bool {
	return pself.spriteMgrImpl.CheckCollisionWithPoint(obj, flipY(point), isTrigger)
}

// extra wrap functions
func (pself *SpriteMgrImpl) GetLastMotion(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetLastMotion(obj))
}

func (pself *SpriteMgrImpl) GetPositionDelta(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetPositionDelta(obj))
}

func (pself *SpriteMgrImpl) GetFloorNormal(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetFloorNormal(obj))
}

func (pself *SpriteMgrImpl) GetWallNormal(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetWallNormal(obj))
}

func (pself *SpriteMgrImpl) GetRealVelocity(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetRealVelocity(obj))
}

func (pself *SpriteMgrImpl) SetAnimOffset(obj gdx.Object, offset Vec2) {
	pself.spriteMgrImpl.SetAnimOffset(obj, flipY(offset))
}

func (pself *SpriteMgrImpl) GetAnimOffset(obj gdx.Object) Vec2 {
	return flipY(pself.spriteMgrImpl.GetAnimOffset(obj))
}

func (pself *SpriteMgrImpl) AddImpulse(obj gdx.Object, impulse Vec2) {
	pself.spriteMgrImpl.AddImpulse(obj, flipY(impulse))
}

func (pself *SpriteMgrImpl) GetGravity(obj gdx.Object) float64 {
	return -pself.spriteMgrImpl.GetGravity(obj)
}

func (pself *SpriteMgrImpl) SetGravity(obj gdx.Object, gravity float64) {
	pself.spriteMgrImpl.SetGravity(obj, -gravity)
}

// ============================================================================
// SceneMgr Coordinate Adapters
// ============================================================================

func (pself *SceneMgrImpl) CreateRenderSprite(texturePath string, pos Vec2, degree float64, scale Vec2, zindex int64, pivot Vec2) gdx.Object {
	return pself.sceneMgrImpl.CreateRenderSprite(texturePath, flipY(pos), degree, scale, zindex, flipY(pivot))
}

func (pself *SceneMgrImpl) CreateStaticSprite(texturePath string, pos Vec2, degree float64, scale Vec2, zindex int64, pivot Vec2, colliderType int64, colliderPivot Vec2, colliderParams gdx.Array) gdx.Object {
	return pself.sceneMgrImpl.CreateStaticSprite(texturePath, flipY(pos), degree, scale, zindex, flipY(pivot), colliderType, flipY(colliderPivot), colliderParams)
}

// ============================================================================
// PhysicMgr Coordinate Adapters
// ============================================================================

func (pself *PhysicMgrImpl) Raycast(from Vec2, to Vec2, collisionMask int64) gdx.Object {
	return pself.physicMgrImpl.Raycast(flipY(from), flipY(to), collisionMask)
}

// RaycastWithDetails returns array: [collide, sprite_gid, pos.x, pos.y, normal.x, normal.y]
// This conversion should be handled at upper layer due to gdx.Array access limitations
func (pself *PhysicMgrImpl) RaycastWithDetails(from Vec2, to Vec2, ignoreSprites gdx.Array, collisionMask int64, collideWithAreas bool, collideWithBodies bool) gdx.Array {
	return pself.physicMgrImpl.RaycastWithDetails(flipY(from), flipY(to), ignoreSprites, collisionMask, collideWithAreas, collideWithBodies)
}

func (pself *PhysicMgrImpl) CheckCollisionRect(pos Vec2, size Vec2, collisionMask int64) gdx.Array {
	return pself.physicMgrImpl.CheckCollisionRect(flipY(pos), size, collisionMask)
}

func (pself *PhysicMgrImpl) CheckCollisionCircle(pos Vec2, radius float64, collisionMask int64) gdx.Array {
	return pself.physicMgrImpl.CheckCollisionCircle(flipY(pos), radius, collisionMask)
}

// ============================================================================
// InputMgr Coordinate Adapters
// ============================================================================
func (pself *InputMgrImpl) SyncGetGlobalMousePos() Vec2 {
	return flipY(gdx.InputMgr.GetGlobalMousePos())
}

func (pself *InputMgrImpl) GetGlobalMousePos() Vec2 {
	return flipY(pself.inputMgrImpl.GetGlobalMousePos())
}

// ============================================================================
// CameraMgr Coordinate Adapters
// ============================================================================
func (pself *CameraMgrImpl) GetCameraPosition() Vec2 {
	return flipY(pself.cameraMgrImpl.GetCameraPosition())
}
func (pself *CameraMgrImpl) SetCameraPosition(position Vec2) {
	pself.cameraMgrImpl.SetCameraPosition(flipY(position))
}
func (pself *CameraMgrImpl) SyncGetCameraPosition() Vec2 {
	return flipY(gdx.CameraMgr.GetCameraPosition())
}
func (pself *CameraMgrImpl) SyncSetCameraPosition(position Vec2) {
	gdx.CameraMgr.SetCameraPosition(flipY(position))
}

// ============================================================================
// DebugMgr Coordinate Adapters
// ============================================================================

func (pself *DebugMgrImpl) DebugDrawCircle(pos Vec2, radius float64, color Color) {
	pself.debugMgrImpl.DebugDrawCircle(flipY(pos), radius, color)
}

func (pself *DebugMgrImpl) DebugDrawRect(pos Vec2, size Vec2, color Color) {
	pself.debugMgrImpl.DebugDrawRect(flipY(pos), size, color)
}

func (pself *DebugMgrImpl) DebugDrawLine(from Vec2, to Vec2, color Color) {
	pself.debugMgrImpl.DebugDrawLine(flipY(from), flipY(to), color)
}

// ============================================================================
// TilemapMgr Coordinate Adapters
// ============================================================================

func (pself *TilemapMgrImpl) PlaceTile(pos Vec2, texturePath string) {
	pself.tilemapMgrImpl.PlaceTile(flipY(pos), texturePath)
}

func (pself *TilemapMgrImpl) PlaceTileWithLayer(pos Vec2, texturePath string, layerIndex int64) {
	pself.tilemapMgrImpl.PlaceTileWithLayer(flipY(pos), texturePath, layerIndex)
}

func (pself *TilemapMgrImpl) EraseTile(pos Vec2) {
	pself.tilemapMgrImpl.EraseTile(flipY(pos))
}

func (pself *TilemapMgrImpl) EraseTileWithLayer(pos Vec2, layerIndex int64) {
	pself.tilemapMgrImpl.EraseTileWithLayer(flipY(pos), layerIndex)
}

func (pself *TilemapMgrImpl) GetTile(pos Vec2) string {
	return pself.tilemapMgrImpl.GetTile(flipY(pos))
}

func (pself *TilemapMgrImpl) GetTileWithLayer(pos Vec2, layerIndex int64) string {
	return pself.tilemapMgrImpl.GetTileWithLayer(flipY(pos), layerIndex)
}

func (pself *TilemapMgrImpl) SetLayerOffset(index int64, offset Vec2) {
	pself.tilemapMgrImpl.SetLayerOffset(index, flipY(offset))
}

func (pself *TilemapMgrImpl) GetLayerOffset(index int64) Vec2 {
	return flipY(pself.tilemapMgrImpl.GetLayerOffset(index))
}

// ============================================================================
// NavigationMgr Coordinate Adapters
// ============================================================================

// FindPath returns array: [x0, y0, x1, y1, x2, y2, ...]
// This conversion should be handled at upper layer due to gdx.Array access limitations
func (pself *NavigationMgrImpl) FindPath(from Vec2, to Vec2, withJump bool) gdx.Array {
	var arr = pself.navigationMgrImpl.FindPath(flipY(from), flipY(to), withJump)
	result := arr.([]float32)
	if result == nil {
		return nil
	}
	// flipY to convert godot coord to spx coord
	for i := 0; i+1 < len(result); i += 2 {
		result[i+1] = -result[i+1]
	}
	return result
}
