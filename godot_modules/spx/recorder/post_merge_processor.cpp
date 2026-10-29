/**************************************************************************/
/*  post_merge_processor.cpp                                             */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/

#include "post_merge_processor.h"
#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/os/os.h"
#include "core/os/time.h"
#include "movie_utils.h"

PostMergeProcessor::PostMergeProcessor() {
	// Initialize with default configuration
}

PostMergeProcessor::~PostMergeProcessor() {
}

void PostMergeProcessor::set_config(const MergeConfig &p_config) {
	config = p_config;
}

PostMergeProcessor::MergeResult PostMergeProcessor::merge_files(const String &video_path, const String &audio_path, const String &output_path) {
	MergeResult result;
	uint64_t start_time = OS::get_singleton()->get_ticks_usec();

	if (config.enable_debug_output && MovieDebugUtils::is_stdout_verbose()) {
		print_line("PostMergeProcessor: Starting merge operation");
		print_line("  Video file: " + video_path);
		print_line("  Audio file: " + audio_path);
		print_line("  Output file: " + output_path);
		print_line("  Method: " + get_method_name(config.method));
	}

	// Validate merge request
	Error validation_error = validate_merge_request(video_path, audio_path, output_path);
	if (validation_error != OK) {
		result.error_code = validation_error;
		result.error_message = "Validation failed";
		return result;
	}

	// Execute merge based on selected method
	switch (config.method) {
		case METHOD_FFMPEG_SYSTEM:
			result.error_code = ffmpeg_system_merge(video_path, audio_path, output_path, result);
			break;

		case METHOD_CUSTOM_AVI:
			result.error_code = custom_avi_merge(video_path, audio_path, output_path, result);
			break;

		case METHOD_NONE:
			result.error_code = OK;
			result.output_file_path = video_path; // Return video file as primary output
			break;

		default:
			result.error_code = ERR_PARAMETER_RANGE_ERROR;
			result.error_message = "Unknown merge method";
			break;
	}

	// Calculate merge duration
	uint64_t end_time = OS::get_singleton()->get_ticks_usec();
	result.merge_duration_seconds = (end_time - start_time) / 1000000.0f;

	// Clean up intermediate files if requested and merge was successful
	if (result.error_code == OK && config.keep_intermediate_files == false && config.method != METHOD_NONE) {
		Error cleanup_error = cleanup_intermediate_files(video_path, audio_path);
		result.intermediate_files_cleaned = (cleanup_error == OK);

		if (config.enable_debug_output && MovieDebugUtils::is_stdout_verbose()) {
			if (result.intermediate_files_cleaned) {
				print_line("PostMergeProcessor: Intermediate files cleaned up successfully");
			} else {
				print_line("PostMergeProcessor: Warning - Failed to clean up intermediate files");
			}
		}
	}

	// Store output file path
	if (result.error_code == OK && config.method != METHOD_NONE) {
		result.output_file_path = output_path;
	}

	if (config.enable_debug_output) {
		if (MovieDebugUtils::is_stdout_verbose()) {
			print_line(String("PostMergeProcessor: Merge completed in ") + String::num(result.merge_duration_seconds, 2) + " seconds");
		}
		if (result.error_code != OK) {
			print_line("PostMergeProcessor: Merge failed - " + result.error_message);
		}
	}

	return result;
}

Error PostMergeProcessor::ffmpeg_system_merge(const String &video_path, const String &audio_path, const String &output_path, MergeResult &result) {
#ifdef WEB_ENABLED
	// Web platform doesn't support system command execution
	result.error_message = "FFmpeg system merge not supported on Web platform";
	return ERR_UNAVAILABLE;
#else
	if (!check_ffmpeg_availability()) {
		result.error_message = "FFmpeg not found in system PATH";
		return ERR_FILE_NOT_FOUND;
	}

	// Build FFmpeg command arguments
	List<String> args;
	args.push_back("-i");
	args.push_back(video_path);
	args.push_back("-i");
	args.push_back(audio_path);
	args.push_back("-c");
	args.push_back("copy"); // Stream copy, no re-encoding
	args.push_back(output_path);
	args.push_back("-y"); // Overwrite output file if exists

	// Execute FFmpeg command
	String stdout_output;
	int exit_code = -1;
	Error execute_error = OS::get_singleton()->execute(config.ffmpeg_path, args, &stdout_output, &exit_code);

	if (execute_error != OK || exit_code != 0) {
		result.error_message = String("FFmpeg failed with exit code ") + String::num_int64(exit_code);
		if (!stdout_output.is_empty()) {
			result.error_message += " - " + stdout_output;
		}
		return ERR_FILE_CANT_WRITE;
	}

	// Verify output file was created
	if (!file_exists(output_path)) {
		result.error_message = "Output file was not created by FFmpeg";
		return ERR_FILE_CANT_WRITE;
	}

	return OK;
#endif
}

