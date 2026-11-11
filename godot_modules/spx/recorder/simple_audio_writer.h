/**************************************************************************/
/*  simple_audio_writer.h                                                */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/

#ifndef SPX_SIMPLE_AUDIO_WRITER_H
#define SPX_SIMPLE_AUDIO_WRITER_H

#include "core/io/file_access.h"
#include "core/object/ref_counted.h"

class SimpleAudioWriter : public RefCounted {
	GDCLASS(SimpleAudioWriter, RefCounted);

private:
	Ref<FileAccess> f;
	uint32_t channels;

	uint64_t total_audio_frames_ofs;
	uint64_t movi_data_ofs;

	Vector<uint32_t> audio_chunk_sizes;

public:
	~SimpleAudioWriter();

	Error open(const String &p_path, uint32_t p_sample_rate, uint32_t p_channels);
	Error write_audio_chunk(const int32_t *p_audio_data, int p_frame_count);
	void close();

	uint32_t get_chunk_count() const { return audio_chunk_sizes.size(); }
};

#endif // SPX_SIMPLE_AUDIO_WRITER_H
