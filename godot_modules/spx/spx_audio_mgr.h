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

#include "core/templates/hash_map.h"
#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include "scene/main/node.h"
#include "spx_audio.h"
#include "spx_object_mgr.h"

class SpxAudioMgr : public SpxObjectMgr<SpxAudio> {

private:
	// Main-thread-only routing index. Player state lives solely in SpxAudio.
	HashMap<GdInt, GdObj> aid_owners;
	GdInt g_audio_id = 0;

	SpxAudio *_get_aid_audio(GdInt aid);

public:

	void on_awake() override;
	void on_destroy() override;
	void on_update(float delta) override;
	void on_reset(int reset_code) override;

	SPX_BIND void stop_all();
	SPX_BIND GdObj create_audio();
	SPX_BIND void destroy_audio(GdObj obj);

	SPX_BIND void set_pitch(GdObj obj, GdFloat pitch);
	SPX_BIND GdFloat get_pitch(GdObj obj);
	SPX_BIND void set_pan(GdObj obj, GdFloat pan);
	SPX_BIND GdFloat get_pan(GdObj obj);
	SPX_BIND void set_volume(GdObj obj, GdFloat volume);
	SPX_BIND GdFloat get_volume(GdObj obj);

	// play audio and return the audioid
	SPX_BIND GdInt play_with_attenuation(GdObj obj, GdString path, GdObj owner_id, GdFloat attenuation, GdFloat max_distance);
	SPX_BIND GdInt play(GdObj obj, GdString path);
	SPX_BIND void pause(GdInt aid);
	SPX_BIND void resume(GdInt aid);
	SPX_BIND void stop(GdInt aid);
	SPX_BIND GdBool restart(GdInt aid);
	SPX_BIND void set_loop(GdInt aid, GdBool loop);
	SPX_BIND GdBool get_loop(GdInt aid);

	SPX_BIND GdFloat get_timer(GdInt aid);
	SPX_BIND void set_timer(GdInt aid, GdFloat time);
	SPX_BIND GdBool is_playing(GdInt aid);
};

#endif // SPX_AUDIO_MGR_H