Error PostMergeProcessor::custom_avi_merge(const String &video_path, const String &audio_path, const String &output_path, MergeResult &result) {
	uint64_t merge_start_time = OS::get_singleton()->get_ticks_usec();

	// Parse input AVI files
	AviFileInfo video_info, audio_info;

	Error video_parse_error = parse_avi_file(video_path, video_info);
	if (video_parse_error != OK) {
		result.error_message = "Failed to parse video AVI file: " + video_path;
		return video_parse_error;
	}

	Error audio_parse_error = parse_avi_file(audio_path, audio_info);
	if (audio_parse_error != OK) {
		result.error_message = "Failed to parse audio AVI file: " + audio_path;
		return audio_parse_error;
	}

	ERR_FAIL_COND_V(memcmp(video_info.streams_info[0].fourcc_type, "vids", 4) != 0 ||
			memcmp(audio_info.streams_info[0].fourcc_type, "auds", 4) != 0, ERR_FILE_CORRUPT);

	// Create output file
	Ref<FileAccess> output_file = FileAccess::open(output_path, FileAccess::WRITE);
	if (output_file.is_null()) {
		result.error_message = "Cannot create output file: " + output_path;
		return ERR_FILE_CANT_WRITE;
	}

	// Write merged AVI header
	Error header_error = write_merged_avi_header(output_file, video_info, audio_info);
	if (header_error != OK) {
		result.error_message = "Failed to write merged AVI header";
		output_file->close();
		return header_error;
	}

	// Interleave video and audio data.
	Error interleave_error = interleave_avi_data(video_path, audio_path, output_file, video_info, audio_info);

	if (interleave_error != OK) {
		result.error_message = "Failed to interleave video and audio data";
		output_file->close();
		return interleave_error;
	}

	// Update file size in RIFF header
	uint64_t final_size = output_file->get_position();
	output_file->seek(4);
	output_file->store_32((uint32_t)(final_size - 8));

	output_file->close();

	// Calculate merge duration
	uint64_t merge_end_time = OS::get_singleton()->get_ticks_usec();
	result.merge_duration_seconds = (merge_end_time - merge_start_time) / 1000000.0f;
	result.output_file_path = output_path;

	if (config.enable_debug_output && MovieDebugUtils::is_stdout_verbose()) {
		print_line("PostMergeProcessor: Custom AVI merge completed successfully");
		print_line("  Output size: " + String::num_int64(get_file_size(output_path)) + " bytes");
		print_line("  Merge time: " + String::num_real(result.merge_duration_seconds) + " seconds");
	}

	return OK;
}

bool PostMergeProcessor::check_ffmpeg_availability() {
#ifdef WEB_ENABLED
	return false;
#else
	// Try to execute ffmpeg -version to check availability
	List<String> args;
	args.push_back("-version");

	String output;
	int exit_code = -1;
	Error error = OS::get_singleton()->execute(config.ffmpeg_path, args, &output, &exit_code);

	return error == OK && exit_code == 0 && output.contains("ffmpeg");
#endif
}

bool PostMergeProcessor::file_exists(const String &path) {
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	return file.is_valid();
}

Error PostMergeProcessor::cleanup_intermediate_files(const String &video_path, const String &audio_path) {
	Error video_error = OK;
	Error audio_error = OK;

	if (file_exists(video_path)) {
		String video_dir = video_path.get_base_dir();
		Ref<DirAccess> dir = DirAccess::open(video_dir);
		if (dir.is_valid()) {
			video_error = dir->remove(video_path.get_file());
		} else {
			video_error = ERR_FILE_CANT_OPEN;
		}
	}

	if (file_exists(audio_path)) {
		String audio_dir = audio_path.get_base_dir();
		Ref<DirAccess> dir = DirAccess::open(audio_dir);
		if (dir.is_valid()) {
			audio_error = dir->remove(audio_path.get_file());
		} else {
			audio_error = ERR_FILE_CANT_OPEN;
		}
	}

	// Return error if either cleanup failed
	if (video_error != OK) {
		return video_error;
	}
	if (audio_error != OK) {
		return audio_error;
	}

	return OK;
}

uint64_t PostMergeProcessor::get_file_size(const String &path) {
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	if (file.is_valid()) {
		return file->get_length();
	}
	return 0;
}

