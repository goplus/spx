#include "register_types.h"

#include "recorder/register_recorder_types.h"
#include "spx.h"

#ifdef WEB_ENABLED
#include "web/spx_web_bridge.h"
#endif

void initialize_spx_module(ModuleInitializationLevel p_level) {
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
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_SERVERS) {
		uninitialize_spx_recorder_servers();
	} else if (p_level == MODULE_INITIALIZATION_LEVEL_CORE) {
		uninitialize_spx_recorder_core();
		Spx::unregister_extension_functions();
	}
}
