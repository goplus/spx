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
#include "main/main_loop_phase_callback_bus.h"
#include "servers/rendering_server.h"
#include "spx_test_fixtures.h"

namespace TestSpxRecorder {

TEST_CASE("[SPX] Recording buffer publishes the latest frame and resets its statistics") {
	ThreadSafeFrameBuffer buffer;
	CHECK_FALSE(buffer.has_new_frame());
	CHECK(buffer.get_current_frame().image.is_null());
	CHECK(buffer.get_last_sequence() == 0);
	buffer.update_frame(Ref<Image>(), 1, 1);
	CHECK(buffer.get_total_updates() == 0);
	Ref<Image> first = Image::create_empty(2, 2, false, Image::FORMAT_RGB8);
	Ref<Image> second = Image::create_empty(4, 4, false, Image::FORMAT_RGBA8);
	buffer.update_frame(first, 1000, 7);
	const auto snapshot = buffer.get_current_frame();
	CHECK(snapshot.image == first);
	CHECK(snapshot.game_timestamp == 1000);
	CHECK(snapshot.frame_sequence == 7);
	CHECK(snapshot.is_new_frame);
	CHECK(buffer.has_new_frame());
	buffer.update_frame(second, 2000, 8);
	const auto latest = buffer.get_current_frame();
	CHECK(latest.image == second);
	CHECK(latest.game_timestamp == 2000);
	CHECK(latest.frame_sequence == 8);
	CHECK(snapshot.image == first);
	CHECK(snapshot.frame_sequence == 7);
	CHECK(buffer.get_last_sequence() == 8);
	CHECK(buffer.get_total_updates() == 2);
	CHECK(buffer.get_buffer_switches() == 2);
	buffer.reset();
	CHECK_FALSE(buffer.has_new_frame());
	CHECK_FALSE(buffer.get_current_frame().is_new_frame);
	CHECK(buffer.get_current_frame().image.is_null());
	CHECK(buffer.get_current_frame().game_timestamp == 0);
	CHECK(buffer.get_last_sequence() == 0);
	CHECK(buffer.get_total_updates() == 0);
	CHECK(buffer.get_buffer_switches() == 0);
	buffer.update_frame(first, 3000, 1);
	CHECK(buffer.get_current_frame().image == first);
	CHECK(buffer.get_last_sequence() == 1);
	CHECK(buffer.get_total_updates() == 1);
}

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

TEST_CASE("[SPX] Movie merge validates inputs and preserves them when merging is disabled") {
	TestSpx::TempDirectory files("spx_merge_validation");
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	CHECK(processor.get_method_name(static_cast<PostMergeProcessor::MergeMethod>(99)) == "Unknown");
	auto result = processor.merge_files(video, audio, output);
	CHECK(result.error_code == ERR_FILE_CORRUPT);
	CHECK_FALSE(result.error_message.is_empty());
	CHECK_FALSE(result.intermediate_files_cleaned);
	CHECK_FALSE(FileAccess::exists(output));
	config.method = PostMergeProcessor::METHOD_NONE;
	processor.set_config(config);
	result = processor.merge_files(video, audio, output);
	CHECK(result.error_code == OK);
	CHECK(result.output_file_path == video);
	CHECK_FALSE(result.intermediate_files_cleaned);
	CHECK(FileAccess::exists(video));
	CHECK(FileAccess::exists(audio));
	CHECK_FALSE(FileAccess::exists(output));
	config.method = static_cast<PostMergeProcessor::MergeMethod>(99);
	processor.set_config(config);
	result = processor.merge_files(video, audio, output);
	CHECK(result.error_code == ERR_UNAVAILABLE);
	CHECK(result.error_message == "Validation failed");
	CHECK_FALSE(result.intermediate_files_cleaned);
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
	ERR_PRINT_ON;
	IndependentVideoRecorder::RecordingConfig config;
	config.video_width = 4;
	config.video_height = 4;
	config.jpeg_quality = 0.9;
	recorder.update_config(config);
	REQUIRE(recorder.initialize(&buffer, files.file("video.avi"), config) == OK);
	REQUIRE(recorder.start_recording() == OK);
	CHECK(recorder.is_thread_running());
	ERR_PRINT_OFF;
	CHECK(recorder.start_recording() == ERR_ALREADY_IN_USE);
	ERR_PRINT_ON;
	Ref<Image> frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	frame->fill(Color(1, 0, 0));
	buffer.update_frame(frame, 1, 1);
	REQUIRE(wait_for([&] { return recorder.get_statistics().repeated_frames_count >= 2; }));
	Ref<Image> second = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	second->fill(Color(0, 1, 0));
	buffer.update_frame(second, 2, 2);
	REQUIRE(wait_for([&] { return recorder.get_statistics().new_frames_count == 2; }));
	recorder.stop_recording();
	recorder.stop_recording();
	CHECK_FALSE(recorder.is_recording());
	CHECK_FALSE(recorder.is_thread_running());
	auto stats = recorder.get_statistics();
	CHECK(stats.total_recorded_frames == stats.new_frames_count + stats.repeated_frames_count);
	CHECK(stats.last_game_frame_sequence == 2);
	CHECK(stats.recording_duration_us > 0);
	CHECK(recorder.get_repeat_frame_ratio() == doctest::Approx(double(stats.repeated_frames_count) / stats.total_recorded_frames));
	PackedByteArray bytes = read_avi(files.file("video.avi"));
	const int avih = find_tag(bytes, "avih");
	REQUIRE(avih >= 0);
	CHECK(decode_uint32(bytes.ptr() + avih + 24) == stats.total_recorded_frames);
	CHECK(recorder.get_debug_info().contains("Stopped"));
	recorder.reset_statistics();
	CHECK(recorder.get_statistics().total_recorded_frames == 0);
}

TEST_CASE("[SceneTree][SPX] Audio capture preserves the master mix and detaches its effect") {
	TestSpx::EngineScene scene;
	auto *server = AudioServer::get_singleton();
	const int before = server->get_bus_effect_count(0);
	HybridAudioDriver driver;
	REQUIRE(driver.init(100, AudioDriver::SPEAKER_MODE_STEREO) == OK);
	CHECK(server->get_bus_effect_count(0) == before + 1);
	CHECK(driver.get_mix_rate() == 100);
	CHECK(driver.get_channels() == 2);
	ERR_PRINT_OFF;
	CHECK(driver.init(100, AudioDriver::SPEAKER_MODE_STEREO) == ERR_ALREADY_IN_USE);
	CHECK(driver.get_captured_audio_data(nullptr, 1) == 0);
	int32_t unused[2] = { 1, 1 };
	CHECK(driver.get_captured_audio_data(unused, -1) == 0);
	driver.register_audio_recorder(nullptr);
	ERR_PRINT_ON;
	CHECK(driver.get_captured_audio_data(unused, 0) == 0);
	CHECK(driver.get_captured_audio_data(unused, 1) == 1);
	CHECK(unused[0] == 0);
	CHECK(unused[1] == 0);
	IndependentAudioRecorder first, second, absent;
	driver.register_audio_recorder(&first);
	driver.register_audio_recorder(&second);
	driver.unregister_audio_recorder(&absent);
	driver.unregister_audio_recorder(nullptr);
	CHECK(driver.get_registered_recorder_count() == 2);
	driver.unregister_audio_recorder(&second);
	CHECK(driver.get_registered_recorder_count() == 1);
	driver.start();
	Ref<AudioEffectInstance> effect = server->get_bus_effect(0, before)->instantiate();
	REQUIRE(effect.is_valid());
	CHECK_FALSE(effect->process_silence());
	AudioFrame input[2] = { AudioFrame(0.25, -0.5), AudioFrame(-0.25, 0.5) };
	AudioFrame output[2];
	effect->process(input, output, 2);
	CHECK(output[0].left == input[0].left);
	CHECK(output[1].right == input[1].right);
	CHECK_FALSE(driver.has_audio_data());
	driver.enable_recording(true);
	effect->process(input, output, 2);
	CHECK(driver.get_available_frames() == 2);
	int32_t pcm[6] = {};
	CHECK(driver.get_captured_audio_data(pcm, 3) == 3);
	CHECK(pcm[0] == 536870912);
	CHECK(pcm[1] == -1073741824);
	CHECK(pcm[2] == -536870912);
	CHECK(pcm[3] == 1073741824);
	CHECK(pcm[4] == 0);
	CHECK(pcm[5] == 0);
	CHECK_FALSE(driver.has_audio_data());
	int32_t mono[40];
	for (int i = 0; i < 40; ++i) {
		mono[i] = i * 65536;
	}
	driver.capture_audio_data(mono, 40, 0); // Overflow retains the newest samples.
	int available = driver.get_available_frames();
	REQUIRE(available > 0);
	REQUIRE(available < 40);
	Vector<int32_t> captured;
	captured.resize(available * 2);
	driver.get_captured_audio_data(captured.ptrw(), available);
	CHECK(captured[0] == mono[40 - available]);
	CHECK(captured[1] == captured[0]);
	CHECK(captured[captured.size() - 1] == mono[39]);
	driver.enable_recording(false);
	driver.capture_audio_data(mono, 1, 0);
	CHECK_FALSE(driver.has_audio_data());
	driver.enable_recording(true);
	driver.capture_audio_data(nullptr, 1, 0);
	driver.capture_audio_data(mono, 0, 0);
	CHECK_FALSE(driver.has_audio_data());
	driver.finish();
	CHECK(server->get_bus_effect_count(0) == before);
	CHECK_FALSE(driver.is_recording_enabled());
	effect->process(input, output, 2); // Detached instances must remain safe passthroughs.
	CHECK(output[0].right == input[0].right);
	CHECK_FALSE(driver.has_audio_data());
	driver.finish();
}

TEST_CASE("[SceneTree][SPX] Audio recorder consumes captured PCM and closes the worker file") {
	TestSpx::TempDirectory files("spx_audio_thread");
	TestSpx::EngineScene scene;
	HybridAudioDriver driver;
	REQUIRE(driver.init(8000, AudioDriver::SPEAKER_MODE_STEREO) == OK);
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
	driver.capture_audio_data(samples.ptr(), 80 * 4, 1);
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
static void write_avi_pair(const String &video, const String &audio, int frame_count = 2) {
	Ref<SimpleVideoWriter> video_writer;
	video_writer.instantiate();
	Ref<SimpleAudioWriter> audio_writer;
	audio_writer.instantiate();
	REQUIRE(video_writer->open(video, Size2i(4, 4), 30, 0.85) == OK);
	REQUIRE(audio_writer->open(audio, 48000, 2) == OK);
	auto image = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	image->fill(Color(1, 0, 0));
	Vector<int32_t> pcm;
	pcm.resize(1600 * 2);
	pcm.fill(0x12340000);
	for (int i = 0; i < frame_count; ++i) {
		REQUIRE(video_writer->write_frame(image) == OK);
		REQUIRE(audio_writer->write_audio_chunk(pcm.ptr(), 1600) == OK);
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
	for (auto strategy : { PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE, PostMergeProcessor::MergeConfig::INTERLEAVE_TIMESTAMP_BASED, PostMergeProcessor::MergeConfig::INTERLEAVE_BUFFERED_TIMESTAMP }) {
		PostMergeProcessor::MergeConfig config;
		config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
		config.interleave_strategy = strategy;
		config.keep_intermediate_files = true;
		config.enable_debug_output = true;
		processor.set_config(config);
		const String output = files.file("merged_" + itos(strategy) + ".avi");
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
	}
	auto config = processor.get_config();
	config.keep_intermediate_files = false;
	processor.set_config(config);
	auto result = processor.merge_files(video, audio, files.file("final.avi"));
	REQUIRE(result.error_code == OK);
	CHECK(result.intermediate_files_cleaned);
	CHECK_FALSE(FileAccess::exists(video));
	CHECK_FALSE(FileAccess::exists(audio));
}

TEST_CASE("[SPX] AVI interleaving preserves unequal streams and strategy ordering") {
	TestSpx::TempDirectory files("spx_interleave_order");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio); // Video frames occur at 0 and 33,333 microseconds.
	SimpleAudioWriter writer;
	REQUIRE(writer.open(audio, 48000, 2) == OK);
	Vector<int32_t> pcm;
	pcm.resize(480 * 2);
	for (int i = 0; i < 3; ++i) {
		pcm.fill((i + 1) * 0x10000000);
		REQUIRE(writer.write_audio_chunk(pcm.ptr(), 480) == OK); // 0, 10,000, 20,000 microseconds.
	}
	writer.close();
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	for (auto strategy : { PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE, PostMergeProcessor::MergeConfig::INTERLEAVE_TIMESTAMP_BASED, PostMergeProcessor::MergeConfig::INTERLEAVE_BUFFERED_TIMESTAMP }) {
		INFO("Interleave strategy: ", strategy);
		PostMergeProcessor processor;
		PostMergeProcessor::MergeConfig config;
		config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
		config.interleave_strategy = strategy;
		config.keep_intermediate_files = true;
		processor.set_config(config);
		const String output = files.file("merged.avi");
		REQUIRE(processor.merge_files(video, audio, output).error_code == OK);
		const auto bytes = read_avi(output);
		check_avi_index(bytes, 5);
		const int index = find_tag(bytes, "idx1"), movi = find_tag(bytes, "movi"), stream = find_tag(bytes, "auds");
		REQUIRE(stream > 0);
		CHECK(decode_uint32(bytes.ptr() + stream + 32) == 1440);
		bool is_video[5] = {};
		int audio_chunks = 0;
		for (int i = 0; i < 5; ++i) {
			const uint8_t *entry = bytes.ptr() + index + 8 + i * 16;
			is_video[i] = memcmp(entry, "00db", 4) == 0;
			if (!is_video[i]) {
				CHECK(memcmp(entry, "01wb", 4) == 0);
				REQUIRE(decode_uint32(entry + 12) == 480 * 2 * 2);
				const uint32_t chunk = movi + decode_uint32(entry + 8);
				++audio_chunks;
				PackedByteArray expected_pcm;
				expected_pcm.resize(480 * 2 * 2);
				for (int sample = 0; sample < 480 * 2; ++sample) {
					encode_uint16(audio_chunks * 0x1000, expected_pcm.ptrw() + sample * 2);
				}
				CHECK(memcmp(bytes.ptr() + chunk + 8, expected_pcm.ptr(), expected_pcm.size()) == 0);
			}
		}
		CHECK(audio_chunks == 3);
		if (strategy == PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE) {
			CHECK(is_video[0]);
			CHECK_FALSE(is_video[1]);
			CHECK(is_video[2]);
			CHECK_FALSE(is_video[3]);
			CHECK_FALSE(is_video[4]);
		} else {
			CHECK(is_video[0] != is_video[1]); // Equal timestamps need not have a stable tie order.
			CHECK_FALSE(is_video[2]);
			CHECK_FALSE(is_video[3]);
			CHECK(is_video[4]);
		}
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
	auto config = ObsStyleMovieWriter::get_performance_config();
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
	ERR_PRINT_OFF;
	CHECK(writer->pause_recording() == ERR_UNAVAILABLE);
	CHECK(writer->resume_recording() == ERR_UNAVAILABLE);
	ERR_PRINT_ON;
	CHECK_FALSE(writer->is_paused());
	Ref<Image> frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
	frame->fill(Color(1, 0, 0));
	for (int run = 0; run < 2; ++run) {
		config.enable_post_merge = (run == 1);
		config.keep_intermediate_files = true;
		config.enable_debug_output = (run == 1);
		writer->set_recording_config(config);
		const String path = files.file("capture_" + itos(run) + ".avi");
		REQUIRE(writer->begin_realtime(Size2i(4, 4), 30, path) == OK);
		CHECK(writer->is_recording_active());
		CHECK(writer->get_state_name() == "Recording");
		REQUIRE(writer->write_frame(Ref<Image>(), nullptr) == OK);
		CHECK(writer->get_combined_statistics().game_frames_added == 0);
		CHECK(writer->get_combined_statistics().video_stats.total_recorded_frames == 0);
		ERR_PRINT_OFF;
		CHECK(writer->begin_realtime(Size2i(4, 4), 30, path) == ERR_INVALID_PARAMETER);
		auto changed = config;
		changed.video_fps = 60;
		writer->set_recording_config(changed);
		ERR_PRINT_ON;
		CHECK(writer->get_recording_config().video_fps == 30);
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
		const uint32_t frames = decode_uint32(video.ptr() + header + 24);
		CHECK(frames >= stats.video_stats.total_recorded_frames);
		check_avi_index(video, frames);
		CHECK(FileAccess::exists(path + "_audio.avi"));
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
	auto config = ObsStyleMovieWriter::get_standard_config();
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
		auto config = ObsStyleMovieWriter::get_standard_config();
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
	auto config = ObsStyleMovieWriter::get_high_quality_config();
	config.enable_post_merge = false;
	writer->set_recording_config(config);
	ERR_PRINT_OFF;
	CHECK(writer->begin_realtime(Size2i(4, 4), 30, files.file("missing/invalid.avi")) != OK);
	ERR_PRINT_ON;
	CHECK(writer->get_recording_state() == ObsStyleMovieWriter::STATE_UNINITIALIZED);
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
	CHECK(MovieRecorderManager::pause_recording() == ERR_UNAVAILABLE);
	CHECK(MovieRecorderManager::resume_recording() == ERR_UNAVAILABLE);
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
	config.keep_intermediate_files = true;
	config.enable_debug_output = true;
	for (auto strategy : { PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE, PostMergeProcessor::MergeConfig::INTERLEAVE_TIMESTAMP_BASED }) {
		config.interleave_strategy = strategy;
		processor.set_config(config);
		String output = files.file("recovered_" + itos(strategy) + ".avi");
		auto result = processor.merge_files(video, audio, output);
		INFO(result.error_message);
		REQUIRE(result.error_code == OK);
		check_avi_index(read_avi(output), 40);
	}
	CHECK(processor.get_recommended_method() == PostMergeProcessor::METHOD_CUSTOM_AVI);
	CHECK(processor.get_method_name(PostMergeProcessor::METHOD_NONE) == "No Merge");
	CHECK(processor.get_method_name(PostMergeProcessor::METHOD_FFMPEG_SYSTEM) == "FFmpeg System Command");
	CHECK_FALSE(processor.is_method_available(static_cast<PostMergeProcessor::MergeMethod>(99)));
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
	config.method = PostMergeProcessor::METHOD_FFMPEG_SYSTEM;
	config.ffmpeg_path = files.file("missing-ffmpeg");
	processor.set_config(config);
	ERR_PRINT_OFF;
	CHECK(processor.merge_files(video, audio, files.file("ffmpeg.avi")).error_code == ERR_UNAVAILABLE);
	ERR_PRINT_ON;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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

TEST_CASE("[SPX] AVI merger requires complete ordered indexes and preserves existing output") {
	TestSpx::TempDirectory files("spx_avi_index_order");
	const String video = files.file("video.avi"), audio = files.file("audio.avi"), output = files.file("merged.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	files.write("merged.avi", "existing recording");
	const auto original_output = FileAccess::get_file_as_bytes(output);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
	for (auto strategy : { PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE, PostMergeProcessor::MergeConfig::INTERLEAVE_TIMESTAMP_BASED }) {
		config.interleave_strategy = strategy;
		processor.set_config(config);
		for (bool damage_audio : { true, false }) {
			for (int damage = 0; damage < 4; ++damage) {
				INFO("Strategy: ", strategy, ", audio: ", damage_audio, ", index damage: ", damage);
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
	config.keep_intermediate_files = true;
	for (auto strategy : { PostMergeProcessor::MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE, PostMergeProcessor::MergeConfig::INTERLEAVE_TIMESTAMP_BASED }) {
		config.interleave_strategy = strategy;
		processor.set_config(config);
		for (int empty = 1; empty <= 3; ++empty) {
			const String output = files.file("merged.avi");
			auto result = processor.merge_files(empty & 1 ? empty_video : video, empty & 2 ? empty_audio : audio, output);
			INFO(result.error_message);
			REQUIRE(result.error_code == OK);
			check_avi_index(read_avi(output), empty == 3 ? 0 : 2);
		}
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

#ifdef UNIX_ENABLED
TEST_CASE("[SPX] FFmpeg merge validates command arguments exit status and produced output") {
	TestSpx::TempDirectory files("spx_ffmpeg");
	const String video = files.file("video.avi"), audio = files.file("audio.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	// A local executable models the subprocess contract without depending on installed codecs.
	files.write("ffmpeg fixture", R"SH(#!/bin/sh
if [ "$1" = "-version" ]; then
  printf 'ffmpeg test fixture\n'
  exit 0
fi
if [ "$#" -ne 8 ] || [ "$1" != "-i" ] || [ "$3" != "-i" ] || [ "$5" != "-c" ] || [ "$6" != "copy" ] || [ "$8" != "-y" ]; then
  exit 99
fi
case "$7" in
  *failure.avi) printf 'synthetic encoder failure\n'; exit 7 ;;
  *no-output.avi) exit 0 ;;
  *) cp "$2" "$7" ;;
esac
)SH");
	REQUIRE(FileAccess::set_unix_permissions(files.file("ffmpeg fixture"), 0700) == OK);
	PostMergeProcessor processor;
	PostMergeProcessor::MergeConfig config;
	config.method = PostMergeProcessor::METHOD_FFMPEG_SYSTEM;
	config.ffmpeg_path = files.file("ffmpeg fixture");
	config.keep_intermediate_files = true;
	config.enable_debug_output = true;
	processor.set_config(config);
	REQUIRE(processor.is_method_available(config.method));
	const String output = files.file("success.avi");
	auto result = processor.merge_files(video, audio, output);
	REQUIRE(result.error_code == OK);
	CHECK(result.output_file_path == output);
	CHECK(FileAccess::get_file_as_bytes(output) == original_video);
	files.write("failure.avi", "previous recording");
	const auto previous = FileAccess::get_file_as_bytes(files.file("failure.avi"));
	result = processor.merge_files(video, audio, files.file("failure.avi"));
	CHECK(result.error_code == ERR_FILE_CANT_WRITE);
	CHECK(result.error_message.contains("exit code 7"));
	CHECK(result.error_message.contains("synthetic encoder failure"));
	CHECK(FileAccess::get_file_as_bytes(files.file("failure.avi")) == previous);
	CHECK_FALSE(result.intermediate_files_cleaned);
	result = processor.merge_files(video, audio, files.file("no-output.avi"));
	CHECK(result.error_code == ERR_FILE_CANT_WRITE);
	CHECK(result.error_message.contains("not created"));
	CHECK(FileAccess::get_file_as_bytes(video) == original_video);
	CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
	files.write("ffmpeg fixture", "#!/bin/sh\nprintf 'ffmpeg failed version check\n'\nexit 4\n");
	CHECK_FALSE(processor.is_method_available(config.method));
	result = processor.merge_files(video, audio, files.file("unavailable.avi"));
	CHECK(result.error_code == ERR_UNAVAILABLE);
	CHECK(result.error_message == "Validation failed");
	CHECK_FALSE(FileAccess::exists(files.file("unavailable.avi")));
	files.write("ffmpeg fixture", "#!/bin/sh\nprintf 'unrelated executable\\n'\nexit 0\n");
	CHECK_FALSE(processor.is_method_available(config.method));
	CHECK(FileAccess::get_file_as_bytes(video) == original_video);
	CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
}

TEST_CASE("[SPX] FFmpeg merge preserves recordings when a checked executable stops working") {
	TestSpx::TempDirectory files("spx_ffmpeg_changed");
	const String video = files.file("video.avi"), audio = files.file("audio.avi"), output = files.file("output.avi");
	write_avi_pair(video, audio);
	const auto original_video = read_avi(video), original_audio = read_avi(audio);
	for (int failure = 0; failure < 3; ++failure) {
		INFO("Subprocess failure: ", failure);
		// Deterministic subprocess lifecycle: fail the second probe, disappear after
		// the second probe, or launch successfully but fail without diagnostics.
		const String executable = "ffmpeg_" + itos(failure);
		String script = R"SH(#!/bin/sh
if [ "$1" = "-version" ]; then
  if [ -f "$0.checked" ]; then
    SECOND_PROBE
  else
    touch "$0.checked"
  fi
  printf 'ffmpeg test fixture\n'
  exit 0
fi
exit 7
)SH";
		script = script.replace("SECOND_PROBE", failure == 0 ? "exit 1" : (failure == 1 ? "rm -- \"$0\"" : ":"));
		files.write(executable, script);
		REQUIRE(FileAccess::set_unix_permissions(files.file(executable), 0700) == OK);
		files.write("output.avi", "previous recording");
		PostMergeProcessor processor;
		PostMergeProcessor::MergeConfig config;
		config.method = PostMergeProcessor::METHOD_FFMPEG_SYSTEM;
		config.ffmpeg_path = files.file(executable);
		config.keep_intermediate_files = false;
		processor.set_config(config);
		ERR_PRINT_OFF;
		const auto result = processor.merge_files(video, audio, output);
		ERR_PRINT_ON;
		CHECK(result.error_code == (failure == 0 ? ERR_FILE_NOT_FOUND : ERR_FILE_CANT_WRITE));
		if (failure == 0) {
			CHECK(result.error_message == "FFmpeg not found in system PATH");
		} else if (failure == 2) {
			CHECK(result.error_message == "FFmpeg failed with exit code 7");
		}
		CHECK_FALSE(result.intermediate_files_cleaned);
		CHECK(result.output_file_path.is_empty());
		CHECK(FileAccess::get_file_as_string(output) == "previous recording");
		CHECK(FileAccess::get_file_as_bytes(video) == original_video);
		CHECK(FileAccess::get_file_as_bytes(audio) == original_audio);
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	REQUIRE(driver.init(8000, AudioDriver::SPEAKER_MODE_STEREO) == OK);
	{
		IndependentVideoRecorder video;
		IndependentVideoRecorder::RecordingConfig video_config;
		video_config.video_width = 4;
		video_config.video_height = 4;
		REQUIRE(video.initialize(&buffer, files.file("video.avi"), video_config) == OK);
		video.update_config(video_config);
		REQUIRE(video.start_recording() == OK);
		auto frame = Image::create_empty(4, 4, false, Image::FORMAT_RGB8);
		frame->fill(Color(0.5, 0.25, 0));
		buffer.update_frame(frame, 1, 0);
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
	auto config = ObsStyleMovieWriter::get_standard_config();
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
	for (const char *name : { "obs_video_fps", "obs_video_quality", "obs_audio_sample_rate", "obs_audio_channels", "obs_enable_timestamp_chunks", "obs_enable_repeat_frame_marking", "obs_enable_debug_output", "obs_enable_post_merge", "obs_keep_intermediate_files", "obs_ffmpeg_path" }) {
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
	CHECK(config.enable_timestamp_chunks);
	CHECK(config.enable_repeat_frame_marking);
	CHECK_FALSE(config.enable_debug_output);
	CHECK(config.enable_post_merge);
	CHECK_FALSE(config.keep_intermediate_files);
	CHECK(config.ffmpeg_path == "ffmpeg");
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
	config.method = PostMergeProcessor::METHOD_CUSTOM_AVI;
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
