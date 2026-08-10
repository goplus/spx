/**************************************************************************/
/*  movie_recorder_manager.h                                             */
/**************************************************************************/

#ifndef SPX_MOVIE_RECORDER_MANAGER_H
#define SPX_MOVIE_RECORDER_MANAGER_H

#include "core/math/vector2i.h"
#include "core/string/ustring.h"

class SpxRealtimeRecorder;

class MovieRecorderManager {
public:
	struct RecordingConfig {
		String output_path;
		uint32_t video_fps = 30;
		uint32_t video_width = 0;
		uint32_t video_height = 0;
		float video_quality = 0.85f;
		uint32_t audio_sample_rate = 48000;
		uint32_t audio_channels = 2;
		bool enable_audio = true;
		bool realtime_mode = true;

		RecordingConfig() = default;
		explicit RecordingConfig(const String &p_path) :
				output_path(p_path) {}
	};

	static void initialize();
	static void shutdown();

	// Compatibility entry points retained for module callers.
	static void onInit();
	static void onStart();
	static void onUpdate();
	static void onCleanup();
	static void set_fixed_fps(int p_fps);

	static Error start_recording(const RecordingConfig &p_config);
	static Error stop_recording();
	static Error pause_recording();
	static Error resume_recording();

	static bool is_recording();
	static bool is_initialized();
	static bool has_active_instance();
	static String get_current_output_path();
	static float get_recording_duration();

private:
	enum InstanceState {
		NONE,
		STARTED,
	};

	static SpxRealtimeRecorder *instance;
	static InstanceState state;
	static RecordingConfig current_config;
	static Size2i default_movie_size;
	static uint32_t default_fps;
	static uint64_t recording_start_time;
	static uint64_t callback_registration;
	static bool initialized;
	static bool command_line_recording;

	static Error _begin(const Size2i &p_movie_size, uint32_t p_fps, const String &p_path, bool p_command_line);
	static void _finish();

	static bool _movie_claim(void *p_userdata, const String &p_movie_path);
	static bool _movie_requires_live_audio(void *p_userdata, const String &p_movie_path);
	static Error _movie_begin(void *p_userdata, const Size2i &p_movie_size, uint32_t p_fps, const String &p_movie_path);
	static void _movie_frame(void *p_userdata);
	static void _movie_end(void *p_userdata);
	static void _destroy(void *p_userdata);
};

#endif // SPX_MOVIE_RECORDER_MANAGER_H
