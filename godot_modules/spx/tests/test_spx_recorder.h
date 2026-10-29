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
#include "../recorder/post_merge_processor.h"
#include "../recorder/register_recorder_types.h"
#include "../recorder/simple_audio_writer.h"
#include "../recorder/simple_video_writer.h"
#include "../recorder/thread_safe_frame_buffer.h"
#include "core/io/marshalls.h"
#include "main/main_loop_phase_callback_bus.h"
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

TEST_CASE("[SceneTree][SPX] Realtime recording rejects invalid configuration before allocating workers") {
	TestSpx::TempDirectory files("spx_realtime_invalid");
	TestSpx::EngineScene scene;
	auto destroy = [](RealtimeWriter *value) { memdelete(value); };
	std::unique_ptr<RealtimeWriter, decltype(destroy)> writer(memnew(RealtimeWriter), destroy);
	for (int invalid = 0; invalid < 5; ++invalid) {
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
	bus.notify_movie_end();
	CHECK_FALSE(MovieRecorderManager::is_recording());
	CHECK_FALSE(bus.is_movie_route_resolved());
	MovieRecorderManager::shutdown();
	MovieRecorderManager::shutdown();
	CHECK_FALSE(MovieRecorderManager::is_initialized());
	MovieRecorderManager::initialize();
	CHECK(MovieRecorderManager::is_initialized());
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
	for (const char *tag : { "AVI ", "hdrl", "avih", "strh", "movi" }) {
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
			for (int damage = 0; damage < 3; ++damage) {
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
				} else {
					// Individually valid entries must preserve their recording order.
					for (int i = 0; i < 16; ++i) {
						uint8_t first = bytes[index + 8 + i];
						bytes.set(index + 8 + i, bytes[index + 24 + i]);
						bytes.set(index + 24 + i, first);
					}
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
		{ false, "strl", -4, 2 },
		{ false, "strl", -4, 8 },
		{ false, "strh", 4, 4 },
		{ false, "strh", 4, UINT32_MAX },
		{ false, "strf", 4, UINT32_MAX },
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

} // namespace TestSpxRecorder

#endif // TEST_SPX_RECORDER_H
