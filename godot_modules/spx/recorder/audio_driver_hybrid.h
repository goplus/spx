/**************************************************************************/
/*  audio_driver_hybrid.h                                                 */
/**************************************************************************/

#ifndef SPX_AUDIO_DRIVER_HYBRID_H
#define SPX_AUDIO_DRIVER_HYBRID_H

#include "core/os/mutex.h"
#include "core/templates/safe_refcount.h"
#include "core/templates/vector.h"
#include "servers/audio/audio_effect.h"
#include "servers/audio_server.h"

class IndependentAudioRecorder;
class SpxAudioCaptureEffect;

// Captures the final Master bus mix through a regular AudioEffect. It never
// replaces AudioDriver::singleton and requires no AudioServer fork hooks.
class HybridAudioDriver {
	SafeFlag recording_enabled;
	SafeFlag initialized;
	int mix_rate = 44100;

	Vector<IndependentAudioRecorder *> registered_recorders;
	mutable Mutex recorders_mutex;

	Ref<AudioEffect> capture_effect;
	SpxAudioCaptureEffect *capture_effect_owner = nullptr;
	static constexpr int MASTER_BUS = 0;

	void _remove_capture_effect();

public:
	HybridAudioDriver() = default;
	~HybridAudioDriver();

	Error init(int p_mix_rate);
	void finish();

	void enable_recording(bool p_enable);
	bool is_recording_enabled() const { return recording_enabled.is_set(); }

	int get_mix_rate() const { return mix_rate; }

	// Master mix PCM is interleaved stereo and is pushed directly to active recorders.
	void capture_audio_data(const int32_t *p_buffer, int p_frames);

	void register_audio_recorder(IndependentAudioRecorder *p_recorder);
	void unregister_audio_recorder(IndependentAudioRecorder *p_recorder);
	int get_registered_recorder_count() const;
};

#endif // SPX_AUDIO_DRIVER_HYBRID_H
