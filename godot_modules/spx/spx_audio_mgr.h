/**************************************************************************/
/*  spx_audio_mgr.h                                                       */
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

#ifndef SPX_AUDIO_MGR_H
#define SPX_AUDIO_MGR_H

#include "core/templates/list.h"
#include "core/templates/rb_map.h"
#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include "scene/main/node.h"
#include "spx_audio.h"
#include "spx_object_mgr.h"

// Forward declarations
class AudioStreamPlayer2D;

class SpxAudioMgr : public SpxObjectMgr<SpxAudio> {
	SPXCLASS(SpxAudioMgr, SpxObjectMgr<SpxAudio>)

private:
	// Additional mapping for audio instance IDs (aid) to audio objects
	RBMap<GdInt, SpxAudio *> aid_audios;
	mutable Mutex aid_mutex;
	GdInt g_audio_id;

	SpxAudio *_get_aid_audio(GdInt aid);

public:
	virtual ~SpxAudioMgr() = default;

	void on_awake() override;
	void on_destroy() override;
	void on_update(float delta) override;
	void on_reset(int reset_code) override;

	SPX_API void stop_all();
	SPX_API GdObj create_audio();
	SPX_API void destroy_audio(GdObj obj);

	SPX_API void set_pitch(GdObj obj, GdFloat pitch);
	SPX_API GdFloat get_pitch(GdObj obj);
	SPX_API void set_pan(GdObj obj, GdFloat pan);
	SPX_API GdFloat get_pan(GdObj obj);
	SPX_API void set_volume(GdObj obj, GdFloat volume);
	SPX_API GdFloat get_volume(GdObj obj);

	// play audio and return the audioid
	SPX_API GdInt play_with_attenuation(GdObj obj, GdString path, GdObj owner_id, GdFloat attenuation, GdFloat max_distance);
	SPX_API GdInt play(GdObj obj, GdString path);
	SPX_API void pause(GdInt aid);
	SPX_API void resume(GdInt aid);
	SPX_API void stop(GdInt aid);
	SPX_API GdBool restart(GdInt aid);
	SPX_API void set_loop(GdInt aid, GdBool loop);
	SPX_API GdBool get_loop(GdInt aid);

	SPX_API GdFloat get_timer(GdInt aid);
	SPX_API void set_timer(GdInt aid, GdFloat time);
	SPX_API GdBool is_playing(GdInt aid);
};

#endif // SPX_AUDIO_MGR_H