// AVI file parsing implementation
Error PostMergeProcessor::parse_avi_file(const String &file_path, AviFileInfo &avi_info) {
	Ref<FileAccess> file = FileAccess::open(file_path, FileAccess::READ);
	if (file.is_null()) {
		return ERR_FILE_CANT_OPEN;
	}

	const uint64_t file_size = file->get_length();
	ERR_FAIL_COND_V(file_size < 12, ERR_FILE_CORRUPT);

	// Read RIFF header
	char riff_header[4];
	file->get_buffer((uint8_t *)riff_header, 4);
	if (memcmp(riff_header, "RIFF", 4) != 0) {
		return ERR_FILE_CORRUPT;
	}

	const uint32_t riff_size = file->get_32();
	ERR_FAIL_COND_V(riff_size < 4 || uint64_t(riff_size) + 8 != file_size, ERR_FILE_CORRUPT);

	char avi_header[4];
	file->get_buffer((uint8_t *)avi_header, 4);
	if (memcmp(avi_header, "AVI ", 4) != 0) {
		return ERR_FILE_CORRUPT;
	}

	bool found_hdrl = false;
	bool found_movi = false;
	bool found_index = false;
	for (uint64_t pos = 12; pos < file_size;) {
		ERR_FAIL_COND_V(file_size - pos < 8, ERR_FILE_CORRUPT);
		file->seek(pos);
		char fourcc[4];
		file->get_buffer((uint8_t *)fourcc, 4);
		const uint32_t chunk_size = file->get_32();
		const uint64_t padded_size = uint64_t(chunk_size) + (chunk_size & 1);
		ERR_FAIL_COND_V(padded_size > file_size - file->get_position(), ERR_FILE_CORRUPT);

		if (memcmp(fourcc, "LIST", 4) == 0) {
			ERR_FAIL_COND_V(chunk_size < 4, ERR_FILE_CORRUPT);
			char list_type[4];
			file->get_buffer((uint8_t *)list_type, 4);
			if (memcmp(list_type, "hdrl", 4) == 0) {
				ERR_FAIL_COND_V(found_hdrl || found_movi, ERR_FILE_CORRUPT);
				Error error = parse_hdrl_chunk(file, avi_info, chunk_size - 4);
				if (error != OK) {
					return error;
				}
				found_hdrl = true;
			} else if (memcmp(list_type, "movi", 4) == 0) {
				ERR_FAIL_COND_V(!found_hdrl || found_movi, ERR_FILE_CORRUPT);
				avi_info.movi_list_offset = pos;
				avi_info.movi_data_offset = pos + 12;
				avi_info.movi_data_size = chunk_size - 4;
				found_movi = true;
			}
		} else if (memcmp(fourcc, "idx1", 4) == 0) {
			ERR_FAIL_COND_V(!found_movi || found_index, ERR_FILE_CORRUPT);
			Error error = parse_idx1_chunk(file, avi_info, chunk_size);
			if (error != OK) {
				return error;
			}
			found_index = true;
		}
		pos += 8 + padded_size;
	}

	ERR_FAIL_COND_V(!found_hdrl || !found_movi || avi_info.streams != 1 || avi_info.streams_info.size() != 1, ERR_FILE_CORRUPT);
	const AviFileInfo::StreamInfo &stream = avi_info.streams_info[0];
	ERR_FAIL_COND_V(stream.scale == 0 || stream.rate < stream.scale, ERR_FILE_CORRUPT);
	if (memcmp(stream.fourcc_type, "vids", 4) == 0) {
		ERR_FAIL_COND_V(avi_info.microsec_per_frame == 0 || avi_info.width == 0 || avi_info.height == 0, ERR_FILE_CORRUPT);
	} else if (memcmp(stream.fourcc_type, "auds", 4) == 0) {
		ERR_FAIL_COND_V(stream.sample_size == 0 || stream.sample_size % 4 != 0, ERR_FILE_CORRUPT);
	} else {
		return ERR_FILE_CORRUPT;
	}

	if (avi_info.index_entries.is_empty()) {
		return scan_movi_chunks(file, avi_info, avi_info.index_entries);
	}
	// Our single-stream writers index every movi chunk in file order.
	uint64_t expected_offset = 0;
	for (const AviFileInfo::IndexEntry &entry : avi_info.index_entries) {
		ERR_FAIL_COND_V(entry.chunk_offset != expected_offset, ERR_FILE_CORRUPT);
		Error error = validate_movi_chunk(file, avi_info, entry);
		if (error != OK) {
			return error;
		}
		expected_offset += 8 + uint64_t(entry.chunk_size) + (entry.chunk_size & 1);
	}
	ERR_FAIL_COND_V(expected_offset != avi_info.movi_data_size, ERR_FILE_CORRUPT);
	return OK;
}

Error PostMergeProcessor::parse_hdrl_chunk(Ref<FileAccess> file, AviFileInfo &avi_info, uint32_t chunk_size) {
	uint64_t hdrl_start = file->get_position();
	uint64_t chunk_end = hdrl_start + chunk_size;
	ERR_FAIL_COND_V(chunk_size < 64 || chunk_end > file->get_length(), ERR_FILE_CORRUPT);

	// First, look for avih chunk
	char chunk_fourcc[4];
	file->get_buffer((uint8_t *)chunk_fourcc, 4);
	uint32_t sub_chunk_size = file->get_32();
	ERR_FAIL_COND_V(sub_chunk_size < 56 || uint64_t(sub_chunk_size) + (sub_chunk_size & 1) > chunk_end - file->get_position(), ERR_FILE_CORRUPT);

	if (memcmp(chunk_fourcc, "avih", 4) == 0) {
		// Parse main AVI header
		avi_info.microsec_per_frame = file->get_32();
		avi_info.max_bytes_per_sec = file->get_32();
		file->get_32(); // padding_granularity
		file->get_32(); // flags
		avi_info.total_frames = file->get_32();
		file->get_32(); // initial_frames
		avi_info.streams = file->get_32();
		file->get_32(); // suggested_buffer_size
		avi_info.width = file->get_32();
		avi_info.height = file->get_32();
		file->seek(hdrl_start + 8 + sub_chunk_size + (sub_chunk_size & 1));
	} else {
		return ERR_FILE_CORRUPT;
	}

	// Look for LIST strl chunk(s)
	while (file->get_position() < chunk_end) {
		// Check if we have enough bytes for another chunk
		ERR_FAIL_COND_V(chunk_end - file->get_position() < 8, ERR_FILE_CORRUPT);

		uint64_t chunk_start = file->get_position();
		file->get_buffer((uint8_t *)chunk_fourcc, 4);
		sub_chunk_size = file->get_32();
		ERR_FAIL_COND_V(uint64_t(sub_chunk_size) + (sub_chunk_size & 1) > chunk_end - file->get_position(), ERR_FILE_CORRUPT);

		if (memcmp(chunk_fourcc, "LIST", 4) == 0) {
			ERR_FAIL_COND_V(sub_chunk_size < 4, ERR_FILE_CORRUPT);
			char list_type[4];
			file->get_buffer((uint8_t *)list_type, 4);

			if (memcmp(list_type, "strl", 4) == 0) {
				// Parse stream header
				AviFileInfo::StreamInfo stream_info;
				Error stream_error = parse_stream_header(file, stream_info, sub_chunk_size - 4);
				if (stream_error != OK) {
					return stream_error;
				}
				avi_info.streams_info.push_back(stream_info);
			}
		}
		file->seek(chunk_start + 8 + sub_chunk_size + (sub_chunk_size & 1));
	}

	return OK;
}

