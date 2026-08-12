/**************************************************************************/
/*  movie_recorder_manager.cpp                                           */
/**************************************************************************/

#include "movie_recorder_manager.h"

#include "core/os/os.h"
#include "core/os/time.h"
#include "main/main_loop_phase_callback_bus.h"
#include "register_recorder_types.h"
#include "servers/display_server.h"
#include "spx_realtime_recorder.h"

SpxRealtimeRecorder *MovieRecorderManager::instance = nullptr;
MovieRecorderManager::InstanceState MovieRecorderManager::state = NONE;
MovieRecorderManager::RecordingConfig MovieRecorderManager::current_config;
Size2i MovieRecorderManager::default_movie_size;
uint32_t MovieRecorderManager::default_fps = 60;
uint64_t MovieRecorderManager::recording_start_time = 0;
uint64_t MovieRecorderManager::callback_registration = MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;
bool MovieRecorderManager::command_line_recording = false;

void MovieRecorderManager::initialize() {
	if (callback_registration != MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID) {
		return;
	}

	MainLoopPhaseCallbackBus::Callbacks callbacks;
	callbacks.movie_claim = _movie_claim;
	callbacks.movie_requires_live_audio = _movie_requires_live_audio;
	callbacks.movie_begin = _movie_begin;
	callbacks.movie_frame = _movie_frame;
	callbacks.movie_end = _movie_end;
	callbacks.destroy = _destroy;
	callback_registration = get_main_loop_phase_callback_bus().register_callbacks(callbacks);
}

void MovieRecorderManager::shutdown() {
	_finish();
	if (callback_registration != MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID) {
		get_main_loop_phase_callback_bus().unregister_callbacks(callback_registration);
		callback_registration = MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;
	}
}

Error MovieRecorderManager::_begin(const Size2i &p_movie_size, uint32_t p_fps, const String &p_path, bool p_command_line) {
	if (state == STARTED) {
		return ERR_ALREADY_IN_USE;
	}
	ERR_FAIL_COND_V(p_path.is_empty(), ERR_INVALID_PARAMETER);

	instance = spx_find_realtime_movie_recorder(p_path);
	ERR_FAIL_NULL_V_MSG(instance, ERR_FILE_UNRECOGNIZED, "No SPX realtime recorder handles: " + p_path);

	Error err = instance->begin_realtime(p_movie_size, p_fps, p_path);
	if (err != OK) {
		instance = nullptr;
		return err;
	}

	current_config.output_path = p_path;
	current_config.video_fps = p_fps;
	state = STARTED;
	command_line_recording = p_command_line;
	recording_start_time = Time::get_singleton()->get_ticks_usec();
	return OK;
}

Error MovieRecorderManager::start_recording(const RecordingConfig &p_config) {
	if (state == STARTED) {
		return ERR_ALREADY_IN_USE;
	}
	ERR_FAIL_COND_V(p_config.output_path.is_empty(), ERR_INVALID_PARAMETER);
	current_config = p_config;

#ifdef WEB_ENABLED
	// The public Web API starts MediaRecorder in JavaScript. C++ only owns its
	// state; command-line Web recording still uses _begin() through the bus.
	state = STARTED;
	command_line_recording = false;
	recording_start_time = Time::get_singleton()->get_ticks_usec();
	return OK;
#else
	Size2i size(p_config.video_width, p_config.video_height);
	if (size.x <= 0 || size.y <= 0) {
		size = default_movie_size;
		if (DisplayServer::get_singleton() != nullptr) {
			size = DisplayServer::get_singleton()->window_get_size();
		}
	}
	const uint32_t fps = p_config.video_fps > 0 ? p_config.video_fps : default_fps;
	return _begin(size, fps, p_config.output_path, false);
#endif
}

void MovieRecorderManager::_finish() {
	if (state != STARTED) {
		return;
	}
	if (instance != nullptr) {
		instance->end_realtime();
	}
	instance = nullptr;
	state = NONE;
	recording_start_time = 0;
	command_line_recording = false;
}

Error MovieRecorderManager::stop_recording() {
	if (state != STARTED) {
		return ERR_INVALID_PARAMETER;
	}
	_finish();
	return OK;
}

Error MovieRecorderManager::pause_recording() {
	return ERR_UNAVAILABLE;
}

Error MovieRecorderManager::resume_recording() {
	return ERR_UNAVAILABLE;
}

bool MovieRecorderManager::is_recording() {
	return state == STARTED;
}

bool MovieRecorderManager::is_initialized() {
	return callback_registration != MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;
}

bool MovieRecorderManager::has_active_instance() {
	return instance != nullptr;
}

String MovieRecorderManager::get_current_output_path() {
	return current_config.output_path;
}

float MovieRecorderManager::get_recording_duration() {
	if (state != STARTED || recording_start_time == 0) {
		return 0.0f;
	}
	return float(Time::get_singleton()->get_ticks_usec() - recording_start_time) / 1000000.0f;
}

bool MovieRecorderManager::_movie_claim(void *p_userdata, const String &p_movie_path) {
	return spx_recorder_claims_movie(p_movie_path);
}

bool MovieRecorderManager::_movie_requires_live_audio(void *p_userdata, const String &p_movie_path) {
	return spx_recorder_claims_movie(p_movie_path);
}

Error MovieRecorderManager::_movie_begin(void *p_userdata, const Size2i &p_movie_size, uint32_t p_fps, const String &p_movie_path) {
	default_movie_size = p_movie_size;
	default_fps = p_fps > 0 ? p_fps : 60;
	return _begin(p_movie_size, default_fps, p_movie_path, true);
}

void MovieRecorderManager::_movie_frame(void *p_userdata) {
	if (state == STARTED && instance != nullptr) {
		instance->add_realtime_frame();
	}
}

void MovieRecorderManager::_movie_end(void *p_userdata) {
	if (command_line_recording) {
		_finish();
	}
}

void MovieRecorderManager::_destroy(void *p_userdata) {
	_finish();
}
