/**************************************************************************/
/*  audio_driver_hybrid.h                                                 */
/**************************************************************************/

#ifndef SPX_AUDIO_DRIVER_HYBRID_H
#define SPX_AUDIO_DRIVER_HYBRID_H

#include "core/math/audio_frame.h"
#include "core/os/mutex.h"
#include "core/templates/ring_buffer.h"
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
	AudioDriver::SpeakerMode speaker_mode = AudioDriver::SPEAKER_MODE_STEREO;
	int channels = 2;

	mutable Mutex data_mutex;
	RingBuffer<AudioFrame> capture_buffer;
	int buffer_power = 0;
	float buffer_length_seconds = 0.2f;

	Vector<IndependentAudioRecorder *> registered_recorders;
	mutable Mutex recorders_mutex;

	Ref<AudioEffect> capture_effect;
	SpxAudioCaptureEffect *capture_effect_owner = nullptr;
	int capture_bus = -1;

	void _clear_capture_buffer();
	void _remove_capture_effect();

public:
	HybridAudioDriver() = default;
	~HybridAudioDriver();

	Error init(int p_mix_rate, AudioDriver::SpeakerMode p_speaker_mode);
	void start();
	void finish();

	void enable_recording(bool p_enable);
	bool is_recording_enabled() const { return recording_enabled.is_set(); }

	int get_channels() const { return channels; }
	int get_mix_rate() const { return mix_rate; }

	void capture_audio_data(const int32_t *p_buffer, int p_frames, int p_channel_pairs);
	int get_captured_audio_data(int32_t *p_output_buffer, int p_requested_frames);

	int get_available_frames() const;
	bool has_audio_data() const;

	void register_audio_recorder(IndependentAudioRecorder *p_recorder);
	void unregister_audio_recorder(IndependentAudioRecorder *p_recorder);
	int get_registered_recorder_count() const;
};

#endif // SPX_AUDIO_DRIVER_HYBRID_H
