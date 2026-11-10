/**************************************************************************/
/*  thread_safe_frame_buffer.h                                           */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/

#ifndef SPX_THREAD_SAFE_FRAME_BUFFER_H
#define SPX_THREAD_SAFE_FRAME_BUFFER_H

#include "core/io/image.h"
#include "core/os/mutex.h"

/**
 * Publishes the latest image reference and sequence as one snapshot.
 * Readers retain their image even after a later publication or reset.
 */
class ThreadSafeFrameBuffer {
public:
	struct FrameData {
		Ref<Image> image; // Image data
		uint32_t frame_sequence = 0; // Game frame sequence number
	};

private:
	mutable Mutex buffer_mutex;
	FrameData current_frame;
	uint64_t total_updates = 0;

public:
	/**
	 * Called by the game thread: update the frame.
	 * @param new_frame New image frame
	 * @param sequence Game frame sequence number
	 */
	void update_frame(const Ref<Image> &new_frame, uint32_t sequence);

	/**
	 * Called by the recording thread: get the stable frame.
	 * @return Current stable frame data
	 */
	FrameData get_current_frame() const;

	/**
	 * Get statistics.
	 */
	uint64_t get_total_updates() const;

	/**
	 * Reset the buffer.
	 */
	void reset();
};

#endif // SPX_THREAD_SAFE_FRAME_BUFFER_H
