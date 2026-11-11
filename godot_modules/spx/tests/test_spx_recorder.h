/**************************************************************************/
/*  test_spx_recorder.h                                                 */
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

#ifndef TEST_SPX_RECORDER_H
#define TEST_SPX_RECORDER_H

#include "../recorder/audio_driver_hybrid.h"
#include "../recorder/independent_audio_recorder.h"
#include "../recorder/independent_video_recorder.h"
#include "../recorder/movie_recorder_manager.h"
#include "../recorder/movie_writer_obs_runtime.h"
#include "../recorder/movie_writer_webm.h"
#include "../recorder/post_merge_processor.h"
#include "../recorder/register_recorder_types.h"
#include "../recorder/simple_audio_writer.h"
#include "../recorder/simple_video_writer.h"
#include "../recorder/thread_safe_frame_buffer.h"
#include "core/io/marshalls.h"
#include "core/os/semaphore.h"
#include "core/string/print_string.h"
#include "main/main_loop_phase_callback_bus.h"
#include "servers/audio/effects/audio_effect_amplify.h"
#include "servers/rendering_server.h"
#include "spx_test_fixtures.h"

#include <atomic>

namespace TestSpxRecorder {

TEST_CASE("[SPX] Recording buffer publishes the latest frame and resets its statistics") {
	ThreadSafeFrameBuffer buffer;
	CHECK(buffer.get_current_frame().image.is_null());
	CHECK(buffer.get_current_frame().frame_sequence == 0);
	buffer.update_frame(Ref<Image>(), 1);
	CHECK(buffer.get_total_updates() == 0);
	Ref<Image> first = Image::create_empty(2, 2, false, Image::FORMAT_RGB8);
	Ref<Image> second = Image::create_empty(4, 4, false, Image::FORMAT_RGBA8);
	buffer.update_frame(first, 7);
	const auto snapshot = buffer.get_current_frame();
	CHECK(snapshot.image == first);
	CHECK(snapshot.frame_sequence == 7);
	buffer.update_frame(second, 8);
	const auto latest = buffer.get_current_frame();
	CHECK(latest.image == second);
	CHECK(latest.frame_sequence == 8);
	CHECK(snapshot.image == first);
	CHECK(snapshot.frame_sequence == 7);
	CHECK(buffer.get_total_updates() == 2);
	buffer.reset();
	CHECK(buffer.get_current_frame().image.is_null());
	CHECK(buffer.get_current_frame().frame_sequence == 0);
	CHECK(buffer.get_total_updates() == 0);
	CHECK(snapshot.image == first);
	CHECK(latest.image == second);
	buffer.update_frame(first, 1);
	CHECK(buffer.get_current_frame().image == first);
	CHECK(buffer.get_current_frame().frame_sequence == 1);
	CHECK(buffer.get_total_updates() == 1);
}

#ifdef THREADS_ENABLED
TEST_CASE("[SPX] Recording buffer publishes coherent image and sequence snapshots across threads") {
	ThreadSafeFrameBuffer buffer;
	struct Publisher {
		ThreadSafeFrameBuffer *buffer;
		Ref<Image> odd = Image::create_empty(2, 2, false, Image::FORMAT_RGB8);
		Ref<Image> even = Image::create_empty(4, 4, false, Image::FORMAT_RGBA8);
		Semaphore first_published;
		Semaphore reader_ready;
		std::atomic<bool> done{ false };
	} publisher{ &buffer };
	Thread producer;
	producer.start([](void *userdata) {
		auto &state = *static_cast<Publisher *>(userdata);
		for (uint32_t sequence = 1; sequence <= 2000; sequence++) {
			state.buffer->update_frame(sequence % 2 ? state.odd : state.even, sequence);
			if (sequence == 1) {
				state.first_published.post();
				state.reader_ready.wait();
			}
		}
		state.done.store(true);
	},
			&publisher);
	bool coherent = true;
	const bool started = producer.is_started();
	if (started) {
		publisher.first_published.wait();
		CHECK(buffer.get_current_frame().frame_sequence == 1);
		publisher.reader_ready.post();
		do {
			const auto snapshot = buffer.get_current_frame();
			if (snapshot.image.is_valid()) {
				coherent = coherent && snapshot.image == (snapshot.frame_sequence % 2 ? publisher.odd : publisher.even);
			}
		} while (!publisher.done.load());
		producer.wait_to_finish();
	}
	REQUIRE(started);
	CHECK(coherent);
	CHECK(buffer.get_current_frame().image == publisher.even);
	CHECK(buffer.get_current_frame().frame_sequence == 2000);
	CHECK(buffer.get_total_updates() == 2000);
}
#endif

// Locate the known chunk tags in these small fixtures, then check the bytes
// independently of the recorder's own AVI reader.
static int find_tag(const PackedByteArray &bytes, const char *tag) {
	for (int i = 0; i + 4 <= bytes.size(); i++) {
		if (memcmp(bytes.ptr() + i, tag, 4) == 0) {
			return i;
		}
	}
	return -1;
}

static void write_avi(const String &path, const PackedByteArray &bytes) {
	auto file = FileAccess::open(path, FileAccess::WRITE);
	REQUIRE(file.is_valid());
	file->store_buffer(bytes);
}

static PackedByteArray read_avi(const String &path) {
	PackedByteArray bytes = FileAccess::get_file_as_bytes(path);
	REQUIRE(bytes.size() >= 12);
	CHECK(find_tag(bytes, "RIFF") == 0);
	CHECK(find_tag(bytes, "AVI ") == 8);
	CHECK(decode_uint32(bytes.ptr() + 4) == bytes.size() - 8);
	return bytes;
}

TEST_CASE("[SPX] Audio recorder writes PCM payload sample counts and chunk indexes") {
	TestSpx::TempDirectory files("spx_audio_writer");
	const int32_t samples[] = { 0, 100, -100, 200, -200, 0 };
	SimpleAudioWriter writer;
	ERR_PRINT_OFF;
	CHECK(writer.write_audio_chunk(samples, 3) == ERR_UNCONFIGURED);
	CHECK(writer.open(files.file("missing/audio.avi"), 48000, 2) == ERR_CANT_OPEN);
	ERR_PRINT_ON;
	REQUIRE(writer.open(files.file("audio.avi"), 48000, 2) == OK);
	ERR_PRINT_OFF;
	CHECK(writer.write_audio_chunk(nullptr, 3) == ERR_INVALID_PARAMETER);
	CHECK(writer.write_audio_chunk(samples, 0) == ERR_INVALID_PARAMETER);
	ERR_PRINT_ON;
	CHECK(writer.get_chunk_count() == 0);
	REQUIRE(writer.write_audio_chunk(samples, 3) == OK);
	REQUIRE(writer.write_audio_chunk(samples, 1) == OK);
	CHECK(writer.get_chunk_count() == 2);
	writer.close();
	writer.close();
	const PackedByteArray bytes = read_avi(files.file("audio.avi"));
	const int stream = find_tag(bytes, "auds");
	const int format = find_tag(bytes, "strf");
	const int chunk = find_tag(bytes, "00wb");
	const int index = find_tag(bytes, "idx1");
	REQUIRE(stream >= 0);
	REQUIRE(format >= 0);
	REQUIRE(chunk >= 0);
	REQUIRE(index >= 0);
	CHECK(decode_uint32(bytes.ptr() + stream + 32) == 4); // Total stereo frames.
	CHECK(decode_uint16(bytes.ptr() + format + 10) == 2);
	CHECK(decode_uint32(bytes.ptr() + format + 12) == 48000);
	CHECK(decode_uint32(bytes.ptr() + chunk + 4) == sizeof(samples));
	CHECK(memcmp(bytes.ptr() + chunk + 8, samples, sizeof(samples)) == 0);
	CHECK(decode_uint32(bytes.ptr() + index + 4) == 2 * 16);
	CHECK(index + 8 + 2 * 16 == bytes.size());
	CHECK(decode_uint32(bytes.ptr() + index + 8 + 8) == 4);
	CHECK(decode_uint32(bytes.ptr() + index + 8 + 16 + 8) == 4 + 8 + sizeof(samples));
	REQUIRE(writer.open(files.file("empty.avi"), 22050, 1) == OK);
	CHECK(writer.get_chunk_count() == 0);
	writer.close();
	read_avi(files.file("empty.avi"));
}

TEST_CASE("[SPX] Video recorder encodes frames without mutating caller images") {
	TestSpx::TempDirectory files("spx_video_writer");
	Ref<Image> image = Image::create_empty(8, 6, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0, 0.5));
	const PackedByteArray original = image->get_data();
	{
		SimpleVideoWriter writer;
		ERR_PRINT_OFF;
		CHECK(writer.write_frame(image) == ERR_UNCONFIGURED);
		CHECK(writer.open(files.file("missing/video.avi"), Size2i(8, 6), 30) == ERR_CANT_OPEN);
		CHECK(writer.open(files.file("zero_fps.avi"), Size2i(8, 6), 0) == ERR_INVALID_PARAMETER);
		CHECK_FALSE(FileAccess::exists(files.file("zero_fps.avi")));
		ERR_PRINT_ON;
		REQUIRE(writer.open(files.file("video.avi"), Size2i(8, 6), 30) == OK);
		ERR_PRINT_OFF;
		CHECK(writer.write_frame(Ref<Image>()) == ERR_INVALID_PARAMETER);
		ERR_PRINT_ON;
		CHECK(writer.get_frame_count() == 0);
		writer.set_quality(0.9);
		REQUIRE(writer.write_frame(image) == OK);
		Ref<Image> rgb = Image::create_empty(8, 6, false, Image::FORMAT_RGB8);
		rgb->fill(Color(0, 1, 0));
		REQUIRE(writer.write_frame(rgb) == OK);
		CHECK(writer.get_frame_count() == 2);
		// Destruction must finalize the file even without an explicit close.
	}
	CHECK(image->get_format() == Image::FORMAT_RGBA8);
	CHECK(image->get_data() == original);
	const PackedByteArray bytes = read_avi(files.file("video.avi"));
	const int header = find_tag(bytes, "avih");
	const int chunk = find_tag(bytes, "00db");
	const int index = find_tag(bytes, "idx1");
	REQUIRE(header >= 0);
	REQUIRE(chunk >= 0);
	REQUIRE(index >= 0);
	CHECK(decode_uint32(bytes.ptr() + header + 8) == 1000000 / 30);
	CHECK(decode_uint32(bytes.ptr() + header + 24) == 2);
	CHECK(decode_uint32(bytes.ptr() + header + 40) == 8);
	CHECK(decode_uint32(bytes.ptr() + header + 44) == 6);
	const int jpeg_size = decode_uint32(bytes.ptr() + chunk + 4);
	REQUIRE(jpeg_size > 0);
	REQUIRE(chunk + 8 + jpeg_size <= bytes.size());
	Ref<Image> decoded;
	decoded.instantiate();
	REQUIRE(decoded->load_jpg_from_buffer(bytes.slice(chunk + 8, chunk + 8 + jpeg_size)) == OK);
	CHECK(decoded->get_size() == Size2i(8, 6));
	CHECK(decoded->get_pixel(4, 3).r > 0.9);
	CHECK(decoded->get_pixel(4, 3).g < 0.1);
	CHECK(decode_uint32(bytes.ptr() + index + 4) == 32);
	CHECK(index + 40 == bytes.size());
	SimpleVideoWriter empty;
	REQUIRE(empty.open(files.file("empty.avi"), Size2i(8, 6), 24) == OK);
	empty.close();
	empty.close();
	read_avi(files.file("empty.avi"));
}