Error PostMergeProcessor::parse_stream_header(Ref<FileAccess> file, AviFileInfo::StreamInfo &stream_info, uint32_t chunk_size) {
	uint64_t strl_start = file->get_position();
	uint64_t chunk_end = strl_start + chunk_size;
	ERR_FAIL_COND_V(chunk_size < 56 || chunk_end > file->get_length(), ERR_FILE_CORRUPT);

	// Look for strh chunk first
	char chunk_fourcc[4];
	file->get_buffer((uint8_t *)chunk_fourcc, 4);
	uint32_t sub_chunk_size = file->get_32();

	ERR_FAIL_COND_V(sub_chunk_size < 48 || uint64_t(sub_chunk_size) + (sub_chunk_size & 1) > chunk_end - file->get_position(), ERR_FILE_CORRUPT);
	if (memcmp(chunk_fourcc, "strh", 4) == 0) {
		// Parse stream header
		file->get_buffer((uint8_t *)stream_info.fourcc_type, 4);
		file->get_buffer((uint8_t *)stream_info.fourcc_handler, 4);
		file->get_32(); // flags
		file->get_32(); // priority + language
		file->get_32(); // initial_frames
		stream_info.scale = file->get_32();
		stream_info.rate = file->get_32();
		file->get_32(); // start
		stream_info.length = file->get_32();
		file->get_32(); // suggested_buffer_size
		file->get_32(); // quality
		stream_info.sample_size = file->get_32();
		// Skip remaining strh fields
		file->seek(strl_start + 8 + sub_chunk_size + (sub_chunk_size & 1));
	} else {
		if (config.enable_debug_output) {
			print_line("PostMergeProcessor: Expected strh chunk but found something else");
		}
		return ERR_FILE_CORRUPT;
	}

	// Our writers always provide a complete stream format immediately after strh.
	ERR_FAIL_COND_V(chunk_end - file->get_position() < 8, ERR_FILE_CORRUPT);
	file->get_buffer((uint8_t *)chunk_fourcc, 4);
	sub_chunk_size = file->get_32();
	const uint32_t format_size = memcmp(stream_info.fourcc_type, "vids", 4) == 0 ? 40 : 16;
	ERR_FAIL_COND_V(memcmp(chunk_fourcc, "strf", 4) != 0 || sub_chunk_size < format_size ||
			uint64_t(sub_chunk_size) + (sub_chunk_size & 1) > chunk_end - file->get_position(), ERR_FILE_CORRUPT);
	if (config.enable_debug_output) {
		print_line("PostMergeProcessor: Found strf chunk, size=" + String::num_int64(sub_chunk_size));
	}
	file->seek(file->get_position() + sub_chunk_size + (sub_chunk_size & 1));

	return OK;
}

Error PostMergeProcessor::parse_idx1_chunk(Ref<FileAccess> file, AviFileInfo &avi_info, uint32_t chunk_size) {
	ERR_FAIL_COND_V(chunk_size % 16 != 0 || chunk_size > file->get_length() - file->get_position(), ERR_FILE_CORRUPT);
	uint32_t entry_count = chunk_size / 16; // Each index entry is 16 bytes

	if (config.enable_debug_output) {
		print_line("PostMergeProcessor: Parsing " + String::num_int64(entry_count) + " index entries");
	}

	for (uint32_t i = 0; i < entry_count; i++) {
		AviFileInfo::IndexEntry entry;
		file->get_buffer((uint8_t *)entry.fourcc, 4);
		entry.flags = file->get_32();
		// idx1 offsets include the four-byte movi type; internal offsets start at the first chunk.
		const uint32_t offset = file->get_32();
		ERR_FAIL_COND_V(offset < 4, ERR_FILE_CORRUPT);
		entry.chunk_offset = offset - 4;
		entry.chunk_size = file->get_32();

		avi_info.index_entries.push_back(entry);
	}

	return OK;
}

