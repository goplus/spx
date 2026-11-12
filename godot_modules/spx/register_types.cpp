#include "register_types.h"

#include "recorder/register_recorder_types.h"
#include "spx.h"
#include "spx_build_config.gen.h"
#ifdef SPX_SPINE_ENABLED
#include "thirdparty/spine_godot/register_types.h"
#endif

#ifdef WEB_ENABLED
#include "web/spx_web_bridge.h"
#endif

void initialize_spx_module(ModuleInitializationLevel p_level) {
#ifdef SPX_SPINE_ENABLED
	initialize_spine_godot_module(p_level);
#endif
	if (p_level == MODULE_INITIALIZATION_LEVEL_CORE) {
		Spx::register_extension_functions();
		initialize_spx_recorder_core();
#ifdef WEB_ENABLED
		spx_web_register_callbacks();
#endif
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_SERVERS) {
		initialize_spx_recorder_servers();
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_SCENE) {
		Spx::register_types();
		Spx::register_main_loop_callbacks();
	}
}

void uninitialize_spx_module(ModuleInitializationLevel p_level) {
	if (p_level == MODULE_INITIALIZATION_LEVEL_SCENE) {
		Spx::unregister_main_loop_callbacks();
		// Normally the main-loop destroy phase owns this cleanup. Keep module
		// teardown as an idempotent fallback for startup failures and tests that
		// never enter a SceneTree.
		Spx::on_destroy();
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_SERVERS) {
		uninitialize_spx_recorder_servers();
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_CORE) {
		uninitialize_spx_recorder_core();
		Spx::unregister_extension_functions();
	}
#ifdef SPX_SPINE_ENABLED
	uninitialize_spine_godot_module(p_level);
#endif
}