TEST_CASE("[SPX] Movie merge validates inputs and preserves them on failure") {
	TestSpx::TempDirectory files("spx_merge_validation");
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	const String video = files.file("video.avi");
	const String audio = files.file("audio.avi");
	const String output = files.file("merged.avi");
	CHECK(processor.validate_merge_request(video, audio, output) == ERR_FILE_NOT_FOUND);
	files.write("video.avi", "invalid video");
	CHECK(processor.validate_merge_request(video, audio, output) == ERR_FILE_NOT_FOUND);
	files.write("audio.avi", "");
	CHECK(processor.validate_merge_request(video, audio, output) == ERR_FILE_CORRUPT);
	files.write("audio.avi", "invalid audio");
	CHECK(processor.validate_merge_request(video, audio, files.file("missing/result.avi")) == ERR_FILE_BAD_PATH);
	CHECK(processor.validate_merge_request(video, audio, output) == OK);
	files.write("video.avi", "");
	CHECK(processor.validate_merge_request(video, audio, output) == ERR_FILE_CORRUPT);
	files.write("video.avi", "invalid video");
	auto result = processor.merge_files(video, audio, output);
	CHECK(result.error_code == ERR_FILE_CORRUPT);
	CHECK_FALSE(result.error_message.is_empty());
	CHECK_FALSE(result.intermediate_files_cleaned);
	CHECK_FALSE(FileAccess::exists(output));
	CHECK(FileAccess::get_file_as_string(video) == "invalid video");
	CHECK(FileAccess::get_file_as_string(audio) == "invalid audio");
	CHECK_FALSE(FileAccess::exists(output));
}

// Poll published counters with a deadline instead of assuming thread scheduling latency.
template <typename Predicate>
static bool wait_for(Predicate ready) {
	const uint64_t deadline = OS::get_singleton()->get_ticks_usec() + 3000000;
	while (!ready() && OS::get_singleton()->get_ticks_usec() < deadline) {
		OS::get_singleton()->delay_usec(1000);
	}
	return ready();
}

TEST_CASE("[SPX] Video session deadlines follow its configured AVI frame rate without drift") {
	IndependentVideoRecorder::RecordingConfig config;
	for (uint32_t fps : { 1, 24, 30, 60, 120 }) {
		INFO("FPS: ", fps);
		config.target_fps = fps;
		CHECK(config.frame_time_usec(0) == 0);
		CHECK(config.frame_time_usec(1) == 1000000 / fps);
		CHECK(config.frame_time_usec(fps) == 1000000);
		CHECK(config.frame_time_usec(uint64_t(fps) * 3600) == uint64_t(3600000000));
		for (uint32_t frame = 0; frame < fps * 2; ++frame) {
			const uint64_t time = config.frame_time_usec(frame);
			CHECK(time * fps <= uint64_t(frame) * 1000000);
			CHECK((time + 1) * fps > uint64_t(frame) * 1000000);
		}
	}
}

TEST_CASE("[SPX] Video recorder writes new and repeated frames and joins its worker") {
	TestSpx::TempDirectory files("spx_video_thread");
	ThreadSafeFrameBuffer buffer;
	IndependentVideoRecorder recorder;
	CHECK_FALSE(recorder.is_recording());
	CHECK(recorder.get_repeat_frame_ratio() == 0);
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_UNCONFIGURED);
	CHECK(recorder.initialize(nullptr, files.file("video.avi")) == ERR_INVALID_PARAMETER);
	CHECK(recorder.initialize(&buffer, "") == ERR_INVALID_PARAMETER);
	CHECK(recorder.initialize(&buffer, files.file("missing/video.avi")) == ERR_CANT_OPEN);
	CHECK(recorder.start_recording() == ERR_UNCONFIGURED);
	ERR_PRINT_ON;
	IndependentVideoRecorder::RecordingConfig config;
	config.video_width = 4;
	config.video_height = 4;
	config.jpeg_quality = 0.9;
	ERR_PRINT_OFF;
	for (uint32_t invalid_fps : { 0, 121 }) {
		config.target_fps = invalid_fps;
		CHECK(recorder.initialize(&buffer, files.file("invalid.avi"), config) == ERR_INVALID_PARAMETER);
		CHECK_FALSE(FileAccess::exists(files.file("invalid.avi")));
	}
	ERR_PRINT_ON;
	config.target_fps = 10;
	REQUIRE(recorder.initialize(&buffer, files.file("video.avi"), config) == OK);
	REQUIRE(recorder.start_recording() == OK);
	CHECK(recorder.is_thread_running());
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_ALREADY_IN_USE);
	auto changed = config;
	changed.target_fps = 24;
	CHECK(recorder.initialize(&buffer, files.file("changed.avi"), changed) == ERR_ALREADY_IN_USE);
	CHECK_FALSE(FileAccess::exists(files.file("changed.avi")));
	ERR_PRINT_ON;
	Ref<Image> frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	frame->fill(Color(1, 0, 0));
	buffer.update_frame(frame, 1);
	REQUIRE(wait_for([&] { return recorder.get_statistics().repeated_frames_count >= 2; }));
	Ref<Image> second = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	second->fill(Color(0, 1, 0));
	buffer.update_frame(second, 2);
	REQUIRE(wait_for([&] { return recorder.get_statistics().new_frames_count == 2; }));
	ERR_PRINT_OFF;
	recorder.reset_statistics();
	ERR_PRINT_ON;
	CHECK(recorder.get_statistics().new_frames_count == 2);
	recorder.stop_recording();
	recorder.stop_recording();
	CHECK_FALSE(recorder.is_recording());
	CHECK_FALSE(recorder.is_thread_running());
	auto stats = recorder.get_statistics();
	CHECK(stats.total_recorded_frames == stats.new_frames_count + stats.repeated_frames_count);
	CHECK(stats.last_game_frame_sequence == 2);
	REQUIRE(stats.total_recorded_frames > 0);
	// A 30 FPS worker would emit these frames before the configured 10 FPS deadlines.
	// Only enforce a lower bound: slow CI scheduling must not make this test flaky.
	CHECK(stats.recording_duration_us >= config.frame_time_usec(stats.total_recorded_frames - 1));
	CHECK(recorder.get_repeat_frame_ratio() == doctest::Approx(double(stats.repeated_frames_count) / stats.total_recorded_frames));
	PackedByteArray bytes = read_avi(files.file("video.avi"));
	const int avih = find_tag(bytes, "avih");
	REQUIRE(avih >= 0);
	CHECK(decode_uint32(bytes.ptr() + avih + 24) == stats.total_recorded_frames);
	CHECK(decode_uint32(bytes.ptr() + avih + 8) == config.frame_time_usec(1));
	const int stream = find_tag(bytes, "vids");
	REQUIRE(stream >= 0);
	CHECK(decode_uint32(bytes.ptr() + stream + 20) == 1);
	CHECK(decode_uint32(bytes.ptr() + stream + 24) == config.target_fps);
	CHECK(recorder.get_debug_info().contains("Stopped"));
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_UNCONFIGURED);
	ERR_PRINT_ON;
	// A new session must reopen the file and reset both the clock and frame identity.
	config.target_fps = 1;
	REQUIRE(recorder.initialize(&buffer, files.file("restarted.avi"), config) == OK);
	CHECK(recorder.get_statistics().total_recorded_frames == 0);
	const uint64_t restart_start = OS::get_singleton()->get_ticks_usec();
	REQUIRE(recorder.start_recording() == OK);
	REQUIRE(wait_for([&] { return recorder.get_statistics().total_recorded_frames >= 1; }));
	recorder.stop_recording();
	// Include startup and polling so a late observation cannot hide a full-frame sleep.
	// Half a second leaves ample scheduling slack without accepting the old one-second wait.
	CHECK(OS::get_singleton()->get_ticks_usec() - restart_start < 500000);
	CHECK_FALSE(recorder.is_thread_running());
	CHECK(recorder.get_statistics().new_frames_count == 1);
	const auto restarted = read_avi(files.file("restarted.avi"));
	const int restarted_stream = find_tag(restarted, "vids");
	REQUIRE(restarted_stream >= 0);
	CHECK(decode_uint32(restarted.ptr() + restarted_stream + 24) == config.target_fps);
	CHECK(decode_uint32(restarted.ptr() + restarted_stream + 32) >= 1);
	recorder.reset_statistics();
	CHECK(recorder.get_statistics().total_recorded_frames == 0);
}

TEST_CASE("[SceneTree][SPX] Audio capture saturates PCM, preserves the master mix and detaches its effect") {
	TestSpx::TempDirectory files("spx_audio_capture");
	TestSpx::EngineScene scene;
	auto *server = AudioServer::get_singleton();
	const int before = server->get_bus_effect_count(0);
	HybridAudioDriver driver;
	REQUIRE(driver.init(100) == OK);
	CHECK(server->get_bus_effect_count(0) == before + 1);
	CHECK(driver.get_mix_rate() == 100);
	ERR_PRINT_OFF;
	CHECK(driver.init(100) == ERR_ALREADY_IN_USE);
	driver.register_audio_recorder(nullptr);
	ERR_PRINT_ON;
	IndependentAudioRecorder first, second, absent;
	driver.register_audio_recorder(&first);
	driver.register_audio_recorder(&first); // Duplicate registration must not duplicate PCM.
	driver.register_audio_recorder(&second);
	driver.unregister_audio_recorder(&absent);
	driver.unregister_audio_recorder(nullptr);
	CHECK(driver.get_registered_recorder_count() == 2);
	driver.unregister_audio_recorder(&second);
	CHECK(driver.get_registered_recorder_count() == 1);
	IndependentAudioRecorder::AudioConfig config;
	config.sample_rate = 100;
	config.chunk_size = 4;
	config.buffer_size_seconds = 1;
	REQUIRE(first.initialize(&driver, files.file("audio.avi"), config) == OK);
	REQUIRE(first.start_recording() == OK);
	Ref<AudioEffectInstance> effect = server->get_bus_effect(0, before)->instantiate();
	REQUIRE(effect.is_valid());
	CHECK_FALSE(effect->process_silence());
	AudioFrame input[4] = { AudioFrame(0.25, -0.5), AudioFrame(-0.25, 0.5), AudioFrame(1, -1), AudioFrame(2, -2) };
	AudioFrame silence[4] = {};
	AudioFrame output[4];
	effect->process(silence, output, 4); // Disabled capture must not enqueue these frames.
	CHECK_FALSE(driver.is_recording_enabled());
	driver.enable_recording(true);
	effect->process(input, output, 4);
	for (int i = 0; i < 4; ++i) {
		CHECK(output[i].left == input[i].left);
		CHECK(output[i].right == input[i].right);
	}
	REQUIRE(wait_for([&] { return first.get_statistics().total_chunks_recorded >= 1; }));
	driver.enable_recording(false);
	effect->process(silence, output, 4);
	driver.enable_recording(true);
	driver.capture_audio_data(nullptr, 1);
	int32_t unused[2] = {};
	driver.capture_audio_data(unused, 0);
	effect->process(input, output, 4);
	REQUIRE(wait_for([&] { return first.get_statistics().total_chunks_recorded >= 2; }));
	Ref<AudioEffectAmplify> project_effect;
	project_effect.instantiate();
	server->add_bus_effect(0, project_effect);
	driver.finish();
	CHECK(server->get_bus_effect_count(0) == before + 1);
	CHECK(server->get_bus_effect(0, before) == project_effect);
	server->remove_bus_effect(0, before);
	CHECK(server->get_bus_effect_count(0) == before);
	CHECK(driver.get_registered_recorder_count() == 0);
	CHECK_FALSE(driver.is_recording_enabled());
	effect->process(input, output, 4); // Detached instances must remain safe passthroughs.
	CHECK(output[0].right == input[0].right);
	driver.finish();
	first.stop_recording();
	CHECK(first.get_statistics().total_chunks_recorded == 2);
	const auto bytes = read_avi(files.file("audio.avi"));
	int chunk = find_tag(bytes, "00wb");
	REQUIRE(chunk >= 0);
	for (int i = 0; i < 2; i++) {
		REQUIRE(chunk + 40 <= bytes.size());
		CHECK(memcmp(bytes.ptr() + chunk, "00wb", 4) == 0);
		CHECK(decode_uint32(bytes.ptr() + chunk + 4) == 32);
		CHECK(decode_uint32(bytes.ptr() + chunk + 8) == uint32_t(536870912));
		CHECK(decode_uint32(bytes.ptr() + chunk + 12) == uint32_t(-1073741824));
		CHECK(decode_uint32(bytes.ptr() + chunk + 16) == uint32_t(-536870912));
		CHECK(decode_uint32(bytes.ptr() + chunk + 20) == uint32_t(1073741824));
		CHECK(decode_uint32(bytes.ptr() + chunk + 24) == uint32_t(INT32_MAX));
		CHECK(decode_uint32(bytes.ptr() + chunk + 28) == uint32_t(INT32_MIN));
		CHECK(decode_uint32(bytes.ptr() + chunk + 32) == uint32_t(INT32_MAX));
		CHECK(decode_uint32(bytes.ptr() + chunk + 36) == uint32_t(INT32_MIN));
		chunk += 40;
	}

	REQUIRE(driver.init(100) == OK);
	auto detached_effect = server->get_bus_effect(0, before)->instantiate();
	REQUIRE(detached_effect.is_valid());
	server->remove_bus_effect(0, before); // The project can remove the effect before shutdown.
	driver.finish();
	detached_effect->process(input, output, 4);
	CHECK(output[0].left == input[0].left);
	CHECK(server->get_bus_effect_count(0) == before);
}