bool PostMergeProcessor::is_method_available(MergeMethod method) {
	switch (method) {
		case METHOD_FFMPEG_SYSTEM:
#ifdef WEB_ENABLED
			return false;
#else
			return check_ffmpeg_availability();
#endif

		case METHOD_CUSTOM_AVI:
			// Always available (Phase 2)
			return true; // Custom AVI merge implementation

		case METHOD_NONE:
			return true;

		default:
			return false;
	}
}

PostMergeProcessor::MergeMethod PostMergeProcessor::get_recommended_method() {
#ifdef WEB_ENABLED
	return METHOD_NONE; // Web platform keeps separate files
#else
	return METHOD_CUSTOM_AVI;
#endif
}

String PostMergeProcessor::get_method_name(MergeMethod method) {
	switch (method) {
		case METHOD_FFMPEG_SYSTEM:
			return "FFmpeg System Command";
		case METHOD_CUSTOM_AVI:
			return "Custom AVI Merge";
		case METHOD_NONE:
			return "No Merge";
		default:
			return "Unknown";
	}
}

Error PostMergeProcessor::validate_merge_request(const String &video_path, const String &audio_path, const String &output_path) {
	// Check if method is available
	if (!is_method_available(config.method)) {
		return ERR_UNAVAILABLE;
	}

	// For non-merge method, skip file validation
	if (config.method == METHOD_NONE) {
		return OK;
	}

	// Check input files exist
	if (!file_exists(video_path)) {
		return ERR_FILE_NOT_FOUND;
	}

	if (!file_exists(audio_path)) {
		return ERR_FILE_NOT_FOUND;
	}

	// Check input files are not empty
	if (get_file_size(video_path) == 0) {
		return ERR_FILE_CORRUPT;
	}

	if (get_file_size(audio_path) == 0) {
		return ERR_FILE_CORRUPT;
	}

	// Check output path is valid (directory exists)
	String output_dir = output_path.get_base_dir();
	if (!output_dir.is_empty()) {
		Ref<DirAccess> dir = DirAccess::open(output_dir);
		if (!dir.is_valid()) {
			return ERR_FILE_BAD_PATH;
		}
	}

	return OK;
}

// AVI header merging implementation
Error PostMergeProcessor::write_merged_avi_header(Ref<FileAccess> output_file, const AviFileInfo &video_info, const AviFileInfo &audio_info) {
	// Write RIFF header
	output_file->store_buffer((const uint8_t *)"RIFF", 4);
	output_file->store_32(0); // Placeholder for file size
	output_file->store_buffer((const uint8_t *)"AVI ", 4);

	// Write header list
	output_file->store_buffer((const uint8_t *)"LIST", 4);
	uint64_t hdrl_size_pos = output_file->get_position();
	output_file->store_32(0); // Placeholder for hdrl size
	output_file->store_buffer((const uint8_t *)"hdrl", 4);
	uint64_t hdrl_start = output_file->get_position();

	// Write main AVI header (avih)
	output_file->store_buffer((const uint8_t *)"avih", 4);
	output_file->store_32(56); // avih chunk size

	// Use video timing as primary
	output_file->store_32(video_info.microsec_per_frame); // μs per frame
	output_file->store_32(video_info.max_bytes_per_sec + 44100 * 4 * 2); // Est. bytes/sec
	output_file->store_32(0); // padding_granularity
	output_file->store_32(0x10); // flags (HASINDEX)
	output_file->store_32(video_info.total_frames); // total frames
	output_file->store_32(0); // initial_frames
	output_file->store_32(2); // total streams (video + audio)
	output_file->store_32(0); // suggested_buffer_size
	output_file->store_32(video_info.width);
	output_file->store_32(video_info.height);
	// Reserved fields
	for (int i = 0; i < 4; i++) {
		output_file->store_32(0);
	}

	// Write video stream header
	write_video_stream_header(output_file, video_info);

	// Write audio stream header - convert 32-bit to 16-bit format
	write_audio_stream_header(output_file, audio_info);

	// Update hdrl size
	uint64_t hdrl_end = output_file->get_position();
	uint32_t hdrl_size = hdrl_end - hdrl_start;
	output_file->seek(hdrl_size_pos);
	output_file->store_32(hdrl_size + 4); // +4 for 'hdrl' fourcc
	output_file->seek(hdrl_end);

	return OK;
}

