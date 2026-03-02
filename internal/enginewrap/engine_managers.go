package enginewrap

// EngineManagers groups all engine-facing manager wrappers.
// This keeps runtime dependencies scoped to the game instead of package-level globals.
type EngineManagers struct {
	AudioMgr         AudioMgrImpl
	CameraMgr        CameraMgrImpl
	InputMgr         InputMgrImpl
	PhysicsMgr       PhysicsMgrImpl
	PlatformMgr      PlatformMgrImpl
	ResMgr           ResMgrImpl
	SceneMgr         SceneMgrImpl
	SpriteMgr        SpriteMgrImpl
	UiMgr            UiMgrImpl
	ExtMgr           ExtMgrImpl
	PenMgr           PenMgrImpl
	DebugMgr         DebugMgrImpl
	NavigationMgr    NavigationMgrImpl
	TilemapMgr       TilemapMgrImpl
	TilemapparserMgr TilemapparserMgrImpl
}