TEST_CASE("[Audio][SPX] Audio capture safely detaches after the audio server shuts down") {
	auto *server = AudioServer::get_singleton();
	REQUIRE(server != nullptr);
	HybridAudioDriver driver;
	REQUIRE(driver.init(8000) == OK);
	Ref<AudioEffectInstance> effect = server->get_bus_effect(0, server->get_bus_effect_count(0) - 1)->instantiate();
	REQUIRE(effect.is_valid());
	IndependentAudioRecorder recorder;
	driver.register_audio_recorder(&recorder);
	driver.enable_recording(true);

	// Shutdown can destroy AudioServer before the capture owner and retained effect instances.
	server->finish();
	memdelete(server);
	REQUIRE(AudioServer::get_singleton() == nullptr);
	driver.finish();
	CHECK_FALSE(driver.is_recording_enabled());
	CHECK(driver.get_registered_recorder_count() == 0);
	AudioFrame input[2] = { AudioFrame(0.25, -0.5), AudioFrame(-0.25, 0.5) };
	AudioFrame output[2];
	effect->process(input, output, 2);
	for (int i = 0; i < 2; ++i) {
		CHECK(output[i].left == input[i].left);
		CHECK(output[i].right == input[i].right);
	}
	driver.finish(); // Explicit cleanup and the destructor remain idempotent.
	ERR_PRINT_OFF;
	CHECK(driver.init(8000) == ERR_UNCONFIGURED);
	ERR_PRINT_ON;
}

TEST_CASE("[SceneTree][SPX] Audio recorder consumes captured PCM and closes the worker file") {
	TestSpx::TempDirectory files("spx_audio_thread");
	TestSpx::EngineScene scene;
	HybridAudioDriver driver;
	REQUIRE(driver.init(8000) == OK);
	IndependentAudioRecorder recorder;
	CHECK_FALSE(recorder.is_recording());
	CHECK(recorder.get_buffer_usage_ratio() == 0);
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_UNCONFIGURED);
	CHECK(recorder.initialize(nullptr, files.file("audio.avi")) == ERR_INVALID_PARAMETER);
	CHECK(recorder.initialize(&driver, "") == ERR_INVALID_PARAMETER);
	CHECK(recorder.initialize(&driver, files.file("missing/audio.avi")) == ERR_CANT_OPEN);
	ERR_PRINT_ON;
	IndependentAudioRecorder::AudioConfig config;
	config.sample_rate = 8000;
	config.channels = 2;
	config.chunk_size = 80;
	config.buffer_size_seconds = 1;
	recorder.update_config(config);
	REQUIRE(recorder.initialize(&driver, files.file("audio.avi"), config) == OK);
	driver.register_audio_recorder(&recorder);
	driver.register_audio_recorder(&recorder);
	CHECK(driver.get_registered_recorder_count() == 1);
	REQUIRE(recorder.start_recording() == OK);
	REQUIRE(wait_for([&] { return recorder.get_statistics().buffer_underruns > 0; }));
	recorder.on_audio_output(nullptr, 1);
	recorder.on_audio_output(nullptr, 0);
	Vector<int32_t> oversized;
	oversized.resize(32768);
	oversized.fill(0);
	recorder.on_audio_output(oversized.ptr(), oversized.size() / 2);
	CHECK(recorder.get_statistics().buffer_overruns == 1);
	CHECK(recorder.get_buffer_usage_ratio() >= 0);
	CHECK(recorder.get_buffer_usage_ratio() <= 1);
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_ALREADY_IN_USE);
	auto changed = config;
	changed.sample_rate = 16000;
	recorder.update_config(changed);
	ERR_PRINT_ON;
	CHECK(recorder.get_config().sample_rate == 8000);
	driver.enable_recording(true);
	Vector<int32_t> samples;
	samples.resize(80 * 2 * 4);
	samples.fill(0x10000000);
	driver.capture_audio_data(samples.ptr(), 80 * 4);
	REQUIRE(wait_for([&] { return recorder.get_statistics().total_chunks_recorded >= 4; }));
	recorder.stop_recording();
	CHECK_FALSE(recorder.is_recording());
	CHECK_FALSE(recorder.is_thread_running());
	auto stats = recorder.get_statistics();
	CHECK(stats.total_chunks_recorded == 4);
	CHECK(stats.total_samples_recorded == 320);
	CHECK(stats.recording_duration_us > 0);
	CHECK_FALSE(recorder.has_audio_data());
	auto bytes = read_avi(files.file("audio.avi"));
	int data = find_tag(bytes, "00wb");
	REQUIRE(data >= 0);
	CHECK(decode_uint32(bytes.ptr() + data + 4) == 80 * 2 * 4);
	CHECK(decode_uint32(bytes.ptr() + data + 8) == 0x10000000);
	CHECK(recorder.get_debug_info().contains("Stopped"));
	driver.unregister_audio_recorder(&recorder);
	driver.unregister_audio_recorder(nullptr);
	CHECK(driver.get_registered_recorder_count() == 0);
	recorder.stop_recording();
	recorder.reset_statistics();
	CHECK(recorder.get_statistics().total_chunks_recorded == 0);
}

// Check LIST boundaries recursively; searching for FourCC alone can miss broken sizes.
static void check_chunks(const PackedByteArray &bytes, uint32_t begin, uint32_t end) {
	uint32_t pos = begin;
	while (pos < end) {
		REQUIRE(pos + 8 <= end);
		uint32_t size = decode_uint32(bytes.ptr() + pos + 4);
		REQUIRE(uint64_t(pos) + 8 + size <= end);
		if (memcmp(bytes.ptr() + pos, "LIST", 4) == 0) {
			REQUIRE(size >= 4);
			check_chunks(bytes, pos + 12, pos + 8 + size);
		}
		pos += 8 + size + (size & 1);
	}
	CHECK(pos == end);
}

static void check_avi_index(const PackedByteArray &bytes, int expected_count) {
	check_chunks(bytes, 12, bytes.size());
	const int movi = find_tag(bytes, "movi"), idx = find_tag(bytes, "idx1");
	REQUIRE(movi >= 0);
	REQUIRE(idx >= 0);
	REQUIRE(decode_uint32(bytes.ptr() + idx + 4) == expected_count * 16);
	for (int i = 0; i < expected_count; ++i) {
		const uint8_t *entry = bytes.ptr() + idx + 8 + i * 16;
		uint32_t offset = decode_uint32(entry + 8), size = decode_uint32(entry + 12);
		REQUIRE(uint64_t(movi) + offset + 8 + size <= uint64_t(idx));
		CHECK(memcmp(entry, bytes.ptr() + movi + offset, 4) == 0);
		CHECK(size == decode_uint32(bytes.ptr() + movi + offset + 4));
	}
}

// One video frame and one stereo PCM chunk represent the same 1/30-second interval.
static void write_avi_pair(const String &video, const String &audio, int frame_count = 2, int fps = 30, int sample_rate = 48000) {
	Ref<SimpleVideoWriter> video_writer;
	video_writer.instantiate();
	Ref<SimpleAudioWriter> audio_writer;
	audio_writer.instantiate();
	REQUIRE(video_writer->open(video, Size2i(4, 4), fps, 0.85) == OK);
	REQUIRE(audio_writer->open(audio, sample_rate, 2) == OK);
	auto image = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	image->fill(Color(1, 0, 0));
	Vector<int32_t> pcm;
	const int samples_per_frame = sample_rate / fps;
	pcm.resize(samples_per_frame * 2);
	pcm.fill(0x12340000);
	for (int i = 0; i < frame_count; ++i) {
		REQUIRE(video_writer->write_frame(image) == OK);
		REQUIRE(audio_writer->write_audio_chunk(pcm.ptr(), samples_per_frame) == OK);
	}
	video_writer->close();
	audio_writer->close();
}

TEST_CASE("[SPX] AVI merger preserves frame payloads and converts PCM with valid indexes") {
	TestSpx::TempDirectory files("spx_merge");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	auto original = read_avi(video);
	check_avi_index(original, 2);
	check_avi_index(read_avi(audio), 2);
	PackedByteArray expected_pcm;
	expected_pcm.resize(1600 * 2 * 2);
	for (int i = 0; i < expected_pcm.size(); i += 2) {
		encode_uint16(0x1234, expected_pcm.ptrw() + i);
	}
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = true;
	config.enable_debug_output = true;
	processor.set_config(config);
	const String output = files.file("merged.avi");
	const auto result = processor.merge_files(video, audio, output);
	INFO(result.error_message);
	REQUIRE(result.error_code == OK);
	CHECK(result.output_file_path == output);
	CHECK_FALSE(result.intermediate_files_cleaned);
	auto bytes = read_avi(output);
	check_avi_index(bytes, 4);
	int avih = find_tag(bytes, "avih"), video_chunk = find_tag(bytes, "00db"), audio_chunk = find_tag(bytes, "01wb");
	REQUIRE(avih >= 0);
	REQUIRE(video_chunk >= 0);
	REQUIRE(audio_chunk >= 0);
	CHECK(decode_uint32(bytes.ptr() + avih + 32) == 2);
	const int original_chunk = find_tag(original, "00db");
	uint32_t frame_size = decode_uint32(original.ptr() + original_chunk + 4);
	REQUIRE(decode_uint32(bytes.ptr() + video_chunk + 4) == frame_size);
	CHECK(memcmp(bytes.ptr() + video_chunk + 8, original.ptr() + original_chunk + 8, frame_size) == 0);
	REQUIRE(decode_uint32(bytes.ptr() + audio_chunk + 4) == 1600 * 2 * 2);
	CHECK(memcmp(bytes.ptr() + audio_chunk + 8, expected_pcm.ptr(), expected_pcm.size()) == 0);
	config.keep_intermediate_files = false;
	processor.set_config(config);
	const auto cleaned = processor.merge_files(video, audio, files.file("final.avi"));
	REQUIRE(cleaned.error_code == OK);
	CHECK(cleaned.intermediate_files_cleaned);
	CHECK_FALSE(FileAccess::exists(video));
	CHECK_FALSE(FileAccess::exists(audio));
}

