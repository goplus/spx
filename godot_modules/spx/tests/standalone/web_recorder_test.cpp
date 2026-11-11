#include "movie_recorder_manager.h"
#include "movie_writer_webm.h"

#include <cstdlib>
#include <iostream>

static std::vector<std::string> calls;
static int video_init = 1, video_start = 1;
static int recorded_fps = 0;
static String download_name;
static bool api_active = false;
static String api_path;

extern "C" {
int godot_video_recorder_init(int fps) {
	calls.push_back("video.init");
	recorded_fps = fps;
	return video_init;
}
int godot_video_recorder_start() {
	calls.push_back("video.start");
	return video_start;
}
void godot_video_recorder_stop() {
	calls.push_back("video.stop");
}
void godot_video_recorder_cleanup() {
	calls.push_back("video.cleanup");
}
void godot_video_recorder_download_data(const char *filename) {
	calls.push_back("video.download");
	download_name = filename;
}
int godot_web_recording_request_start(const char *);
int godot_web_recording_request_stop();
int godot_web_recording_is_active();
}

Error MovieRecorderManager::start_recording(const RecordingConfig &config) {
	api_path = config.output_path;
	api_active = true;
	return OK;
}
Error MovieRecorderManager::stop_recording() {
	api_active = false;
	return OK;
}
bool MovieRecorderManager::is_recording() {
	return api_active;
}

class WebRecorder : public MovieWriterWebM {
public:
	using MovieWriterWebM::write_frame;
};

static void check(bool condition, const char *message) {
	if (!condition) {
		std::cerr << "Web recorder check failed: " << message << '\n';
		std::exit(1);
	}
}

int main() {
	// Failed initialization or start must report failure and release resources.
	// Exercise the production C++ branch, including retries and repeated stops.
	int cases = 0;
	for (int failure = 0; failure < 3; failure++) {
		video_init = failure == 1 ? 0 : 1;
		video_start = failure == 2 ? -1 : 1;
		const bool succeeds = failure == 0;
		std::vector<std::string> start_calls{ "video.init" };
		if (failure != 1) {
			start_calls.push_back("video.start");
		}
		if (!succeeds) {
			start_calls.push_back("video.cleanup");
		}
		WebRecorder recorder;
		check(recorder.write_frame({}, nullptr) == ERR_UNAVAILABLE, "inactive frame");
		for (int run = 0; run < 2; run++) {
			calls.clear();
			OS::get_singleton()->verbose = run != 0;
			check(recorder.begin_realtime(Size2i(4, 4), 60, "capture.webm") == (succeeds ? OK : ERR_CANT_CREATE), "start result");
			check(calls == start_calls && recorded_fps == 60, "start call order and FPS");
			check(recorder.write_frame({}, nullptr) == (succeeds ? OK : ERR_UNAVAILABLE), "active frame");
			recorder.add_realtime_frame();
			auto expected = start_calls;
			if (succeeds) {
				expected.insert(expected.end(), { "video.stop", "video.download", "video.cleanup" });
			}
			recorder.end_realtime();
			recorder.end_realtime();
			check(calls == expected, "stop order and repeated stop");
			check(recorder.write_frame({}, nullptr) == ERR_UNAVAILABLE, "frame after stop");
		}
		video_init = video_start = 1;
		check(recorder.begin_realtime(Size2i(4, 4), 30, "restarted.webm") == OK, "restart after stop or failure");
		recorder.end_realtime();
		cases++;
	}
	// Request automatic download even before the browser delivers any data.
	// JavaScript owns completion and empty-output handling after C++ cleanup.
	for (int setting : { -1, 0, 1 }) {
		ProjectSettings::get_singleton()->setting_present = setting != -1;
		ProjectSettings::get_singleton()->auto_download = setting == 1;
		WebRecorder recorder;
		check(recorder.begin_realtime({}, 30, "download.webm") == OK, "download recording start");
		calls.clear();
		recorder.end_realtime();
		std::vector<std::string> expected{ "video.stop" };
		if (setting != 0) {
			expected.push_back("video.download");
			check(download_name == "spx_recording_test-time.webm", "download filename");
		}
		expected.push_back("video.cleanup");
		check(calls == expected, "download request precedes cleanup");
		cases++;
	}
	check(godot_web_recording_request_start(nullptr) == 1 && api_path == "recording", "default API path");
	check(godot_web_recording_is_active() == 1, "active API");
	check(godot_web_recording_request_stop() == 1 && godot_web_recording_is_active() == 0, "stopped API");
	check(godot_web_recording_request_start("named.webm") == 1 && api_path == "named.webm", "named API path");
	std::cout << "Web recorder C++: " << cases << " lifecycle/download cases passed\n";
}
