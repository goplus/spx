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

		RecordingConfig() = default;
		explicit RecordingConfig(const String &p_path) :
				output_path(p_path) {}
	};

	static void initialize();
	static void shutdown();

	static Error start_recording(const RecordingConfig &p_config);
	static Error stop_recording();

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
	static String current_output_path;
	static Size2i default_movie_size;
	static uint32_t default_fps;
	static uint64_t recording_start_time;
	static uint64_t callback_registration;
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
