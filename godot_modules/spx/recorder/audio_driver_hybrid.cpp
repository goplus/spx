/**************************************************************************/
/*  audio_driver_hybrid.cpp                                               */
/**************************************************************************/

#include "audio_driver_hybrid.h"

#include "core/math/audio_frame.h"
#include "core/os/os.h"
#include "independent_audio_recorder.h"
#include "servers/audio/audio_effect.h"

class SpxAudioCaptureEffect;

class SpxAudioCaptureEffectInstance : public AudioEffectInstance {
	GDCLASS(SpxAudioCaptureEffectInstance, AudioEffectInstance);

	Ref<SpxAudioCaptureEffect> base;
	Vector<int32_t> conversion_buffer;

protected:
	static void _bind_methods() {}

public:
	void set_base(const Ref<SpxAudioCaptureEffect> &p_base) { base = p_base; }
	void process(const AudioFrame *p_src_frames, AudioFrame *p_dst_frames, int p_frame_count) override;
	bool process_silence() const override { return false; }
};

class SpxAudioCaptureEffect : public AudioEffect {
	GDCLASS(SpxAudioCaptureEffect, AudioEffect);
	friend class SpxAudioCaptureEffectInstance;

	mutable Mutex owner_mutex;
	HybridAudioDriver *owner = nullptr;

protected:
	static void _bind_methods() {}

public:
	explicit SpxAudioCaptureEffect(HybridAudioDriver *p_owner) :
			owner(p_owner) {}

	void detach() {
		MutexLock lock(owner_mutex);
		owner = nullptr;
	}

	Ref<AudioEffectInstance> instantiate() override {
		Ref<SpxAudioCaptureEffectInstance> instance;
		instance.instantiate();
		instance->set_base(Ref<SpxAudioCaptureEffect>(this));
		return instance;
	}
};

void SpxAudioCaptureEffectInstance::process(const AudioFrame *p_src_frames, AudioFrame *p_dst_frames, int p_frame_count) {
	for (int i = 0; i < p_frame_count; i++) {
		p_dst_frames[i] = p_src_frames[i];
	}

	if (base.is_null()) {
		return;
	}

	MutexLock lock(base->owner_mutex);
	if (base->owner == nullptr) {
		return;
	}

	// Clamp in double precision: float rounds INT32_MAX up past the signed range.
	conversion_buffer.resize(p_frame_count * 2);
	for (int i = 0; i < p_frame_count; i++) {
		conversion_buffer.write[i * 2] = int32_t(CLAMP(double(p_src_frames[i].left) * 2147483648.0, -2147483648.0, 2147483647.0));
		conversion_buffer.write[i * 2 + 1] = int32_t(CLAMP(double(p_src_frames[i].right) * 2147483648.0, -2147483648.0, 2147483647.0));
	}
	base->owner->capture_audio_data(conversion_buffer.ptr(), p_frame_count);
}

HybridAudioDriver::~HybridAudioDriver() {
	finish();
}

Error HybridAudioDriver::init(int p_mix_rate) {
	ERR_FAIL_COND_V(initialized.is_set(), ERR_ALREADY_IN_USE);
	ERR_FAIL_NULL_V(AudioServer::get_singleton(), ERR_UNCONFIGURED);

	mix_rate = p_mix_rate;

	Ref<SpxAudioCaptureEffect> effect;
	effect.instantiate(this);
	capture_effect_owner = effect.ptr();
	capture_effect = effect;
	AudioServer::get_singleton()->add_bus_effect(MASTER_BUS, capture_effect);
	initialized.set();

	if (OS::get_singleton()->is_stdout_verbose()) {
		print_line(vformat("SPX recorder audio capture initialized: %d Hz, %d channels", mix_rate, 2));
	}
	return OK;
}

void HybridAudioDriver::_remove_capture_effect() {
	if (capture_effect.is_null()) {
		return;
	}

	if (capture_effect_owner != nullptr) {
		capture_effect_owner->detach();
	}
	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server != nullptr && audio_server->get_bus_count() > MASTER_BUS) {
		for (int i = audio_server->get_bus_effect_count(MASTER_BUS) - 1; i >= 0; i--) {
			if (audio_server->get_bus_effect(MASTER_BUS, i) == capture_effect) {
				audio_server->remove_bus_effect(MASTER_BUS, i);
				break;
			}
		}
	}
	capture_effect.unref();
	capture_effect_owner = nullptr;
}

void HybridAudioDriver::finish() {
	if (!initialized.is_set()) {
		return;
	}

	recording_enabled.clear();
	_remove_capture_effect();
	{
		MutexLock lock(recorders_mutex);
		registered_recorders.clear();
	}
	initialized.clear();
}

void HybridAudioDriver::enable_recording(bool p_enable) {
	if (p_enable) {
		recording_enabled.set();
	} else {
		recording_enabled.clear();
	}
}

void HybridAudioDriver::capture_audio_data(const int32_t *p_buffer, int p_frames) {
	if (!recording_enabled.is_set() || !initialized.is_set() || p_buffer == nullptr || p_frames <= 0) {
		return;
	}

	MutexLock recorders_lock(recorders_mutex);
	for (IndependentAudioRecorder *recorder : registered_recorders) {
		if (recorder != nullptr && recorder->is_recording()) {
			recorder->on_audio_output(p_buffer, p_frames);
		}
	}
}

void HybridAudioDriver::register_audio_recorder(IndependentAudioRecorder *p_recorder) {
	ERR_FAIL_NULL(p_recorder);
	MutexLock lock(recorders_mutex);
	if (!registered_recorders.has(p_recorder)) {
		registered_recorders.push_back(p_recorder);
	}
}

void HybridAudioDriver::unregister_audio_recorder(IndependentAudioRecorder *p_recorder) {
	MutexLock lock(recorders_mutex);
	registered_recorders.erase(p_recorder);
}

int HybridAudioDriver::get_registered_recorder_count() const {
	MutexLock lock(recorders_mutex);
	return registered_recorders.size();
}
