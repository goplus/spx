/**************************************************************************/
/*  register_recorder_types.cpp                                          */
/**************************************************************************/

#include "register_recorder_types.h"

#include "core/config/project_settings.h"
#include "movie_recorder_manager.h"
#include "servers/movie_writer/movie_writer.h"
#include "spx_realtime_recorder.h"

#ifdef WEB_ENABLED
#include "movie_writer_webm.h"
#else
#include "movie_writer_obs_runtime.h"
#endif

#ifdef WEB_ENABLED
static MovieWriterWebM *writer_webm = nullptr;
#else
static ObsStyleMovieWriter *writer_obs = nullptr;
#endif

void initialize_spx_recorder_core() {
	GLOBAL_DEF("movie_writer/realtime_mode", false);
	GLOBAL_DEF("movie_writer/obs_mode", false);
	GLOBAL_DEF("movie_writer/enable_audio_playback", true);
	GLOBAL_DEF("movie_writer/enable_web_auto_download", true);
	GLOBAL_DEF(PropertyInfo(Variant::INT, "movie_writer/obs_video_fps", PROPERTY_HINT_RANGE, "10,120,1,suffix:FPS"), 30);
	GLOBAL_DEF(PropertyInfo(Variant::FLOAT, "movie_writer/obs_video_quality", PROPERTY_HINT_RANGE, "0.1,1.0,0.01"), 0.85);
	GLOBAL_DEF(PropertyInfo(Variant::INT, "movie_writer/obs_audio_sample_rate", PROPERTY_HINT_RANGE, "8000,192000,1,suffix:Hz"), 48000);
	GLOBAL_DEF(PropertyInfo(Variant::INT, "movie_writer/obs_audio_channels", PROPERTY_HINT_RANGE, "2,2,1"), 2);
	GLOBAL_DEF("movie_writer/obs_enable_timestamp_chunks", true);
	GLOBAL_DEF("movie_writer/obs_enable_repeat_frame_marking", true);
	GLOBAL_DEF("movie_writer/obs_enable_debug_output", false);
	GLOBAL_DEF("movie_writer/obs_enable_post_merge", true);
	GLOBAL_DEF("movie_writer/obs_keep_intermediate_files", false);
	GLOBAL_DEF("movie_writer/obs_ffmpeg_path", "ffmpeg");
}

void uninitialize_spx_recorder_core() {
	MovieRecorderManager::shutdown();
}

void initialize_spx_recorder_servers() {
#ifdef WEB_ENABLED
	writer_webm = memnew(MovieWriterWebM);
	MovieWriter::add_writer(writer_webm);
#else
	writer_obs = memnew(ObsStyleMovieWriter);
	MovieWriter::add_writer(writer_obs);
#endif
	MovieRecorderManager::initialize();
}

void uninitialize_spx_recorder_servers() {
	// Stop recording and detach main-loop callbacks before deleting the writer
	// selected by those callbacks. Core teardown calls shutdown again safely.
	MovieRecorderManager::shutdown();
#ifdef WEB_ENABLED
	memdelete(writer_webm);
	writer_webm = nullptr;
#else
	memdelete(writer_obs);
	writer_obs = nullptr;
#endif
}

bool spx_recorder_claims_movie(const String &p_path) {
	if (p_path.is_empty()) {
		return false;
	}
#ifdef WEB_ENABLED
	return true;
#else
	if (p_path.get_extension().to_lower() != "avi") {
		return false;
	}
	return bool(GLOBAL_GET("movie_writer/obs_mode")) || bool(GLOBAL_GET("movie_writer/realtime_mode"));
#endif
}

SpxRealtimeRecorder *spx_find_realtime_movie_recorder(const String &p_path) {
#ifdef WEB_ENABLED
	return writer_webm != nullptr && writer_webm->handles_realtime_file(p_path) ? writer_webm : nullptr;
#else
	return writer_obs != nullptr && writer_obs->handles_realtime_file(p_path) ? writer_obs : nullptr;
#endif
}
