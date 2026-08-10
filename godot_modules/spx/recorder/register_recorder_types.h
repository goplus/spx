/**************************************************************************/
/*  register_recorder_types.h                                            */
/**************************************************************************/

#ifndef SPX_REGISTER_RECORDER_TYPES_H
#define SPX_REGISTER_RECORDER_TYPES_H

#include "core/string/ustring.h"

class SpxRealtimeRecorder;

void initialize_spx_recorder_core();
void uninitialize_spx_recorder_core();
void initialize_spx_recorder_servers();
void uninitialize_spx_recorder_servers();

bool spx_recorder_claims_movie(const String &p_path);
SpxRealtimeRecorder *spx_find_realtime_movie_recorder(const String &p_path);

#endif // SPX_REGISTER_RECORDER_TYPES_H
