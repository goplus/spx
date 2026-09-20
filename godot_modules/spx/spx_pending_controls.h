#ifndef SPX_PENDING_CONTROLS_H
#define SPX_PENDING_CONTROLS_H

#include "core/os/mutex.h"

// Process-lived mailbox. Producers never need to access an engine instance.
// Requests coalesce by kind; the main loop owns their priority and execution.
class SpxPendingControls {
public:
	enum Kind { RESTART, RESET, PAUSE, RESUME, NEXT_FRAME, COUNT };

private:
	mutable Mutex mutex;
	bool accepting = false;
	bool pending[COUNT] = {};
	int reset_code = 0;
	bool paused = false;

public:
	void set_accepting(bool p_accepting) {
		MutexLock lock(mutex);
		accepting = p_accepting;
		for (bool &request : pending) {
			request = false;
		}
		reset_code = 0;
		paused = false;
	}

	void submit(Kind p_kind, int p_reset_code = 0) {
		MutexLock lock(mutex);
		if (!accepting) {
			return;
		}
		pending[p_kind] = true;
		if (p_kind == RESET) {
			reset_code = p_reset_code;
		}
	}

	bool take(Kind p_kind, int *r_reset_code = nullptr) {
		MutexLock lock(mutex);
		if (!pending[p_kind]) {
			return false;
		}
		pending[p_kind] = false;
		if (p_kind == RESET && r_reset_code) {
			*r_reset_code = reset_code;
		}
		return true;
	}

	void set_paused(bool p_paused) {
		MutexLock lock(mutex);
		paused = accepting && p_paused;
	}

	bool is_paused() const {
		MutexLock lock(mutex);
		return paused;
	}
};

#endif // SPX_PENDING_CONTROLS_H