Error PostMergeProcessor::write_video_stream_header(Ref<FileAccess> output_file, const AviFileInfo &video_info) {
	// VIDEO STREAM (Stream 0)
	output_file->store_buffer((const uint8_t *)"LIST", 4);
	output_file->store_32(116); // strl type + strh + strf
	output_file->store_buffer((const uint8_t *)"strl", 4);

	// Video stream header (strh)
	output_file->store_buffer((const uint8_t *)"strh", 4);
	output_file->store_32(56); // strh size

	output_file->store_buffer((const uint8_t *)"vids", 4); // Stream type
	output_file->store_buffer((const uint8_t *)"MJPG", 4); // Handler
	output_file->store_32(0); // flags
	output_file->store_16(0); // priority
	output_file->store_16(0); // language
	output_file->store_32(0); // initial_frames
	output_file->store_32(1); // scale
	output_file->store_32(1000000 / video_info.microsec_per_frame); // rate (FPS)
	output_file->store_32(0); // start
	output_file->store_32(video_info.total_frames); // length
	output_file->store_32(0); // suggested_buffer_size
	output_file->store_32(0); // quality
	output_file->store_32(0); // sample_size (variable for video)
	output_file->store_16(0); // left
	output_file->store_16(0); // top
	output_file->store_16(video_info.width); // right
	output_file->store_16(video_info.height); // bottom

	// Video stream format (strf) - BITMAPINFOHEADER
	output_file->store_buffer((const uint8_t *)"strf", 4);
	output_file->store_32(40); // Size

	output_file->store_32(40); // biSize
	output_file->store_32(video_info.width); // biWidth
	output_file->store_32(video_info.height); // biHeight
	output_file->store_16(1); // biPlanes
	output_file->store_16(24); // biBitCount
	output_file->store_buffer((const uint8_t *)"MJPG", 4); // biCompression
	output_file->store_32(((video_info.width * 24 / 8 + 3) & 0xFFFFFFFC) * video_info.height); // biSizeImage
	output_file->store_32(0); // biXPelsPerMeter
	output_file->store_32(0); // biYPelsPerMeter
	output_file->store_32(0); // biClrUsed
	output_file->store_32(0); // biClrImportant

	// OpenDML header for large file support
	output_file->store_buffer((const uint8_t *)"LIST", 4);
	output_file->store_32(16); // Size
	output_file->store_buffer((const uint8_t *)"odml", 4);
	output_file->store_buffer((const uint8_t *)"dmlh", 4);
	output_file->store_32(4); // Size
	output_file->store_32(video_info.total_frames); // Total frames

	return OK;
}

Error PostMergeProcessor::write_audio_stream_header(Ref<FileAccess> output_file, const AviFileInfo &audio_info) {
	const AviFileInfo::StreamInfo &audio_stream = audio_info.streams_info[0];
	const uint32_t sample_rate = audio_stream.rate / audio_stream.scale;
	const uint32_t channels = audio_stream.sample_size / 4; // SPX audio writers store 32-bit PCM.

	// AUDIO STREAM (Stream 1) - Convert to 16-bit
	output_file->store_buffer((const uint8_t *)"LIST", 4);
	output_file->store_32(92); // strl type + strh + strf
	output_file->store_buffer((const uint8_t *)"strl", 4);

	// Audio stream header (strh)
	output_file->store_buffer((const uint8_t *)"strh", 4);
	output_file->store_32(56); // strh size

	output_file->store_buffer((const uint8_t *)"auds", 4); // Stream type
	output_file->store_32(0); // Handler (none for PCM)
	output_file->store_32(0); // flags
	output_file->store_16(0); // priority
	output_file->store_16(0); // language
	output_file->store_32(0); // initial_frames
	output_file->store_32(channels * 2); // scale (16-bit per channel)
	output_file->store_32(sample_rate * channels * 2); // rate (sample_rate * block_align)
	output_file->store_32(0); // start
	output_file->store_32(audio_stream.length); // length (in samples)
	output_file->store_32(12288); // suggested_buffer_size
	output_file->store_32(0xFFFFFFFF); // quality (max for audio)
	output_file->store_32(channels * 2); // sample_size (16-bit per channel)
	output_file->store_32(0); // rcFrame left/top
	output_file->store_32(0); // rcFrame right/bottom

	// Audio stream format (strf) - WAVEFORMATEX
	output_file->store_buffer((const uint8_t *)"strf", 4);
	output_file->store_32(16); // Size (standard PCM format)

	output_file->store_16(1); // wFormatTag (PCM)
	output_file->store_16(channels); // nChannels
	output_file->store_32(sample_rate); // nSamplesPerSec
	output_file->store_32(sample_rate * channels * 2); // nAvgBytesPerSec (16-bit)
	output_file->store_16(channels * 2); // nBlockAlign (16-bit per channel)
	output_file->store_16(16); // wBitsPerSample

	return OK;
}