TEST_CASE("[SPX] AVI interleaving preserves unequal streams in timestamp order") {
	struct Case {
		int fps;
		int sample_rate;
		int audio_chunks;
	};
	for (const Case &test : { Case{ 30, 48000, 3 }, Case{ 30, 44100, 7 }, Case{ 60, 48000, 7 }, Case{ 25, 44100, 7 } }) {
		INFO(test.fps, " fps, ", test.sample_rate, " Hz, ", test.audio_chunks, " audio chunks");
		TestSpx::TempDirectory files("spx_interleave_order");
		const String video = files.file("video.avi"), audio = files.file("audio.avi");
		write_avi_pair(video, audio, 2, test.fps, test.sample_rate);
		SimpleAudioWriter writer;
		REQUIRE(writer.open(audio, test.sample_rate, 2) == OK);
		const int samples_per_chunk = test.sample_rate / 100; // 10 ms chunks.
		Vector<int32_t> pcm;
		pcm.resize(samples_per_chunk * 2);
		for (int i = 0; i < test.audio_chunks; ++i) {
			pcm.fill((i + 1) * 0x10000000);
			REQUIRE(writer.write_audio_chunk(pcm.ptr(), samples_per_chunk) == OK);
		}
		writer.close();
		const auto original_video = read_avi(video), original_audio = read_avi(audio);
		PostMergeProcessor processor;
		PostMergeProcessor::MergeConfig config;
		config.keep_intermediate_files = true;
		processor.set_config(config);
		const String output = files.file("merged.avi");
		REQUIRE(processor.merge_files(video, audio, output).error_code == OK);
		const auto bytes = read_avi(output);
		check_avi_index(bytes, 2 + test.audio_chunks);
		const int index = find_tag(bytes, "idx1"), movi = find_tag(bytes, "movi"), stream = find_tag(bytes, "auds");
		REQUIRE(stream > 0);
		CHECK(decode_uint32(bytes.ptr() + stream + 32) == test.audio_chunks * samples_per_chunk);
		int video_chunks = 0;
		int audio_chunks = 0;
		uint64_t previous_time = 0;
		bool previous_video = true;
		for (int i = 0; i < 2 + test.audio_chunks; ++i) {
			const uint8_t *entry = bytes.ptr() + index + 8 + i * 16;
			const bool is_video = memcmp(entry, "00db", 4) == 0;
			const uint64_t time = is_video ? uint64_t(video_chunks++) * (1000000 / test.fps) : uint64_t(audio_chunks) * 10000;
			CHECK(time >= previous_time);
			if (i > 0 && time == previous_time) {
				CHECK(previous_video); // Video precedes audio at equal timestamps, including 0.
				CHECK_FALSE(is_video);
			}
			previous_time = time;
			previous_video = is_video;
			if (is_video) {
				const uint8_t *source_entry = original_video.ptr() + find_tag(original_video, "idx1") + 8 + (video_chunks - 1) * 16;
				const uint32_t size = decode_uint32(source_entry + 12);
				REQUIRE(decode_uint32(entry + 12) == size);
				const uint32_t source = find_tag(original_video, "movi") + decode_uint32(source_entry + 8);
				const uint32_t target = movi + decode_uint32(entry + 8);
				CHECK(memcmp(bytes.ptr() + target + 8, original_video.ptr() + source + 8, size) == 0);
			} else {
				CHECK(memcmp(entry, "01wb", 4) == 0);
				REQUIRE(decode_uint32(entry + 12) == samples_per_chunk * 2 * 2);
				const uint32_t chunk = movi + decode_uint32(entry + 8);
				++audio_chunks;
				PackedByteArray expected_pcm;
				expected_pcm.resize(samples_per_chunk * 2 * 2);
				for (int sample = 0; sample < samples_per_chunk * 2; ++sample) {
					encode_uint16(audio_chunks * 0x1000, expected_pcm.ptrw() + sample * 2);
				}
				CHECK(memcmp(bytes.ptr() + chunk + 8, expected_pcm.ptr(), expected_pcm.size()) == 0);
			}
		}
		CHECK(video_chunks == 2);
		CHECK(audio_chunks == test.audio_chunks);
		CHECK(FileAccess::get_file_as_bytes(video) == original_video);
		CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	}
}

class RealtimeWriter : public ObsStyleMovieWriter {
public:
	using ObsStyleMovieWriter::get_audio_mix_rate;
	using ObsStyleMovieWriter::get_audio_speaker_mode;
	using ObsStyleMovieWriter::write_frame;
};

