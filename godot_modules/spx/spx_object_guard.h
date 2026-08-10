/**************************************************************************/
/*  spx_object_guard.h                                                    */
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

#ifndef SPX_OBJECT_GUARD_H
#define SPX_OBJECT_GUARD_H

#include "core/os/thread.h"
#include "core/string/ustring.h"
#include "gdextension_spx_ext.h"

/**
 * @class SpxObjectGuard
 * @brief Scoped SPX object validation and main-thread access helper
 *
 * This template class provides a type-safe, unified approach to object validation
 * across all SPX managers (Sprite, UI, Audio, etc.). It deliberately does not
 * make a raw Godot Object pointer safe across threads or lifecycle callbacks.
 *
 * Benefits:
 * - Type safety: Template-based, compile-time type checking
 * - Unified interface: Same pattern for all manager types
 * - Easy to extend: Can add logging, performance monitoring, etc.
 * - Better debugging: Clear stack traces and error context
 * - Zero overhead: Compiler inline optimization
 *
 * @tparam T The object type (e.g., SpxSprite, SpxUi)
 * @tparam Mgr The manager type (e.g., SpxSpriteMgr, SpxUiMgr)
 *
 * @example
 * @code
 * // In SpxSpriteMgr
 * void set_position(GdObj obj, GdVec2 pos) {
 *     SpxObjectGuard<SpxSprite, SpxSpriteMgr> sprite(obj, "set_position", this,
 *         [](SpxSpriteMgr* mgr, GdObj id) { return mgr->get_sprite(id); });
 *     if (sprite) {
 *         sprite->set_position(pos);
 *     }
 * }
 * @endcode
 */
template <typename T, typename Mgr>
class SpxObjectGuard {
private:
	T *object;
	bool valid;
	const char *context;

public:
	/**
	 * @brief Construct an object guard with a getter function
	 * @param obj The object ID to validate
	 * @param p_context Context string for error reporting (e.g., function name)
	 * @param mgr Pointer to the manager
	 * @param getter Function to get object from manager
	 */
	template <typename GetterFunc>
	explicit SpxObjectGuard(GdObj obj, const char *p_context, Mgr *mgr, GetterFunc getter) :
			object(nullptr), valid(false), context(p_context) {
		if (unlikely(!Thread::is_main_thread())) {
			ERR_PRINT(vformat("SPX object access in %s must run on the engine main thread.", context));
			return;
		}
		if (mgr) {
			object = getter(mgr, obj);
			valid = (object != nullptr);

			if (!valid) {
				WARN_PRINT(vformat("Try to access property of a null object in %s (gid=%d)", context, obj));
			}
		}
	}

	// Non-copyable (RAII pattern)
	SpxObjectGuard(const SpxObjectGuard &) = delete;
	SpxObjectGuard &operator=(const SpxObjectGuard &) = delete;

	// Movable (for return value optimization)
	SpxObjectGuard(SpxObjectGuard &&other) noexcept :
			object(other.object), valid(other.valid), context(other.context) {
		other.object = nullptr;
		other.valid = false;
	}

	SpxObjectGuard &operator=(SpxObjectGuard &&other) noexcept {
		if (this != &other) {
			object = other.object;
			valid = other.valid;
			context = other.context;
			other.object = nullptr;
			other.valid = false;
		}
		return *this;
	}

	/**
	 * @brief Check if object is valid
	 * @return true if object exists and is valid, false otherwise
	 */
	operator bool() const { return valid; }

	/**
	 * @brief Access the object via pointer operator
	 * @return Pointer to the object (nullptr if invalid)
	 */
	T *operator->() const { return object; }

	/**
	 * @brief Get the raw object pointer
	 * @return Pointer to the object (nullptr if invalid)
	 */
	T *get() const { return object; }

	/**
	 * @brief Check validity explicitly
	 * @return true if object is valid
	 */
	bool is_valid() const { return valid; }

	/**
	 * @brief Get the context string
	 * @return Context string for debugging
	 */
	const char *get_context() const { return context; }
};

/**
 * @brief Convenience macro for SPX sprite validation (void return)
 *
 * @example
 * @code
 * void SpxSpriteMgr::set_position(GdObj obj, GdVec2 pos) {
 *     SPX_SPRITE_GUARD_VOID(obj, __func__);
 *     sprite->set_position(pos);
 * }
 * @endcode
 */
#define SPX_SPRITE_GUARD_VOID(obj, context_name)                              \
	SpxObjectGuard<SpxSprite, SpxSpriteMgr> sprite(obj, context_name, this,   \
			[](SpxSpriteMgr *mgr, GdObj id) { return mgr->get_sprite(id); }); \
	if (!sprite)                                                              \
		return;

/**
 * @brief Convenience macro for SPX sprite validation with return value
 *
 * @example
 * @code
 * GdVec2 SpxSpriteMgr::get_position(GdObj obj) {
 *     SPX_SPRITE_GUARD_RETURN(obj, __func__, GdVec2());
 *     return sprite->get_position();
 * }
 * @endcode
 */
#define SPX_SPRITE_GUARD_RETURN(obj, context_name, return_val)                \
	SpxObjectGuard<SpxSprite, SpxSpriteMgr> sprite(obj, context_name, this,   \
			[](SpxSpriteMgr *mgr, GdObj id) { return mgr->get_sprite(id); }); \
	if (!sprite)                                                              \
		return return_val;

