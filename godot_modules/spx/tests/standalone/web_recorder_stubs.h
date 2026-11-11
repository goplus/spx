#pragma once

// Only the Godot types needed to compile the real Web recorder on a host C++
// compiler. MediaRecorder behavior is controlled through its C ABI in the test.
#include <cstdint>
#include <string>
#include <vector>

#define GDCLASS(m_class, m_base)
#define EMSCRIPTEN_KEEPALIVE
#define ERR_PRINT(m_message) ((void)(m_message))
#define GLOBAL_GET(m_name) (ProjectSettings::get_singleton()->auto_download)

enum Error { OK,
	ERR_UNAVAILABLE,
	ERR_CANT_CREATE };

class String : public std::string {
public:
	using std::string::string;
	String(const std::string &value) :
			std::string(value) {}
	static String utf8(const char *value) { return value; }
	const String &utf8() const { return *this; }
	const char *get_data() const { return c_str(); }
};

inline String itos(int value) {
	return std::to_string(value);
}
inline void print_line(const String &) {}

struct Size2i {
	int x = 0, y = 0;
	Size2i(int p_x = 0, int p_y = 0) :
			x(p_x), y(p_y) {}
};
class Image {};
template <class T>
class Ref {};
template <class T>
class List : public std::vector<T> {};

class AudioServer {
public:
	enum SpeakerMode { SPEAKER_MODE_STEREO };
};

class MovieWriter {
public:
	virtual ~MovieWriter() = default;
	virtual uint32_t get_audio_mix_rate() const = 0;
	virtual AudioServer::SpeakerMode get_audio_speaker_mode() const = 0;
	virtual void get_supported_extensions(List<String> *) const = 0;
	virtual Error write_begin(const Size2i &, uint32_t, const String &) = 0;
	virtual Error write_frame(const Ref<Image> &, const int32_t *) = 0;
	virtual void write_end() = 0;
	virtual bool handles_file(const String &) const = 0;
};

class ProjectSettings {
public:
	bool setting_present = true;
	bool auto_download = true;
	static ProjectSettings *get_singleton() {
		static ProjectSettings settings;
		return &settings;
	}
	bool has_setting(const char *) const { return setting_present; }
};

class OS {
public:
	bool verbose = false;
	static OS *get_singleton() {
		static OS os;
		return &os;
	}
	bool is_stdout_verbose() const { return verbose; }
};

class Time {
public:
	static Time *get_singleton() {
		static Time time;
		return &time;
	}
	String get_datetime_string_from_system(bool, bool) const { return "test-time"; }
};
