/**************************************************************************/
/*  audio_driver_hybrid.cpp                                               */
/**************************************************************************/

#include "audio_driver_hybrid.h"

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

	conversion_buffer.resize(p_frame_count * 2);
	for (int i = 0; i < p_frame_count; i++) {
		conversion_buffer.write[i * 2] = int32_t(CLAMP(p_src_frames[i].left, -1.0f, 1.0f) * 2147483647.0f);
		conversion_buffer.write[i * 2 + 1] = int32_t(CLAMP(p_src_frames[i].right, -1.0f, 1.0f) * 2147483647.0f);
	}
	base->owner->capture_audio_data(conversion_buffer.ptr(), p_frame_count, 1);
}

HybridAudioDriver::~HybridAudioDriver() {
	finish();
}

Error HybridAudioDriver::init(int p_mix_rate, AudioDriver::SpeakerMode p_speaker_mode) {
	ERR_FAIL_COND_V(initialized.is_set(), ERR_ALREADY_IN_USE);
	ERR_FAIL_NULL_V(AudioServer::get_singleton(), ERR_UNCONFIGURED);

	mix_rate = p_mix_rate;
	speaker_mode = p_speaker_mode;
	channels = 2;

	const int requested_frames = MAX(1, int(mix_rate * buffer_length_seconds));
	buffer_power = 0;
	while ((1 << buffer_power) < requested_frames) {
		buffer_power++;
	}
	capture_buffer = RingBuffer<AudioFrame>(buffer_power);

	Ref<SpxAudioCaptureEffect> effect;
	effect.instantiate(this);
	capture_effect_owner = effect.ptr();
	capture_effect = effect;
	capture_bus = 0; // Master is always bus zero.
	AudioServer::get_singleton()->add_bus_effect(capture_bus, capture_effect);
	initialized.set();

	if (OS::get_singleton()->is_stdout_verbose()) {
		print_line(vformat("SPX recorder audio capture initialized: %d Hz, %d channels", mix_rate, channels));
	}
	return OK;
}

void HybridAudioDriver::start() {
	// The AudioEffect becomes active as soon as it is inserted into Master.
}

void HybridAudioDriver::_remove_capture_effect() {
	if (capture_effect.is_null()) {
		return;
	}

	if (capture_effect_owner != nullptr) {
		capture_effect_owner->detach();
	}
	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server != nullptr && capture_bus >= 0 && capture_bus < audio_server->get_bus_count()) {
		for (int i = audio_server->get_bus_effect_count(capture_bus) - 1; i >= 0; i--) {
			if (audio_server->get_bus_effect(capture_bus, i) == capture_effect) {
				audio_server->remove_bus_effect(capture_bus, i);
				break;
			}
		}
	}
	capture_effect.unref();
	capture_effect_owner = nullptr;
	capture_bus = -1;
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
	_clear_capture_buffer();
	initialized.clear();
}

void HybridAudioDriver::_clear_capture_buffer() {
	MutexLock lock(data_mutex);
	capture_buffer = RingBuffer<AudioFrame>(buffer_power);
}

void HybridAudioDriver::enable_recording(bool p_enable) {
	if (recording_enabled.is_set() == p_enable) {
		return;
	}
	if (p_enable) {
		_clear_capture_buffer();
		recording_enabled.set();
	} else {
		recording_enabled.clear();
	}
}

void HybridAudioDriver::capture_audio_data(const int32_t *p_buffer, int p_frames, int p_channel_pairs) {
	if (!recording_enabled.is_set() || !initialized.is_set() || p_buffer == nullptr || p_frames <= 0) {
		return;
	}

	const int actual_channels = MAX(1, p_channel_pairs * 2);
	{
		MutexLock lock(data_mutex);
		for (int i = 0; i < p_frames; i++) {
			AudioFrame frame;
			frame.left = float(p_buffer[i * actual_channels]) / 2147483648.0f;
			frame.right = actual_channels > 1 ? float(p_buffer[i * actual_channels + 1]) / 2147483648.0f : frame.left;
			if (capture_buffer.space_left() == 0) {
				AudioFrame discarded;
				capture_buffer.read(&discarded, 1);
			}
			capture_buffer.write(&frame, 1);
		}
	}

	MutexLock recorders_lock(recorders_mutex);
	for (IndependentAudioRecorder *recorder : registered_recorders) {
		if (recorder != nullptr && recorder->is_recording()) {
			recorder->on_audio_output(p_buffer, p_frames);
		}
	}
}

int HybridAudioDriver::get_captured_audio_data(int32_t *p_output_buffer, int p_requested_frames) {
	ERR_FAIL_NULL_V(p_output_buffer, 0);
	ERR_FAIL_COND_V(p_requested_frames < 0, 0);

	MutexLock lock(data_mutex);
	const int frames_to_read = MIN(p_requested_frames, capture_buffer.data_left());
	Vector<AudioFrame> frames;
	frames.resize(frames_to_read);
	if (frames_to_read > 0) {
		capture_buffer.read(frames.ptrw(), frames_to_read);
	}
	for (int i = 0; i < frames_to_read; i++) {
		p_output_buffer[i * 2] = int32_t(CLAMP(frames[i].left, -1.0f, 1.0f) * 2147483647.0f);
		p_output_buffer[i * 2 + 1] = int32_t(CLAMP(frames[i].right, -1.0f, 1.0f) * 2147483647.0f);
	}
	for (int i = frames_to_read * 2; i < p_requested_frames * 2; i++) {
		p_output_buffer[i] = 0;
	}
	return p_requested_frames;
}

int HybridAudioDriver::get_available_frames() const {
	MutexLock lock(data_mutex);
	return capture_buffer.data_left();
}

bool HybridAudioDriver::has_audio_data() const {
	return get_available_frames() > 0;
}

void HybridAudioDriver::register_audio_recorder(IndependentAudioRecorder *p_recorder) {
	ERR_FAIL_NULL(p_recorder);
	MutexLock lock(recorders_mutex);
	for (IndependentAudioRecorder *recorder : registered_recorders) {
		if (recorder == p_recorder) {
			return;
		}
	}
	registered_recorders.push_back(p_recorder);
}

void HybridAudioDriver::unregister_audio_recorder(IndependentAudioRecorder *p_recorder) {
	if (p_recorder == nullptr) {
		return;
	}
	MutexLock lock(recorders_mutex);
	for (int i = 0; i < registered_recorders.size(); i++) {
		if (registered_recorders[i] == p_recorder) {
			registered_recorders.remove_at(i);
			return;
		}
	}
}

int HybridAudioDriver::get_registered_recorder_count() const {
	MutexLock lock(recorders_mutex);
	return registered_recorders.size();
}