Error PostMergeProcessor::interleave_avi_data(const String &video_path, const String &audio_path, Ref<FileAccess> output_file, const AviFileInfo &video_info, const AviFileInfo &audio_info) {
	if (config.interleave_strategy == MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE && config.enable_debug_output) {
		print_line("PostMergeProcessor: Starting data interleaving");
		print_line("  Video movi offset: " + String::num_int64(video_info.movi_data_offset) + ", size: " + String::num_int64(video_info.movi_data_size));
		print_line("  Audio movi offset: " + String::num_int64(audio_info.movi_data_offset) + ", size: " + String::num_int64(audio_info.movi_data_size));
	}

	// Open input files
	Ref<FileAccess> video_file = FileAccess::open(video_path, FileAccess::READ);
	Ref<FileAccess> audio_file = FileAccess::open(audio_path, FileAccess::READ);

	if (video_file.is_null() || audio_file.is_null()) {
		return ERR_FILE_CANT_OPEN;
	}

	Vector<TimestampedChunk> all_chunks;
	if (config.interleave_strategy != MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE) {
		calculate_chunk_timestamps(video_info.index_entries, video_info, true, all_chunks);
		calculate_chunk_timestamps(audio_info.index_entries, audio_info, false, all_chunks);
		all_chunks.sort();
	}

	// Write movi list header
	output_file->store_buffer((const uint8_t *)"LIST", 4);
	uint64_t movi_size_pos = output_file->get_position();
	output_file->store_32(0); // Placeholder
	output_file->store_buffer((const uint8_t *)"movi", 4);
	uint64_t movi_start = output_file->get_position();

	Vector<AviFileInfo::IndexEntry> merged_index;

	uint32_t chunk_offset = 4; // Relative to movi data start
	if (config.interleave_strategy == MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE) {
		// Simple interleaving: alternate between video and audio chunks
		int video_idx = 0, audio_idx = 0;

		while (video_idx < video_info.index_entries.size() || audio_idx < audio_info.index_entries.size()) {
			for (bool is_video : { true, false }) {
				int &index = is_video ? video_idx : audio_idx;
				const AviFileInfo &info = is_video ? video_info : audio_info;
				if (index >= info.index_entries.size()) {
					continue;
				}
				const AviFileInfo::IndexEntry &entry = info.index_entries[index++];
				TimestampedChunk chunk{};
				chunk.file_offset = info.movi_data_offset + entry.chunk_offset;
				chunk.chunk_size = entry.chunk_size;
				Error error = write_avi_chunk(is_video ? video_file : audio_file, output_file, chunk, is_video, chunk_offset, merged_index);
				if (error != OK) {
					return error;
				}
			}
		}
	} else {
		for (const TimestampedChunk &chunk : all_chunks) {
			const bool is_video = chunk.type == TimestampedChunk::VIDEO_CHUNK;
			Error error = write_avi_chunk(is_video ? video_file : audio_file, output_file, chunk, is_video, chunk_offset, merged_index);
			if (error != OK) {
				return error;
			}
		}
	}

	// Update movi size
	uint64_t movi_end = output_file->get_position();
	uint32_t movi_size = movi_end - movi_start;
	output_file->seek(movi_size_pos);
	output_file->store_32(movi_size + 4); // +4 for 'movi' fourcc
	output_file->seek(movi_end);

	// Write index
	Error index_error = write_merged_avi_index(output_file, merged_index);
	if (index_error != OK) {
		return index_error;
	}

	if (config.interleave_strategy == MergeConfig::INTERLEAVE_SIMPLE_ALTERNATE) {
		print_line("PostMergeProcessor: Data interleaving completed");
		print_line("  Merged chunks: " + String::num_int64(merged_index.size()));
		print_line("  Movi size: " + String::num_int64(movi_size) + " bytes");
	}
	return OK;
}

Error PostMergeProcessor::validate_movi_chunk(Ref<FileAccess> file, const AviFileInfo &info, const AviFileInfo::IndexEntry &entry) {
	const uint64_t chunk_end = uint64_t(entry.chunk_offset) + 8 + entry.chunk_size + (entry.chunk_size & 1);
	ERR_FAIL_COND_V(chunk_end > info.movi_data_size, ERR_FILE_CORRUPT);
	file->seek(info.movi_data_offset + entry.chunk_offset);
	char fourcc[4];
	file->get_buffer((uint8_t *)fourcc, 4);
	ERR_FAIL_COND_V(memcmp(fourcc, entry.fourcc, 4) != 0 || file->get_32() != entry.chunk_size, ERR_FILE_CORRUPT);
	if (memcmp(info.streams_info[0].fourcc_type, "auds", 4) == 0) {
		ERR_FAIL_COND_V(memcmp(fourcc, "00wb", 4) != 0 || entry.chunk_size % info.streams_info[0].sample_size != 0, ERR_FILE_CORRUPT);
	} else {
		ERR_FAIL_COND_V(memcmp(fourcc, "00db", 4) != 0 && memcmp(fourcc, "00dc", 4) != 0, ERR_FILE_CORRUPT);
	}
	return OK;
}

Error PostMergeProcessor::scan_movi_chunks(Ref<FileAccess> file, const AviFileInfo &info, Vector<AviFileInfo::IndexEntry> &chunks) {
	uint64_t offset = 0;
	while (offset < info.movi_data_size) {
		ERR_FAIL_COND_V(info.movi_data_size - offset < 8, ERR_FILE_CORRUPT);
		file->seek(info.movi_data_offset + offset);
		AviFileInfo::IndexEntry entry{};
		file->get_buffer((uint8_t *)entry.fourcc, 4);
		entry.chunk_size = file->get_32();
		entry.chunk_offset = offset;
		entry.flags = entry.fourcc[2] == 'd' ? 0x10 : 0;
		Error error = validate_movi_chunk(file, info, entry);
		if (error != OK) {
			return error;
		}
		chunks.push_back(entry);
		offset += 8 + uint64_t(entry.chunk_size) + (entry.chunk_size & 1);
	}
	return OK;
}