TEST_CASE("[SceneTree][SPX] Realtime recording captures its first frame and can restart cleanly") {
	TestSpx::TempDirectory files("spx_realtime");
	TestSpx::EngineScene scene;
	const int effects = AudioServer::get_singleton()->get_bus_effect_count(0);
	auto destroy = [](RealtimeWriter *value) { memdelete(value); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	ObsStyleMovieWriter::ObsRecordingConfig config;
	config.jpeg_quality = 0.75f;
	config.audio_buffer_seconds = 1;
	// A different preference must not relabel PCM captured at the server's live rate.
	config.audio_sample_rate = AudioServer::get_singleton()->get_mix_rate() == 48000 ? 44100 : 48000;
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	CHECK(writer->get_audio_mix_rate() == config.audio_sample_rate);
	CHECK(writer->get_audio_speaker_mode() == AudioServer::SPEAKER_MODE_STEREO);
	CHECK(writer->get_state_name() == "Uninitialized");
	CHECK(writer->handles_realtime_file("movie.AVI"));
	CHECK_FALSE(writer->handles_realtime_file("movie.png"));
	List<String> extensions;
	writer->get_supported_extensions(&extensions);
	REQUIRE(extensions.size() == 1);
	CHECK(extensions.front()->get() == "avi");
	CHECK(writer->write_frame(Ref<Image>(), nullptr) == ERR_UNCONFIGURED);
	Ref<Image> frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	frame->fill(Color(1, 0, 0));
	for (int run = 0; run < 2; ++run) {
		config.enable_post_merge = (run == 1);
		config.video_fps = run == 0 ? 24 : 60;
		const uint32_t caller_fps = run == 0 ? 60 : 0;
		config.keep_intermediate_files = true;
		config.enable_debug_output = (run == 1);
		writer->set_recording_config(config);
		const String path = files.file("capture_" + itos(run) + ".avi");
		REQUIRE(writer->begin_realtime(Size2i(4, 4), caller_fps, path) == OK);
		CHECK(writer->is_recording_active());
		CHECK(writer->get_state_name() == "Recording");
		REQUIRE(writer->write_frame(Ref<Image>(), nullptr) == OK);
		CHECK(writer->get_combined_statistics().game_frames_added == 0);
		CHECK(writer->get_combined_statistics().video_stats.total_recorded_frames == 0);
		ERR_PRINT_OFF;
		CHECK(writer->begin_realtime(Size2i(4, 4), 30, path) == ERR_INVALID_PARAMETER);
		auto changed = config;
		changed.video_fps = 120;
		writer->set_recording_config(changed);
		ERR_PRINT_ON;
		CHECK(writer->get_recording_config().video_fps == config.video_fps);
		if (run == 1) {
			auto effect = AudioServer::get_singleton()->get_bus_effect(0, effects)->instantiate();
			Vector<AudioFrame> input, output;
			input.resize(4096);
			output.resize(4096);
			input.fill(AudioFrame(0.25, -0.25));
			effect->process(input.ptr(), output.ptrw(), 4096);
			REQUIRE(wait_for([&] { return writer->get_combined_statistics().audio_stats.total_chunks_recorded > 0; }));
		}
		REQUIRE(writer->write_frame(frame, nullptr) == OK);
		REQUIRE(wait_for([&] { return writer->get_combined_statistics().video_stats.new_frames_count == 1; }));
		auto stats = writer->get_combined_statistics();
		CHECK(stats.game_frames_added == 1);
		CHECK(stats.total_recording_duration_us > 0);
		CHECK(writer->get_comprehensive_debug_info().contains("Recording"));
		writer->end_realtime();
		CHECK_FALSE(writer->is_recording_active());
		CHECK(writer->get_state_name() == "Uninitialized");
		CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
		auto video = read_avi(path + "_video.avi");
		const int header = find_tag(video, "avih");
		REQUIRE(header >= 0);
		CHECK(decode_uint32(video.ptr() + header + 8) == 1000000 / config.video_fps);
		const int stream = find_tag(video, "vids");
		REQUIRE(stream >= 0);
		CHECK(decode_uint32(video.ptr() + stream + 24) == config.video_fps);
		const uint32_t frames = decode_uint32(video.ptr() + header + 24);
		CHECK(frames >= stats.video_stats.total_recorded_frames);
		check_avi_index(video, frames);
		const auto audio = read_avi(path + "_audio.avi");
		const int audio_format = find_tag(audio, "strf");
		REQUIRE(audio_format >= 0);
		// Live capture is not resampled to the configured preference.
		CHECK(decode_uint32(audio.ptr() + audio_format + 12) == AudioServer::get_singleton()->get_mix_rate());
		if (run == 1) {
			auto merged = read_avi(path + "_merged.avi");
			CHECK(find_tag(merged, "00db") >= 0);
			CHECK(find_tag(merged, "01wb") >= 0);
		}
		writer->end_realtime();
	}
}

TEST_CASE("[SceneTree][SPX] Realtime capture tolerates a detached main viewport") {
	TestSpx::TempDirectory files("spx_detached_capture");
	TestSpx::EngineScene scene;
	auto *server = RenderingServer::get_singleton();
	auto *window = SceneTree::get_singleton()->get_root();
	const RID viewport = server->viewport_find_from_screen_attachment(DisplayServer::MAIN_WINDOW_ID);
	REQUIRE(viewport == window->get_viewport_rid());
	// Changing content scale refreshes the window's exact screen attachment on restoration.
	struct RestoreAttachment {
		Window *window;
		real_t scale;
		~RestoreAttachment() {
			window->set_content_scale_factor(scale * 2);
			window->set_content_scale_factor(scale);
		}
	} restore{ window, window->get_content_scale_factor() };
	auto destroy = [](RealtimeWriter *writer) { memdelete(writer); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	ObsStyleMovieWriter::ObsRecordingConfig config;
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	const String path = files.file("capture.avi");
	REQUIRE(writer->begin_realtime(Size2i(4, 4), 30, path) == OK);
	server->viewport_attach_to_screen(viewport, Rect2(), DisplayServer::INVALID_WINDOW_ID);
	REQUIRE_FALSE(server->viewport_find_from_screen_attachment(DisplayServer::MAIN_WINDOW_ID).is_valid());
	writer->add_realtime_frame();
	CHECK(writer->is_recording_active());
	CHECK(writer->get_combined_statistics().game_frames_added == 0);
	CHECK(writer->get_combined_statistics().video_stats.total_recorded_frames == 0);
	writer->end_realtime();
	CHECK_FALSE(writer->is_recording_active());
	check_avi_index(read_avi(path + "_video.avi"), 0);
}

TEST_CASE("[SceneTree][SPX] Realtime recording rejects invalid configuration before allocating workers") {
	TestSpx::TempDirectory files("spx_realtime_invalid");
	TestSpx::EngineScene scene;
	auto destroy = [](RealtimeWriter *value) { memdelete(value); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	for (int invalid = 0; invalid < 9; ++invalid) {
		ObsStyleMovieWriter::ObsRecordingConfig config;
		Size2i size(4, 4);
		switch (invalid) {
			case 0:
				size.x = 0;
				break;
			case 1:
				config.video_fps = 0;
				break;
			case 2:
				config.audio_sample_rate = 1;
				break;
			case 3:
				config.audio_channels = 1;
				break;
			case 4:
				config.jpeg_quality = 0;
				break;
			case 5:
				size.y = 0;
				break;
			case 6:
				config.video_fps = 121;
				break;
			case 7:
				config.audio_sample_rate = 192001;
				break;
			case 8:
				config.jpeg_quality = 1.1;
				break;
		}
		writer->set_recording_config(config);
		ERR_PRINT_OFF;
		CHECK(writer->begin_realtime(size, 30, files.file("invalid.avi")) == ERR_INVALID_PARAMETER);
		ERR_PRINT_ON;
		CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
		CHECK_FALSE(FileAccess::exists(files.file("invalid.avi_video.avi")));
	}
	ObsStyleMovieWriter::ObsRecordingConfig config;
	config.jpeg_quality = 0.95f;
	config.audio_buffer_seconds = 3;
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	const int effects = AudioServer::get_singleton()->get_bus_effect_count(0);
	ERR_PRINT_OFF;
	CHECK(writer->begin_realtime(Size2i(4, 4), 30, files.file("missing/invalid.avi")) != OK);
	ERR_PRINT_ON;
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	writer->end_realtime();
	const String path = files.file("after_video_failure.avi");
	REQUIRE(writer->begin_realtime(Size2i(4, 4), 30, path) == OK);
	writer->end_realtime();
	writer->end_realtime();
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	check_avi_index(read_avi(path + "_video.avi"), 0);
	check_avi_index(read_avi(path + "_audio.avi"), 0);
}

TEST_CASE("[SceneTree][SPX] Recording manager owns API sessions and command-line movie routes") {
	TestSpx::TempDirectory files("spx_recording_manager");
	TestSpx::EngineScene scene;
	auto *writer = dynamic_cast<ObsStyleMovieWriter *>(spx_find_realtime_movie_recorder("capture.avi"));
	REQUIRE(writer != nullptr);
	struct Restore {
		ObsStyleMovieWriter *writer;
		ObsStyleMovieWriter::ObsRecordingConfig config;
		Variant obs_mode;
		~Restore() {
			get_main_loop_phase_callback_bus().notify_movie_end();
			MovieRecorderManager::shutdown();
			MovieRecorderManager::initialize();
			writer->set_recording_config(config);
			ProjectSettings::get_singleton()->set_setting("movie_writer/obs_mode", obs_mode);
		}
	} restore{ writer, writer->get_recording_config(), ProjectSettings::get_singleton()->get_setting("movie_writer/obs_mode") };
	auto config = restore.config;
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	MovieRecorderManager::initialize();
	MovieRecorderManager::initialize();
	CHECK(MovieRecorderManager::is_initialized());
	CHECK_FALSE(MovieRecorderManager::has_active_instance());
	CHECK(MovieRecorderManager::get_recording_duration() == 0);
	CHECK(MovieRecorderManager::stop_recording() == ERR_INVALID_PARAMETER);
	MovieRecorderManager::RecordingConfig request;
	request.video_width = 4;
	request.video_height = 4;
	request.video_fps = 30;
	ERR_PRINT_OFF;
	CHECK(MovieRecorderManager::start_recording(request) == ERR_INVALID_PARAMETER);
	request.output_path = files.file("unsupported.png");
	CHECK(MovieRecorderManager::start_recording(request) == ERR_FILE_UNRECOGNIZED);
	request.output_path = files.file("missing/capture.avi");
	CHECK(MovieRecorderManager::start_recording(request) != OK);
	ERR_PRINT_ON;
	CHECK_FALSE(MovieRecorderManager::has_active_instance());
	request.output_path = files.file("api.avi");
	REQUIRE(MovieRecorderManager::start_recording(request) == OK);
	CHECK(MovieRecorderManager::is_recording());
	CHECK(MovieRecorderManager::has_active_instance());
	CHECK(MovieRecorderManager::get_current_output_path() == request.output_path);
	CHECK(MovieRecorderManager::get_recording_duration() >= 0);
	CHECK(MovieRecorderManager::start_recording(request) == ERR_ALREADY_IN_USE);
	REQUIRE(MovieRecorderManager::stop_recording() == OK);
	CHECK_FALSE(MovieRecorderManager::is_recording());
	CHECK_FALSE(MovieRecorderManager::has_active_instance());
	CHECK(FileAccess::exists(request.output_path + "_video.avi"));
	auto &bus = get_main_loop_phase_callback_bus();
	ProjectSettings::get_singleton()->set_setting("movie_writer/obs_mode", true);
	const String path = files.file("command.avi");
	REQUIRE(bus.resolve_movie_route(path) == OK);
	CHECK(bus.is_movie_route_claimed());
	CHECK(bus.movie_route_requires_live_audio());
	REQUIRE(bus.notify_movie_begin(Size2i(4, 4), 0, path) == OK);
	CHECK(MovieRecorderManager::is_recording());
	CHECK(MovieRecorderManager::get_current_output_path() == path);
	ERR_PRINT_OFF;
	bus.notify_movie_frame(); // The headless renderer has no image, but the route stays active.
	ERR_PRINT_ON;
	CHECK(MovieRecorderManager::is_recording());
	bus.notify_movie_end();
	CHECK_FALSE(MovieRecorderManager::is_recording());
	CHECK_FALSE(bus.is_movie_route_resolved());
	request.video_width = 0;
	request.video_height = 0;
	request.video_fps = 0;
	request.output_path = files.file("defaults.avi");
	REQUIRE(MovieRecorderManager::start_recording(request) == OK);
	REQUIRE(MovieRecorderManager::stop_recording() == OK);
	const auto defaults = read_avi(request.output_path + "_video.avi");
	const int header = find_tag(defaults, "avih");
	REQUIRE(header > 0);
	CHECK(decode_uint32(defaults.ptr() + header + 40) > 0);
	CHECK(decode_uint32(defaults.ptr() + header + 44) > 0);
	request.video_width = 4;
	request.video_height = 0; // One valid dimension must not bypass size resolution.
	request.output_path = files.file("shutdown.avi");
	REQUIRE(MovieRecorderManager::start_recording(request) == OK);
	MovieRecorderManager::shutdown(); // Finalize an API-owned session, not only an idle manager.
	CHECK_FALSE(MovieRecorderManager::is_recording());
	CHECK_FALSE(MovieRecorderManager::has_active_instance());
	CHECK(MovieRecorderManager::get_recording_duration() == 0);
	const auto stopped = read_avi(request.output_path + "_video.avi");
	const int stopped_header = find_tag(stopped, "avih");
	REQUIRE(stopped_header > 0);
	CHECK(decode_uint32(stopped.ptr() + stopped_header + 40) > 0);
	CHECK(decode_uint32(stopped.ptr() + stopped_header + 44) > 0);
	MovieRecorderManager::shutdown();
	CHECK_FALSE(MovieRecorderManager::is_initialized());
	MovieRecorderManager::initialize();
	CHECK(MovieRecorderManager::is_initialized());
}

TEST_CASE("[Audio][SPX] Recording manager reuses the command-line size without a display server") {
	TestSpx::TempDirectory files("spx_recording_no_display");
	REQUIRE(DisplayServer::get_singleton() == nullptr);
	REQUIRE(RenderingServer::get_singleton() == nullptr);
	REQUIRE_FALSE(MovieRecorderManager::is_recording());
	REQUIRE(AudioServer::get_singleton() != nullptr);
	if (DisplayServer::get_singleton() != nullptr || RenderingServer::get_singleton() != nullptr ||
			MovieRecorderManager::is_recording() || AudioServer::get_singleton() == nullptr) {
		return;
	}
	auto *writer = dynamic_cast<ObsStyleMovieWriter *>(spx_find_realtime_movie_recorder("capture.avi"));
	REQUIRE(writer != nullptr);
	if (writer == nullptr) {
		return;
	}
	struct Restore {
		ObsStyleMovieWriter *writer;
		ObsStyleMovieWriter::ObsRecordingConfig config;
		Variant obs_mode;
		bool initialized;
		~Restore() {
			get_main_loop_phase_callback_bus().notify_movie_end();
			MovieRecorderManager::shutdown();
			if (initialized) {
				MovieRecorderManager::initialize();
			}
			writer->set_recording_config(config);
			ProjectSettings::get_singleton()->set_setting("movie_writer/obs_mode", obs_mode);
		}
	} restore{ writer, writer->get_recording_config(), ProjectSettings::get_singleton()->get_setting("movie_writer/obs_mode"), MovieRecorderManager::is_initialized() };
	auto config = restore.config;
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	ProjectSettings::get_singleton()->set_setting("movie_writer/obs_mode", true);
	MovieRecorderManager::initialize();
	const int effects = AudioServer::get_singleton()->get_bus_effect_count(0);
	auto &bus = get_main_loop_phase_callback_bus();
	const String command = files.file("command.avi");
	Error error = bus.resolve_movie_route(command);
	REQUIRE(error == OK);
	if (error != OK) {
		return;
	}
	error = bus.notify_movie_begin(Size2i(4, 6), 30, command);
	REQUIRE(error == OK);
	if (error != OK) {
		return;
	}
	bus.notify_movie_end();
	CHECK_FALSE(MovieRecorderManager::is_recording());
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	MovieRecorderManager::RecordingConfig request;
	request.output_path = files.file("fallback.avi");
	request.video_width = 0;
	request.video_height = 0;
	error = MovieRecorderManager::start_recording(request);
	REQUIRE(error == OK);
	if (error != OK) {
		return;
	}
	error = MovieRecorderManager::stop_recording();
	REQUIRE(error == OK);
	if (error != OK) {
		return;
	}
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	const auto video = FileAccess::get_file_as_bytes(request.output_path + "_video.avi");
	REQUIRE(video.size() >= 12);
	if (video.size() < 12) {
		return;
	}
	CHECK(find_tag(video, "RIFF") == 0);
	CHECK(find_tag(video, "AVI ") == 8);
	CHECK(decode_uint32(video.ptr() + 4) == video.size() - 8);
	const int header = find_tag(video, "avih");
	REQUIRE(header > 0);
	REQUIRE((header >= 0 && uint64_t(header) + 48 <= uint64_t(video.size())));
	if (header <= 0 || uint64_t(header) + 48 > uint64_t(video.size())) {
		return;
	}
	const int index = find_tag(video, "idx1");
	REQUIRE((index >= 0 && uint64_t(index) + 8 == uint64_t(video.size())));
	if (index < 0 || uint64_t(index) + 8 != uint64_t(video.size())) {
		return;
	}
	CHECK(decode_uint32(video.ptr() + index + 4) == 0);
	CHECK(decode_uint32(video.ptr() + header + 40) == 4);
	CHECK(decode_uint32(video.ptr() + header + 44) == 6);
	CHECK_FALSE(MovieRecorderManager::has_active_instance());
}

TEST_CASE("[SPX] AVI recovery scans unindexed streams and rejects malformed headers") {
	TestSpx::TempDirectory files("spx_avi_recovery");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio, 20);
	for (const String &path : { video, audio }) {
		auto bytes = read_avi(path);
		int index = find_tag(bytes, "idx1");
		REQUIRE(index > 0);
		bytes.resize(index);
		encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
		write_avi(path, bytes);
	}
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = true;
	config.enable_debug_output = true;
	processor.set_config(config);
	String output = files.file("recovered.avi");
	auto result = processor.merge_files(video, audio, output);
	INFO(result.error_message);
	REQUIRE(result.error_code == OK);
	check_avi_index(read_avi(output), 40);
	auto valid = read_avi(video);
	for (const char *tag : { "AVI ", "hdrl", "avih", "strh", "vids", "movi" }) {
		auto bad = valid;
		const int offset = find_tag(bad, tag);
		REQUIRE(offset >= 0);
		bad.set(offset, 'X');
		String broken = files.file("broken.avi");
		write_avi(broken, bad);
		ERR_PRINT_OFF;
		auto result = processor.merge_files(broken, audio, files.file("invalid.avi"));
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(result.intermediate_files_cleaned);
	}
	files.write("bad_audio.avi", "RIFFxxxxxxxxxxxxxxxx");
	ERR_PRINT_OFF;
	CHECK(processor.merge_files(video, files.file("bad_audio.avi"), files.file("bad.avi")).error_code == ERR_FILE_CORRUPT);
	ERR_PRINT_ON;
	CHECK(processor.merge_files(video, audio, files.file("missing/output.avi")).error_code == ERR_FILE_BAD_PATH);
	CHECK(FileAccess::exists(video));
	CHECK(FileAccess::exists(audio));
}

TEST_CASE("[SPX] AVI merger rejects damaged indexes without consuming input streams") {
	TestSpx::TempDirectory files("spx_avi_index");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original = read_avi(video);
	const int index = find_tag(original, "idx1");
	REQUIRE(index > 0);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = false;
	processor.set_config(config);

	for (int damage = 0; damage < 7; ++damage) {
		INFO("Index damage case: ", damage);
		auto bytes = original;
		switch (damage) {
			case 0:
				// The first entry remains valid; a bad second offset must not merge a partial index.
				encode_uint32(0, bytes.ptrw() + index + 8 + 16 + 8);
				break;
			case 1:
				encode_uint32(31, bytes.ptrw() + index + 4); // Fractional index entry.
				break;
			case 2:
				bytes.resize(bytes.size() - 1); // Truncated final entry.
				encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
				break;
			case 3:
				encode_uint32(UINT32_MAX, bytes.ptrw() + index + 16); // Offset beyond movi.
				break;
			case 4:
				encode_uint32(UINT32_MAX, bytes.ptrw() + index + 20); // Size overflows a 32-bit end offset.
				break;
			case 5:
				bytes.set(index + 8, 'X'); // Index tag disagrees with the actual chunk.
				break;
			case 6:
				encode_uint32(1, bytes.ptrw() + index + 20); // Index size disagrees with the actual chunk.
				break;
		}
		write_avi(video, bytes);
		ERR_PRINT_OFF;
		auto result = processor.merge_files(video, audio, files.file("invalid.avi"));
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK(FileAccess::exists(video));
		CHECK(FileAccess::exists(audio));
		CHECK_FALSE(FileAccess::exists(files.file("invalid.avi")));
	}
}

TEST_CASE("[SPX] AVI merger rejects source truncation between parsing and interleaving") {
	TestSpx::TempDirectory files("spx_avi_changed_input");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	const int chunk = find_tag(original_video, "00db");
	REQUIRE(chunk > 0);
	const uint32_t payload_size = decode_uint32(original_video.ptr() + chunk + 4);
	REQUIRE(payload_size > 0);

	// The second index log marks a real I/O boundary: video parsing has closed its
	// handle, while audio parsing is still in progress. Change the file there to
	// exercise a concurrent writer deterministically, without racing test threads.
	struct ChangeParsedInput {
		PrintHandlerList handler;
		String path;
		PackedByteArray bytes;
		int indexes_seen = 0;
		bool changed = false;

		ChangeParsedInput(const String &p_path, const PackedByteArray &p_bytes) :
				path(p_path), bytes(p_bytes) {
			handler.userdata = this;
			handler.printfunc = [](void *userdata, const String &message, bool, bool) {
				auto &change = *static_cast<ChangeParsedInput *>(userdata);
				if (message.begins_with("PostMergeProcessor: Parsing ") && ++change.indexes_seen == 2) {
					auto file = FileAccess::open(change.path, FileAccess::WRITE);
					change.changed = file.is_valid() && file->store_buffer(change.bytes.ptr(), change.bytes.size());
				}
			};
			add_print_handler(&handler);
		}
		~ChangeParsedInput() { remove_print_handler(&handler); }
	};

	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.enable_debug_output = true;
	processor.set_config(config);
	for (int damage = 0; damage < 4; damage++) {
		INFO("Source change: ", damage);
		write_avi(video, original_video);
		auto changed = original_video;
		switch (damage) {
			case 0:
				changed.resize(chunk - 1); // The first chunk no longer exists.
				break;
			case 1:
				changed.resize(chunk + 7); // The chunk header is incomplete.
				break;
			case 2:
				encode_uint32(payload_size - 1, changed.ptrw() + chunk + 4); // The parsed size is stale.
				break;
			case 3:
				changed.resize(chunk + 8 + payload_size - 1); // The payload was truncated.
				break;
		}
		ChangeParsedInput change(video, changed);
		ERR_PRINT_OFF;
		const auto result = processor.merge_files(video, audio, files.file("output.avi"));
		ERR_PRINT_ON;
		REQUIRE(change.changed);
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK(result.error_message == "Failed to interleave video and audio data");
		CHECK(result.output_file_path.is_empty());
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK(FileAccess::get_file_as_bytes(video) == changed);
		CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	}
}

TEST_CASE("[SPX] AVI merger requires complete ordered indexes and preserves existing output") {
	TestSpx::TempDirectory files("spx_avi_index_order");
	const String video = files.file("video.avi"), audio = files.file("audio.avi"), output = files.file("merged.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	files.write("merged.avi", "existing recording");
	const auto original_output = FileAccess::get_file_as_bytes(output);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	for (bool damage_audio : { true, false }) {
		for (int damage = 0; damage < 4; ++damage) {
			INFO("Audio: ", damage_audio, ", index damage: ", damage);
			auto bytes = damage_audio ? original_audio : original_video;
			const int index = find_tag(bytes, "idx1");
			REQUIRE(index > 0);
			if (damage == 0) {
				// Duplicating a valid entry must not duplicate its payload.
				memcpy(bytes.ptrw() + index + 24, bytes.ptr() + index + 8, 16);
			} else if (damage == 1) {
				// A valid prefix must not silently drop the last payload.
				bytes.resize(index + 24);
				encode_uint32(16, bytes.ptrw() + index + 4);
				encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
			} else if (damage == 2) {
				// Individually valid entries must preserve their recording order.
				for (int i = 0; i < 16; ++i) {
					uint8_t first = bytes[index + 8 + i];
					bytes.set(index + 8 + i, bytes[index + 24 + i]);
					bytes.set(index + 24 + i, first);
				}
			} else {
				// A stale index size must not be copied into the merged file.
				encode_uint32(1, bytes.ptrw() + index + 20);
			}
			write_avi(video, damage_audio ? original_video : bytes);
			write_avi(audio, damage_audio ? bytes : original_audio);
			ERR_PRINT_OFF;
			auto result = processor.merge_files(video, audio, output);
			ERR_PRINT_ON;
			CHECK(result.error_code == ERR_FILE_CORRUPT);
			CHECK_FALSE(result.intermediate_files_cleaned);
			CHECK(FileAccess::get_file_as_bytes(output) == original_output);
			CHECK(FileAccess::get_file_as_bytes(video) == (damage_audio ? original_video : bytes));
			CHECK(FileAccess::get_file_as_bytes(audio) == (damage_audio ? bytes : original_audio));
		}
	}
}

TEST_CASE("[SPX] AVI merger rejects invalid timing and nested chunk boundaries") {
	TestSpx::TempDirectory files("spx_avi_headers");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	struct Damage {
		bool audio;
		const char *tag;
		int offset;
		uint32_t value;
	};
	const Damage cases[] = {
		{ false, "RIFF", 4, 0 },
		{ false, "hdrl", -4, 0 },
		{ false, "hdrl", -4, 8 },
		{ false, "avih", 4, 4 },
		{ false, "avih", 8, 0 }, // Zero frame duration formerly divided by zero.
		{ false, "avih", 32, 2 }, // A source file must have exactly one stream.
		{ false, "avih", 40, 0 },
		{ false, "avih", 44, 0 },
		{ false, "strl", -4, 2 },
		{ false, "strl", -4, 8 },
		{ false, "strh", 4, 4 },
		{ false, "strh", 4, UINT32_MAX },
		{ false, "strf", 4, UINT32_MAX },
		{ false, "strf", 4, 0 },
		{ true, "strf", 4, 0 },
		{ false, "vids", 20, 0 }, // scale
		{ false, "vids", 24, 0 }, // rate
		{ false, "movi", -4, UINT32_MAX },
		{ true, "auds", 44, 0 }, // Zero PCM frame size formerly divided by zero.
		{ true, "auds", 44, 3 }, // PCM requires whole 32-bit samples.
	};
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	const String output = files.file("invalid.avi");
	for (const Damage &damage : cases) {
		INFO("Invalid ", damage.tag, " field ", damage.offset, " = ", damage.value);
		auto bytes = damage.audio ? original_audio : original_video;
		const int tag = find_tag(bytes, damage.tag);
		REQUIRE(tag >= 0);
		encode_uint32(damage.value, bytes.ptrw() + tag + damage.offset);
		write_avi(video, damage.audio ? original_video : bytes);
		write_avi(audio, damage.audio ? bytes : original_audio);
		ERR_PRINT_OFF;
		auto result = processor.merge_files(video, audio, output);
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK_FALSE(FileAccess::exists(output));
		CHECK(FileAccess::get_file_as_bytes(video) == (damage.audio ? original_video : bytes));
		CHECK(FileAccess::get_file_as_bytes(audio) == (damage.audio ? bytes : original_audio));
	}
}

TEST_CASE("[SPX] AVI recovery rejects incomplete PCM frames and truncated unindexed payloads") {
	TestSpx::TempDirectory files("spx_avi_payloads");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original = read_avi(audio);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	for (bool indexed : { true, false }) {
		for (int damage = 0; damage < 3; ++damage) {
			INFO("Indexed: ", indexed, ", payload damage: ", damage);
			auto bytes = original;
			const int index = find_tag(bytes, "idx1");
			const int chunk = find_tag(bytes, "00wb");
			REQUIRE(index > 0);
			REQUIRE(chunk > 0);
			const uint32_t size = decode_uint32(bytes.ptr() + chunk + 4);
			if (damage == 0) {
				// Both header and index agree, but the payload ends inside an int32 sample.
				encode_uint32(size - 1, bytes.ptrw() + chunk + 4);
				encode_uint32(size - 1, bytes.ptrw() + index + 20);
			} else if (damage == 1) {
				encode_uint32(bytes.size(), bytes.ptrw() + chunk + 4);
				encode_uint32(bytes.size(), bytes.ptrw() + index + 20);
			} else {
				// Matching tags still must identify PCM in an audio stream.
				bytes.set(chunk + 2, 'd');
				bytes.set(index + 10, 'd');
			}
			if (!indexed) {
				bytes.resize(index);
				encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
			}
			write_avi(audio, bytes);
			ERR_PRINT_OFF;
			auto result = processor.merge_files(video, audio, files.file("invalid.avi"));
			ERR_PRINT_ON;
			CHECK(result.error_code == ERR_FILE_CORRUPT);
			CHECK_FALSE(result.intermediate_files_cleaned);
			CHECK_FALSE(FileAccess::exists(files.file("invalid.avi")));
			CHECK(FileAccess::get_file_as_bytes(audio) == bytes);
		}
	}
}

TEST_CASE("[SPX] AVI merger rejects non-video payload tags even when the index agrees") {
	TestSpx::TempDirectory files("spx_avi_video_tags");
	const String video = files.file("video.avi"), audio = files.file("audio.avi"), output = files.file("merged.avi");
	write_avi_pair(video, audio);
	const auto original = read_avi(video), original_audio = read_avi(audio);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	for (bool indexed : { true, false }) {
		auto bytes = original;
		const int index = find_tag(bytes, "idx1"), chunk = find_tag(bytes, "00db");
		REQUIRE(index > 0);
		REQUIRE(chunk > 0);
		memcpy(bytes.ptrw() + chunk, "00wb", 4);
		memcpy(bytes.ptrw() + index + 8, "00wb", 4);
		if (!indexed) {
			bytes.resize(index);
			encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
		}
		write_avi(video, bytes);
		files.write("merged.avi", "previous recording");
		ERR_PRINT_OFF;
		const auto result = processor.merge_files(video, audio, output);
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK(FileAccess::get_file_as_string(output) == "previous recording");
		CHECK(FileAccess::get_file_as_bytes(video) == bytes);
		CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	}
}

TEST_CASE("[SPX] AVI parser keeps word padding inside the parent header") {
	TestSpx::TempDirectory files("spx_avi_padding");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original = read_avi(video);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	for (bool main_header : { true, false }) {
		auto bytes = original;
		const int child = find_tag(bytes, main_header ? "avih" : "strh");
		const int parent = find_tag(bytes, main_header ? "hdrl" : "strl");
		REQUIRE(child > 0);
		REQUIRE(parent > 0);
		const uint32_t size = main_header ? 57 : 49;
		encode_uint32(size, bytes.ptrw() + child + 4);
		encode_uint32(4 + 8 + size, bytes.ptrw() + parent - 4);
		write_avi(video, bytes);
		ERR_PRINT_OFF;
		auto result = processor.merge_files(video, audio, files.file("invalid.avi"));
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(FileAccess::exists(files.file("invalid.avi")));
	}
}

TEST_CASE("[SPX] AVI merger accepts empty video and audio streams") {
	TestSpx::TempDirectory files("spx_avi_empty");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	const String empty_video = files.file("empty_video.avi"), empty_audio = files.file("empty_audio.avi");
	write_avi_pair(video, audio);
	write_avi_pair(empty_video, empty_audio, 0);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = true;
	processor.set_config(config);
	for (int empty = 1; empty <= 3; ++empty) {
		const String output = files.file("merged.avi");
		auto result = processor.merge_files(empty & 1 ? empty_video : video, empty & 2 ? empty_audio : audio, output);
		INFO(result.error_message);
		REQUIRE(result.error_code == OK);
		check_avi_index(read_avi(output), empty == 3 ? 0 : 2);
	}
}

#ifndef WEB_ENABLED
class NativeWebMWriter : public MovieWriterWebM {
public:
	using MovieWriterWebM::get_audio_mix_rate;
	using MovieWriterWebM::get_audio_speaker_mode;
	using MovieWriterWebM::get_supported_extensions;
	using MovieWriterWebM::write_frame;
};

TEST_CASE("[SPX] WebM recorder consistently reports unavailable on native platforms") {
	TestSpx::TempDirectory files("spx_webm_native");
	auto *settings = ProjectSettings::get_singleton();
	const String key = "movie_writer/enable_web_auto_download";
	struct Restore {
		String key;
		Variant value;
		~Restore() { ProjectSettings::get_singleton()->set_setting(key, value); }
	} restore{ key, settings->get_setting(key) };
	for (const Variant &value : { Variant(), Variant(false), Variant(true) }) {
		settings->set_setting(key, value);
		auto destroy = [](NativeWebMWriter *writer) { memdelete(writer); };
		std::unique_ptr<NativeWebMWriter, decltype(destroy)> writer(memnew(NativeWebMWriter), destroy);
		CHECK(writer->get_audio_mix_rate() == 48000);
		CHECK(writer->get_audio_speaker_mode() == AudioServer::SPEAKER_MODE_STEREO);
		List<String> extensions;
		writer->get_supported_extensions(&extensions);
		REQUIRE(extensions.size() == 1);
		CHECK(extensions.front()->get() == "webm");
		CHECK_FALSE(writer->handles_realtime_file("capture.webm"));
		CHECK_FALSE(writer->handles_realtime_file("capture.avi"));
		CHECK(writer->begin_realtime(Size2i(4, 4), 30, files.file("capture.webm")) == ERR_UNAVAILABLE);
		writer->add_realtime_frame();
		CHECK(writer->write_frame(Ref<Image>(), nullptr) == ERR_UNAVAILABLE);
		writer->end_realtime();
		writer->end_realtime();
		CHECK_FALSE(FileAccess::exists(files.file("capture.webm")));
	}
}
#endif

TEST_CASE("[SPX] AVI merge preserves inputs on output failure and cleans up only after success") {
	TestSpx::TempDirectory files("spx_merge_cleanup");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = false;
	config.enable_debug_output = true;
	processor.set_config(config);
	// An existing directory passes parent validation but cannot be opened as a file.
	const String blocked = files.file("output.avi");
	REQUIRE(DirAccess::make_dir_absolute(blocked) == OK);
	auto result = processor.merge_files(video, audio, blocked);
	CHECK(result.error_code == ERR_FILE_CANT_WRITE);
	CHECK(result.error_message.contains("Cannot create output file"));
	CHECK_FALSE(result.intermediate_files_cleaned);
	CHECK(FileAccess::get_file_as_bytes(video) == original_video);
	CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	REQUIRE(DirAccess::remove_absolute(blocked) == OK);
	result = processor.merge_files(video, audio, blocked);
	REQUIRE(result.error_code == OK);
	CHECK(result.intermediate_files_cleaned);
	CHECK(result.output_file_path == blocked);
	check_avi_index(read_avi(blocked), 4);
	CHECK_FALSE(FileAccess::exists(video));
	CHECK_FALSE(FileAccess::exists(audio));
	// Cleanup must have identical behavior with diagnostics disabled.
	write_avi_pair(video, audio);
	config.enable_debug_output = false;
	processor.set_config(config);
	result = processor.merge_files(video, audio, blocked);
	REQUIRE(result.error_code == OK);
	CHECK(result.intermediate_files_cleaned);
	CHECK(result.output_file_path == blocked);
	check_avi_index(read_avi(blocked), 4);
	CHECK_FALSE(FileAccess::exists(video));
	CHECK_FALSE(FileAccess::exists(audio));
	config.enable_debug_output = true;
	processor.set_config(config);
#ifdef UNIX_ENABLED
	const auto expected_output = FileAccess::get_file_as_bytes(blocked);
	for (bool block_video : { true, false }) {
		INFO("Read-only source directory contains video: ", block_video);
		TestSpx::TempDirectory locked("spx_merge_readonly");
		// Restore before TempDirectory removes its contents, including on early returns.
		struct RestorePermissions {
			String path;
			~RestorePermissions() { FileAccess::set_unix_permissions(path, 0700); }
		} restore{ locked.path };
		const String locked_video = block_video ? locked.file("video.avi") : video;
		const String locked_audio = block_video ? audio : locked.file("audio.avi");
		write_avi_pair(locked_video, locked_audio);
		locked.write("permission_probe", "probe");
		const Error permissions = FileAccess::set_unix_permissions(locked.path, 0500);
		REQUIRE(permissions == OK);
		if (permissions != OK) {
			return;
		}
		if (DirAccess::remove_absolute(locked.file("permission_probe")) == OK) {
			WARN_MESSAGE(false, "Skipping read-only cleanup checks: filesystem or user bypasses directory permissions");
			continue;
		}
		result = processor.merge_files(locked_video, locked_audio, blocked);
		CHECK(result.error_code == OK); // A cleanup failure must not invalidate the recording.
		CHECK(result.output_file_path == blocked);
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK(FileAccess::get_file_as_bytes(blocked) == expected_output);
		const String retained = block_video ? locked_video : locked_audio;
		CHECK(FileAccess::get_file_as_bytes(retained) == (block_video ? original_video : original_audio));
		// Both deletions are attempted even if video cleanup fails first.
		CHECK_FALSE(FileAccess::exists(block_video ? locked_audio : locked_video));
		const Error restored = FileAccess::set_unix_permissions(locked.path, 0700);
		REQUIRE(restored == OK);
		if (restored != OK) {
			return;
		}
		CHECK(DirAccess::remove_absolute(retained) == OK);
	}
#endif
}

TEST_CASE("[SceneTree][SPX] Recording workers finalize their files when destroyed while active") {
	TestSpx::TempDirectory files("spx_recorder_destruct");
	TestSpx::EngineScene scene;
	ThreadSafeFrameBuffer buffer;
	HybridAudioDriver driver;
	REQUIRE(driver.init(8000) == OK);
	{
		IndependentVideoRecorder video;
		IndependentVideoRecorder::RecordingConfig video_config;
		video_config.video_width = 4;
		video_config.video_height = 4;
		REQUIRE(video.initialize(&buffer, files.file("video.avi"), video_config) == OK);
		REQUIRE(video.start_recording() == OK);
		auto frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
		frame->fill(Color(0.5, 0.25, 0));
		buffer.update_frame(frame, 0);
		REQUIRE(wait_for([&] { return video.get_statistics().total_recorded_frames >= 1; }));
		IndependentAudioRecorder audio;
		IndependentAudioRecorder::AudioConfig audio_config;
		audio_config.sample_rate = 8000;
		audio_config.chunk_size = 80;
		REQUIRE(audio.initialize(&driver, files.file("audio.avi"), audio_config) == OK);
		REQUIRE(audio.start_recording() == OK);
		Vector<int32_t> samples;
		samples.resize(160);
		samples.fill(0x10000000);
		audio.on_audio_output(samples.ptr(), 80);
		REQUIRE(wait_for([&] { return audio.get_statistics().total_chunks_recorded == 1; }));
	}
	const auto video = read_avi(files.file("video.avi"));
	const int index = find_tag(video, "idx1");
	REQUIRE(index > 0);
	check_avi_index(video, decode_uint32(video.ptr() + index + 4) / 16);
	check_avi_index(read_avi(files.file("audio.avi")), 1);
}

TEST_CASE("[SceneTree][SPX] Realtime capture failure detaches audio and allows a clean restart") {
	TestSpx::TempDirectory files("spx_capture_failure");
	TestSpx::EngineScene scene;
	const int effects = AudioServer::get_singleton()->get_bus_effect_count(0);
	const String path = files.file("capture.avi");
	REQUIRE(DirAccess::make_dir_absolute(path + "_audio.avi") == OK);
	auto destroy = [](RealtimeWriter *writer) { memdelete(writer); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	ObsStyleMovieWriter::ObsRecordingConfig config;
	config.enable_post_merge = true;
	config.keep_intermediate_files = false;
	config.enable_debug_output = true;
	writer->set_recording_config(config);
	ERR_PRINT_OFF;
	CHECK(writer->begin_realtime(Size2i(4, 4), 30, path) == ERR_CANT_OPEN);
	ERR_PRINT_ON;
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	REQUIRE(DirAccess::remove_absolute(path + "_audio.avi") == OK);
	REQUIRE(writer->begin_realtime(Size2i(4, 4), 30, path) == OK);
	REQUIRE(DirAccess::make_dir_absolute(path + "_merged.avi") == OK);
	ERR_PRINT_OFF;
	writer->end_realtime();
	ERR_PRINT_ON;
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
	CHECK(FileAccess::exists(path + "_video.avi"));
	CHECK(FileAccess::exists(path + "_audio.avi"));
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	REQUIRE(DirAccess::remove_absolute(path + "_merged.avi") == OK);
	REQUIRE(writer->begin_realtime(Size2i(4, 4), 30, path) == OK);
	writer.reset(); // Destruction stops both workers and merges their empty, valid streams.
	CHECK(AudioServer::get_singleton()->get_bus_effect_count(0) == effects);
	check_avi_index(read_avi(path + "_merged.avi"), 0);
	CHECK_FALSE(FileAccess::exists(path + "_video.avi"));
	CHECK_FALSE(FileAccess::exists(path + "_audio.avi"));
}

TEST_CASE("[SPX] Native movie routes honor enabled modes and supported extensions") {
	auto *settings = ProjectSettings::get_singleton();
	struct Restore {
		Variant obs, realtime;
		~Restore() {
			auto *settings = ProjectSettings::get_singleton();
			settings->set_setting("movie_writer/obs_mode", obs);
			settings->set_setting("movie_writer/realtime_mode", realtime);
		}
	} restore{ settings->get_setting("movie_writer/obs_mode"), settings->get_setting("movie_writer/realtime_mode") };
	auto destroy = [](RealtimeWriter *writer) { memdelete(writer); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	for (int mode = 0; mode < 4; ++mode) {
		settings->set_setting("movie_writer/obs_mode", bool(mode & 1));
		settings->set_setting("movie_writer/realtime_mode", bool(mode & 2));
		CHECK(writer->handles_file("capture.AVI") == (mode != 0));
		CHECK(spx_recorder_claims_movie("capture.avi") == (mode != 0));
		CHECK_FALSE(writer->handles_file("capture.png"));
		CHECK_FALSE(spx_recorder_claims_movie("capture.png"));
		CHECK_FALSE(spx_recorder_claims_movie(""));
	}
	CHECK(ObsStyleMovieWriter::is_supported_format("AVI"));
	CHECK_FALSE(ObsStyleMovieWriter::is_supported_format("webm"));
	writer->add_realtime_frame();
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
	CHECK(writer->get_combined_statistics().game_frames_added == 0);
}

TEST_CASE("[SPX] Recorder configuration uses documented defaults when project overrides are absent") {
	struct Restore {
		Dictionary values;
		~Restore() {
			for (const Variant *key = values.next(nullptr); key; key = values.next(key)) {
				ProjectSettings::get_singleton()->set_setting(*key, values[*key]);
			}
		}
	} restore;
	for (const char *name : { "obs_video_fps", "obs_video_quality", "obs_audio_sample_rate", "obs_audio_channels", "obs_enable_debug_output", "obs_enable_post_merge", "obs_keep_intermediate_files" }) {
		String key = String("movie_writer/") + name;
		restore.values[key] = ProjectSettings::get_singleton()->get_setting(key);
		ProjectSettings::get_singleton()->set_setting(key, Variant());
	}
	auto destroy = [](RealtimeWriter *writer) { memdelete(writer); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	const auto &config = writer->get_recording_config();
	CHECK(config.video_fps == 30);
	CHECK(config.jpeg_quality == doctest::Approx(0.85));
	CHECK(config.audio_sample_rate == 48000);
	CHECK(config.audio_channels == 2);
	CHECK_FALSE(config.enable_debug_output);
	CHECK(config.enable_post_merge);
	CHECK_FALSE(config.keep_intermediate_files);
	CHECK(writer->get_comprehensive_debug_info().contains("Uninitialized"));
	CHECK_FALSE(writer->get_comprehensive_debug_info().contains("Video Recorder Info"));
	CHECK_FALSE(writer->get_comprehensive_debug_info().contains("Audio Recorder Info"));
}

TEST_CASE("[SPX] AVI parser rejects incomplete duplicated and out-of-order container chunks") {
	TestSpx::TempDirectory files("spx_avi_structure");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original = read_avi(video), original_audio = read_avi(audio);
	const int hdrl = find_tag(original, "hdrl"), strl = find_tag(original, "strl"), movi = find_tag(original, "movi"), index = find_tag(original, "idx1");
	REQUIRE(hdrl > 0);
	REQUIRE(strl > 0);
	REQUIRE(movi > 0);
	REQUIRE(index > 0);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	processor.set_config(config);
	for (int damage = 0; damage < 17; ++damage) {
		INFO("Container damage: ", damage);
		auto bytes = original;
		switch (damage) {
			case 0:
				bytes.resize(4); // Truncated RIFF header.
				break;
			case 1:
				bytes.resize(12); // Header without mandatory lists.
				break;
			case 2:
				bytes.resize(bytes.size() + 4); // Incomplete trailing chunk header.
				break;
			case 3:
				bytes.append_array(original.slice(hdrl - 8, movi - 8)); // Duplicate hdrl.
				break;
			case 4:
				memcpy(bytes.ptrw() + hdrl, "movi", 4); // movi before hdrl.
				break;
			case 5:
				bytes.append_array(original.slice(index)); // Duplicate idx1.
				break;
			case 6:
				memcpy(bytes.ptrw() + 12, "idx1", 4); // idx1 before movi.
				break;
			case 7:
				memcpy(bytes.ptrw() + strl, "odml", 4); // Missing stream header list.
				break;
			case 8: {
				// Four bytes remain after the last complete hdrl child.
				const auto suffix = bytes.slice(movi - 8);
				bytes.resize(movi - 8 + 4);
				bytes.append_array(suffix);
				encode_uint32(decode_uint32(original.ptr() + hdrl - 4) + 4, bytes.ptrw() + hdrl - 4);
				break;
			}
			case 9:
				encode_uint32(60, bytes.ptrw() + strl - 4); // strl ends immediately after strh.
				break;
			case 10:
				memcpy(bytes.ptrw() + find_tag(bytes, "strf"), "JUNK", 4); // Missing format.
				break;
			case 11:
				bytes.resize(index);
				memcpy(bytes.ptrw() + movi, "JUNK", 4); // Missing movi and index.
				break;
			case 12:
				bytes.resize(index + 4); // Recovery must reject an incomplete movi chunk.
				encode_uint32(decode_uint32(original.ptr() + movi - 4) + 4, bytes.ptrw() + movi - 4);
				break;
			case 13:
				bytes.append_array(original.slice(movi - 8, index)); // Duplicate movi after a valid index.
				break;
			case 14:
				// A first hdrl after movi must not repair the earlier missing header.
				memcpy(bytes.ptrw() + hdrl, "JUNK", 4);
				bytes.append_array(original.slice(hdrl - 8, movi - 8));
				break;
			case 15:
				// LIST child has its header, but not the four-byte list type.
				encode_uint32(2, bytes.ptrw() + strl - 4);
				break;
			case 16:
				// strh fits in the physical file but escapes its immediate strl parent.
				encode_uint32(58, bytes.ptrw() + strl - 4);
				break;
		}
		if (bytes.size() >= 12) {
			encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
		}
		write_avi(video, bytes);
		ERR_PRINT_OFF;
		auto result = processor.merge_files(video, audio, files.file("output.avi"));
		ERR_PRINT_ON;
		CHECK(result.error_code == ERR_FILE_CORRUPT);
		CHECK_FALSE(FileAccess::exists(files.file("output.avi")));
		CHECK(FileAccess::get_file_as_bytes(video) == bytes);
		CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	}
	write_avi(video, original);
	ERR_PRINT_OFF;
	CHECK(processor.merge_files(audio, audio, files.file("wrong_video.avi")).error_code == ERR_FILE_CORRUPT);
	CHECK(processor.merge_files(video, video, files.file("wrong_audio.avi")).error_code == ERR_FILE_CORRUPT);
	ERR_PRINT_ON;
}

TEST_CASE("[SPX] AVI merge accepts compressed-video chunk tags and optional unknown metadata") {
	TestSpx::TempDirectory files("spx_avi_metadata");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	auto bytes = read_avi(video);
	const int index = find_tag(bytes, "idx1");
	const int movi = find_tag(bytes, "movi");
	REQUIRE(index > 0);
	REQUIRE(movi > 0);
	for (int i = 0; i < 2; ++i) {
		int entry = index + 8 + i * 16;
		int chunk = movi + decode_uint32(bytes.ptr() + entry + 8);
		bytes.set(entry + 3, 'c');
		bytes.set(chunk + 3, 'c');
	}
	// OpenDML metadata is optional; skip an unrecognized child with the same bounds.
	memcpy(bytes.ptrw() + find_tag(bytes, "odml") - 8, "JUNK", 4);
	PackedByteArray junk;
	junk.resize(10);
	memcpy(junk.ptrw(), "JUNK", 4);
	encode_uint32(1, junk.ptrw() + 4); // One byte plus required word padding.
	const auto chunks = bytes.slice(12);
	bytes.resize(12);
	bytes.append_array(junk);
	bytes.append_array(chunks);
	encode_uint32(bytes.size() - 8, bytes.ptrw() + 4);
	write_avi(video, bytes);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.keep_intermediate_files = true;
	processor.set_config(config);
	const auto result = processor.merge_files(video, audio, files.file("merged.avi"));
	REQUIRE(result.error_code == OK);
	check_avi_index(read_avi(files.file("merged.avi")), 4);
	CHECK(FileAccess::get_file_as_bytes(video) == bytes);
}

#ifdef UNIX_ENABLED
TEST_CASE("[SPX] Verbose headless command-line recording finalizes valid AVI streams") {
	TestSpx::TempDirectory files("spx_cli_movie");
	for (bool keep : { true, false }) {
		files.write("project.godot", String(R"GODOT(config_version=5
[application]
run/main_scene="res://main.tscn"
[display]
window/size/viewport_width=16
window/size/viewport_height=12
[debug]
file_logging/enable_file_logging=false
[movie_writer]
obs_mode=true
obs_enable_debug_output=true
obs_keep_intermediate_files=KEEP
)GODOT").replace("KEEP", keep ? "true" : "false"));
		files.write("main.tscn", "[gd_scene format=3]\n[node name=\"RecordingFixture\" type=\"Node\"]\n");
		const String path = files.file("capture.avi");
		List<String> arguments;
		for (const String &argument : { String("--headless"), String("--verbose"), String("--path"), files.path, String("--write-movie"), path, String("--quit-after"), String("3") }) {
			arguments.push_back(argument);
		}
		String output;
		int exit_code = -1;
		const Error error = OS::get_singleton()->execute(OS::get_singleton()->get_executable_path(), arguments, &output, &exit_code, true);
		INFO(output);
		REQUIRE(error == OK);
		REQUIRE(exit_code == 0);
		CHECK_FALSE(output.contains("ERROR:"));
		CHECK(output.contains("IndependentVideoRecorder initialized"));
		CHECK(output.contains("IndependentAudioRecorder initialization completed"));
		CHECK(output.contains("OBS-style Recording Completed"));
		CHECK(output.contains("Intermediate files cleaned up successfully") == !keep);
		for (const char *suffix : { "_video.avi", "_audio.avi", "_merged.avi" }) {
			if (!keep && String(suffix) != "_merged.avi") {
				CHECK_FALSE(FileAccess::exists(path + suffix));
				continue;
			}
			const auto bytes = read_avi(path + suffix);
			const int index = find_tag(bytes, "idx1");
			REQUIRE(index > 0);
			check_avi_index(bytes, decode_uint32(bytes.ptr() + index + 4) / 16);
		}
		const auto merged = read_avi(path + "_merged.avi");
		const int header = find_tag(merged, "avih");
		REQUIRE(header > 0);
		CHECK(decode_uint32(merged.ptr() + header + 40) == 16);
		CHECK(decode_uint32(merged.ptr() + header + 44) == 12);
		CHECK(decode_uint32(merged.ptr() + header + 32) == 2);
	}
}
#endif

} // namespace TestSpxRecorder

#endif // TEST_SPX_RECORDER_H
