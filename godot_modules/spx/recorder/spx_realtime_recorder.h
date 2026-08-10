/**************************************************************************/
/*  spx_realtime_recorder.h                                               */
/**************************************************************************/

#ifndef SPX_REALTIME_RECORDER_H
#define SPX_REALTIME_RECORDER_H

#include "core/error/error_list.h"
#include "core/math/vector2i.h"
#include "core/string/ustring.h"

// Module-owned movie lifecycle used when SPX claims a command-line recording.
// This deliberately bypasses MovieWriter::add_frame(), whose audio path is
// tied to Godot's offline AudioDriverDummy.
class SpxRealtimeRecorder {
public:
	virtual ~SpxRealtimeRecorder() = default;

	virtual bool handles_realtime_file(const String &p_path) const = 0;
	virtual Error begin_realtime(const Size2i &p_movie_size, uint32_t p_fps, const String &p_base_path) = 0;
	virtual void add_realtime_frame() = 0;
	virtual void end_realtime() = 0;
};

#endif // SPX_REALTIME_RECORDER_H