Error PostMergeProcessor::write_merged_avi_index(Ref<FileAccess> output_file, const Vector<AviFileInfo::IndexEntry> &merged_index) {
	// Write idx1 chunk
	output_file->store_buffer((const uint8_t *)"idx1", 4);
	output_file->store_32(merged_index.size() * 16); // Each entry is 16 bytes

	for (const AviFileInfo::IndexEntry &entry : merged_index) {
		output_file->store_buffer((const uint8_t *)entry.fourcc, 4);
		output_file->store_32(entry.flags);
		output_file->store_32(entry.chunk_offset);
		output_file->store_32(entry.chunk_size);
	}

	return OK;
}

void PostMergeProcessor::calculate_chunk_timestamps(const Vector<AviFileInfo::IndexEntry> &chunks, const AviFileInfo &info, bool is_video, Vector<TimestampedChunk> &timestamped_chunks) {
	if (is_video) {
		// Video timestamp calculation
		uint64_t frame_duration_us = info.microsec_per_frame;

		for (int i = 0; i < chunks.size(); i++) {
			TimestampedChunk tc;
			tc.type = TimestampedChunk::VIDEO_CHUNK;
			tc.timestamp_us = i * frame_duration_us;
			tc.file_offset = info.movi_data_offset + chunks[i].chunk_offset;
			tc.chunk_size = chunks[i].chunk_size;
			tc.index = i;
			timestamped_chunks.push_back(tc);
		}

	} else {
		const AviFileInfo::StreamInfo &audio_stream = info.streams_info[0];
		const uint32_t sample_rate = audio_stream.rate / audio_stream.scale;

		// Calculate timestamp for each audio chunk
		uint64_t cumulative_samples = 0;

		for (int i = 0; i < chunks.size(); i++) {
			TimestampedChunk tc;
			tc.type = TimestampedChunk::AUDIO_CHUNK;

			// Calculate timestamp based on cumulative samples
			tc.timestamp_us = (cumulative_samples * 1000000) / sample_rate;
			tc.file_offset = info.movi_data_offset + chunks[i].chunk_offset;
			tc.chunk_size = chunks[i].chunk_size;
			tc.index = i;
			timestamped_chunks.push_back(tc);

			// Update cumulative samples
			uint32_t samples_in_chunk = chunks[i].chunk_size / audio_stream.sample_size;
			cumulative_samples += samples_in_chunk;
		}
	}
}

Error PostMergeProcessor::write_avi_chunk(Ref<FileAccess> input_file, Ref<FileAccess> output_file,
		const TimestampedChunk &chunk, bool is_video,
		uint32_t &chunk_offset, Vector<AviFileInfo::IndexEntry> &merged_index) {
	// Input files are reopened after parsing; check again before copying a payload.
	const uint64_t file_size = input_file->get_length();
	ERR_FAIL_COND_V(chunk.file_offset > file_size || file_size - chunk.file_offset < 8, ERR_FILE_CORRUPT);
	input_file->seek(chunk.file_offset);

	// Read chunk header
	char fourcc[4];
	input_file->get_buffer((uint8_t *)fourcc, 4);
	const uint32_t original_size = input_file->get_32();
	ERR_FAIL_COND_V(original_size != chunk.chunk_size || original_size > file_size - input_file->get_position(), ERR_FILE_CORRUPT);
	ERR_FAIL_COND_V(!is_video && original_size % 4 != 0, ERR_FILE_CORRUPT);

	uint32_t written_size = original_size;
	if (is_video) {
		// Write video chunk (keep original format)
		output_file->store_buffer((const uint8_t *)"00db", 4); // Stream 0, video
		output_file->store_32(original_size);

		// Copy data
		Vector<uint8_t> chunk_data;
		chunk_data.resize(original_size);
		input_file->get_buffer(chunk_data.ptrw(), original_size);
		output_file->store_buffer(chunk_data.ptr(), original_size);

		// Pad to even boundary
		if (original_size % 2 != 0) {
			output_file->store_8(0);
		}
	} else {
		// Audio chunk - convert 32-bit to 16-bit
		Vector<int32_t> audio_data_32;
		audio_data_32.resize(original_size / 4);
		input_file->get_buffer((uint8_t *)audio_data_32.ptrw(), original_size);

		// Convert to 16-bit
		Vector<int16_t> audio_data_16;
		audio_data_16.resize(audio_data_32.size());

		for (int i = 0; i < audio_data_32.size(); i++) {
			int32_t sample_32 = audio_data_32[i];
			int16_t sample_16 = (int16_t)CLAMP(sample_32 >> 16, -32768, 32767);
			audio_data_16.write[i] = sample_16;
		}

		written_size = audio_data_16.size() * 2;

		// Write audio chunk
		output_file->store_buffer((const uint8_t *)"01wb", 4); // Stream 1, audio
		output_file->store_32(written_size);
		output_file->store_buffer((const uint8_t *)audio_data_16.ptr(), written_size);
	}

	AviFileInfo::IndexEntry entry;
	memcpy(entry.fourcc, is_video ? "00db" : "01wb", 4);
	entry.flags = is_video ? 0x10 : 0; // AVIIF_KEYFRAME for video.
	entry.chunk_offset = chunk_offset;
	entry.chunk_size = written_size;
	merged_index.push_back(entry);
	// PCM conversion produces an even size; only video can require padding.
	chunk_offset += 8 + written_size + (written_size & 1);

	return OK;
}
