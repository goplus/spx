/**************************************************************************/
/*  movie_writer_webm.cpp                                                */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#include "movie_writer_webm.h"
#include "core/config/project_settings.h"
#include "core/os/os.h"
#include "core/os/time.h"
#include "movie_recorder_manager.h"

#if defined(WEB_ENABLED) && defined(MODULE_SPX_ENABLED)
#include <emscripten.h>

extern "C" {
int godot_video_recorder_init(int p_fps);
int godot_video_recorder_start();
void godot_video_recorder_stop();
void godot_video_recorder_download_data(const char *p_filename);
void godot_video_recorder_cleanup();
}
#endif

MovieWriterWebM::MovieWriterWebM() {
	if (ProjectSettings::get_singleton()->has_setting("movie_writer/enable_web_auto_download")) {
		enable_auto_download = GLOBAL_GET("movie_writer/enable_web_auto_download");
	} else {
		enable_auto_download = true;
	}
}

bool MovieWriterWebM::handles_file(const String &p_path) const {
	// web only support this movie writer
#if !defined(WEB_ENABLED) || !defined(MODULE_SPX_ENABLED)
	return false;
#else
	return true;
#endif
}

uint32_t MovieWriterWebM::get_audio_mix_rate() const {
	return 48000;
}

AudioServer::SpeakerMode MovieWriterWebM::get_audio_speaker_mode() const {
	return AudioServer::SPEAKER_MODE_STEREO;
}

void MovieWriterWebM::get_supported_extensions(List<String> *r_extensions) const {
	r_extensions->push_back("webm");
}

bool MovieWriterWebM::handles_realtime_file(const String &p_path) const {
	return handles_file(p_path);
}

Error MovieWriterWebM::begin_realtime(const Size2i &p_movie_size, uint32_t p_fps, const String &p_base_path) {
	return write_begin(p_movie_size, p_fps, p_base_path);
}

void MovieWriterWebM::add_realtime_frame() {
	// Canvas.captureStream() is driven by the browser compositor.
}

void MovieWriterWebM::end_realtime() {
	write_end();
}

#if !defined(WEB_ENABLED) || !defined(MODULE_SPX_ENABLED) // give an empty implementation

Error MovieWriterWebM::write_begin(const Size2i &p_movie_size, uint32_t p_fps, const String &p_base_path) {
	return ERR_UNAVAILABLE;
}
Error MovieWriterWebM::write_frame(const Ref<Image> &p_image, const int32_t *p_audio_data) {
	return ERR_UNAVAILABLE;
}
void MovieWriterWebM::write_end() {
}

#else //WEB_ENABLED
Error MovieWriterWebM::write_begin(const Size2i &p_movie_size, uint32_t p_fps, const String &p_base_path) {
	if (OS::get_singleton()->is_stdout_verbose()) {
		print_line("MovieWriterWebM: Starting web realtime recording");
	}

	if (godot_video_recorder_init(p_fps) != 1 || godot_video_recorder_start() != 1) {
		// Either operation can fail after creating browser-side resources.
		godot_video_recorder_cleanup();
		ERR_PRINT("MovieWriterWebM: Failed to start Canvas video recording");
		return ERR_CANT_CREATE;
	}

	web_video_recording_active = true;
	if (OS::get_singleton()->is_stdout_verbose()) {
		print_line("MovieWriterWebM: Canvas video + audio recording started");
		print_line("  Using Canvas.captureStream() for video");
		print_line("  Using MediaRecorder API for audio+video combined recording");
		print_line("  Frame rate: " + itos(p_fps) + " FPS");
	}
	return OK;
}

Error MovieWriterWebM::write_frame(const Ref<Image> &p_image, const int32_t *p_audio_data) {
	return web_video_recording_active ? OK : ERR_UNAVAILABLE;
}

void MovieWriterWebM::write_end() {
	if (!web_video_recording_active) {
		return;
	}

	godot_video_recorder_stop();
	web_video_recording_active = false;

	if (enable_auto_download) {
		// JavaScript retains this session and waits for its final data event.
		String filename = String("spx_recording_") + Time::get_singleton()->get_datetime_string_from_system(false, true) + ".webm";
		godot_video_recorder_download_data(filename.utf8().get_data());
		if (OS::get_singleton()->is_stdout_verbose()) {
			print_line("MovieWriterWebM: Canvas video file download initiated: " + filename);
		}
	}

	godot_video_recorder_cleanup();
	if (OS::get_singleton()->is_stdout_verbose()) {
		print_line("MovieWriterWebM: Canvas video recorder cleaned up");
	}
}

extern "C" {
EMSCRIPTEN_KEEPALIVE
int godot_web_recording_request_start(const char *filename) {
	String godot_filename = String::utf8(filename ? filename : "recording");

	MovieRecorderManager::RecordingConfig config(godot_filename);
	Error result = MovieRecorderManager::start_recording(config);

	return result == OK ? 1 : 0;
}

EMSCRIPTEN_KEEPALIVE
int godot_web_recording_request_stop() {
	Error result = MovieRecorderManager::stop_recording();
	return result == OK ? 1 : 0;
}

EMSCRIPTEN_KEEPALIVE
int godot_web_recording_is_active() {
	return MovieRecorderManager::is_recording() ? 1 : 0;
}
}

#endif // WEB_ENABLED && MODULE_SPX_ENABLED