/**
 * @brief Convenience macro for SPX target sprite validation (void return)
 *
 * @example
 * @code
 * void SpxSpriteMgr::copy_properties(GdObj src, GdObj target) {
 *     SPX_SPRITE_GUARD_VOID(src, __func__);
 *     SPX_TARGET_SPRITE_GUARD_VOID(target, __func__);
 *     // Use both sprite and sprite_target
 * }
 * @endcode
 */
#define SPX_TARGET_SPRITE_GUARD_VOID(target_obj, context_name)                            \
	SpxObjectGuard<SpxSprite, SpxSpriteMgr> sprite_target(target_obj, context_name, this, \
			[](SpxSpriteMgr *mgr, GdObj id) { return mgr->get_sprite(id); });             \
	if (!sprite_target)                                                                   \
		return;

/**
 * @brief Convenience macro for SPX target sprite validation with return value
 *
 * @example
 * @code
 * GdBool SpxSpriteMgr::check_collision(GdObj obj, GdObj target) {
 *     SPX_SPRITE_GUARD_RETURN(obj, __func__, false);
 *     SPX_TARGET_SPRITE_GUARD_RETURN(target, __func__, false);
 *     return sprite->check_collision(sprite_target.get());
 * }
 * @endcode
 */
#define SPX_TARGET_SPRITE_GUARD_RETURN(target_obj, context_name, return_val)              \
	SpxObjectGuard<SpxSprite, SpxSpriteMgr> sprite_target(target_obj, context_name, this, \
			[](SpxSpriteMgr *mgr, GdObj id) { return mgr->get_sprite(id); });             \
	if (!sprite_target)                                                                   \
		return return_val;

/**
 * @brief Convenience macro for SPX UI node validation (void return)
 *
 * @example
 * @code
 * void SpxUiMgr::set_text(GdObj obj, GdString text) {
 *     SPX_UI_GUARD_VOID(obj, __func__);
 *     node->set_text(text);
 * }
 * @endcode
 */
#define SPX_UI_GUARD_VOID(obj, context_name)                            \
	SpxObjectGuard<SpxUi, SpxUiMgr> node(obj, context_name, this,       \
			[](SpxUiMgr *mgr, GdObj id) { return mgr->get_node(id); }); \
	if (!node)                                                          \
		return;

/**
 * @brief Convenience macro for SPX UI node validation with return value
 *
 * @example
 * @code
 * GdString SpxUiMgr::get_text(GdObj obj) {
 *     SPX_UI_GUARD_RETURN(obj, __func__, GdString());
 *     return node->get_text();
 * }
 * @endcode
 */
#define SPX_UI_GUARD_RETURN(obj, context_name, return_val)              \
	SpxObjectGuard<SpxUi, SpxUiMgr> node(obj, context_name, this,       \
			[](SpxUiMgr *mgr, GdObj id) { return mgr->get_node(id); }); \
	if (!node)                                                          \
		return return_val;

/**
 * @brief Convenience macro for SPX audio validation (void return)
 *
 * @example
 * @code
 * void SpxAudio::pause(GdInt aid) {
 *     SPX_AUDIO_GUARD_VOID(aid, __func__);
 *     audio->set_stream_paused(true);
 * }
 * @endcode
 */
#define SPX_AUDIO_GUARD_VOID(aid, context_name)                                  \
	SpxObjectGuard<AudioStreamPlayer2D, SpxAudio> audio(aid, context_name, this, \
			[](SpxAudio *mgr, GdInt id) { return mgr->_get_aid_audio(id); });    \
	if (!audio)                                                                  \
		return;

/**
 * @brief Convenience macro for SPX audio validation with return value
 *
 * @example
 * @code
 * GdBool SpxAudio::is_playing(GdInt aid) {
 *     SPX_AUDIO_GUARD_RETURN(aid, __func__, false);
 *     return audio->is_playing();
 * }
 * @endcode
 */
#define SPX_AUDIO_GUARD_RETURN(aid, context_name, return_val)                    \
	SpxObjectGuard<AudioStreamPlayer2D, SpxAudio> audio(aid, context_name, this, \
			[](SpxAudio *mgr, GdInt id) { return mgr->_get_aid_audio(id); });    \
	if (!audio)                                                                  \
		return return_val;

/**
 * @brief Convenience macro for SPX UI internal control validation (void return)
 *
 * Used within SpxUi class methods to validate the internal control pointer.
 *
 * @example
 * @code
 * void SpxUi::set_visible(GdBool visible) {
 *     SPX_UI_CONTROL_GUARD_VOID(__func__);
 *     node->set_visible(visible);
 * }
 * @endcode
 */
#define SPX_UI_CONTROL_GUARD_VOID(context_name)                       \
	SpxObjectGuard<Control, SpxUi> node(0, context_name, this,        \
			[](SpxUi *ui, GdInt) { return ui->get_control_item(); }); \
	if (!node)                                                        \
		return;

/**
 * @brief Convenience macro for SPX UI internal control validation with return value
 *
 * Used within SpxUi class methods to validate the internal control pointer.
 *
 * @example
 * @code
 * GdRect2 SpxUi::get_rect() {
 *     SPX_UI_CONTROL_GUARD_RETURN(__func__, GdRect2());
 *     return node->get_rect();
 * }
 * @endcode
 */
#define SPX_UI_CONTROL_GUARD_RETURN(context_name, return_val)         \
	SpxObjectGuard<Control, SpxUi> node(0, context_name, this,        \
			[](SpxUi *ui, GdInt) { return ui->get_control_item(); }); \
	if (!node)                                                        \
		return return_val;

#endif // SPX_OBJECT_GUARD_H
