/**************************************************************************/
/*  thread_safe_frame_buffer.cpp                                         */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/

#include "thread_safe_frame_buffer.h"

void ThreadSafeFrameBuffer::update_frame(const Ref<Image> &new_frame, uint32_t sequence) {
	if (new_frame.is_null()) {
		return;
	}

	MutexLock lock(buffer_mutex);
	current_frame.image = new_frame;
	current_frame.frame_sequence = sequence;
	total_updates++;
}

ThreadSafeFrameBuffer::FrameData ThreadSafeFrameBuffer::get_current_frame() const {
	MutexLock lock(buffer_mutex);

	return current_frame;
}

uint64_t ThreadSafeFrameBuffer::get_total_updates() const {
	MutexLock lock(buffer_mutex);
	return total_updates;
}

void ThreadSafeFrameBuffer::reset() {
	MutexLock lock(buffer_mutex);

	current_frame = FrameData();
	total_updates = 0;
}
