// SPX-owned Emscripten bridge library.
const GodotGdspx = {
	$GodotGdspx__deps: ['$GodotRuntime', '$GodotFS', '$GodotDisplayScreen'],
	$GodotGdspx__postset: [
		'Module["getPThread"] = GodotGdspx.getPThread;',
		'Module["deleteDirFS"] = GodotGdspx.removeDir;',
		'Module["deleteDirRecursive"] = GodotGdspx.removeDirRecursive;',
		'Module["copyToAdapter"] = GodotGdspx.copyToAdapter;',
		'Module["updateGameDatas"] = GodotGdspx.updateGameDatas;',
		'Module["readAllFS"] = GodotGdspx.readAll;',
		'Module["getFileSize"] = GodotGdspx.getFileSize;',
		'Module["request_reset"] = function () { GodotGdspx.requestReset(); };',
	].join(''),
	$GodotGdspx: {
		contactCallbackEventNames: [
			"OnCollisionEnter",
			"OnCollisionStay",
			"OnCollisionExit",
			"OnTriggerEnter",
			"OnTriggerStay",
			"OnTriggerExit",
		],
		contactEventWarnThreshold: 4096 * 5,
		gameDatas: null,
		gameDataCallback: null,
		requestReset: function () {},

		getPThread: function () {
			return typeof PThread !== 'undefined' ? PThread : null;
		},

		updateGameDatas: function (path, files) {
			GodotGdspx.gameDatas = { path: path, files: files };
			if (GodotGdspx.gameDataCallback) {
				GodotGdspx.gameDataCallback(path, files);
			}
		},

		copyToAdapter: function (path, adapter) {
			const promises = [];
			const entries = FS.readdir(path).filter(function (value) {
				return value !== '.' && value !== '..';
			});
			entries.forEach(function (entry) {
				const childPath = `${path}/${entry}`;
				const stat = FS.stat(childPath);
				if (FS.isFile(stat.mode)) {
					promises.push(adapter['writeFile'](childPath, FS.readFile(childPath)));
				} else if (FS.isDir(stat.mode)) {
					promises.push(...GodotGdspx.copyToAdapter(childPath, adapter));
				}
			});
			return promises;
		},

		removeDir: function (path) {
			const analysis = FS.analyzePath(path);
			if (analysis.exists && analysis.object && FS.isDir(analysis.object.mode)) {
				FS.rmdir(path);
			}
		},

		removeDirRecursive: function (path) {
			try {
				const stat = FS.stat(path);
				if (!FS.isDir(stat.mode)) {
					FS.unlink(path);
					return;
				}

				const entries = FS.readdir(path).filter((name) => name !== '.' && name !== '..');
				for (const entry of entries) {
					GodotGdspx.removeDirRecursive(`${path}/${entry}`);
				}
				FS.rmdir(path);
			} catch (error) {
				if (error.errno !== GodotFS.ENOENT) {
					GodotRuntime.error(`Failed to remove ${path}`, error);
				}
			}
		},

		readAll: function (path) {
			try {
				const stat = FS.stat(path);
				if (!FS.isFile(stat.mode)) {
					throw new Error(`Path is not a file: ${path}`);
				}
				return FS.readFile(path);
			} catch (error) {
				GodotRuntime.error(`Failed to read file: ${path}`, error);
				return null;
			}
		},

		getFileSize: function (path) {
			return FS.stat(path).size;
		},

		// Go's syscall/js bridge represents int64 values as two uint32 parts.
		splitInt64: function (value) {
			return {
				'low': Number(value & 0xffffffffn),
				'high': Number((value >> 32n) & 0xffffffffn),
			};
		},

		dispatch: function (eventName, ...args) {
			const ffi = globalThis['FFI'];
			if (ffi) {
				ffi['gdspx_dispatch'](eventName, ...args);
			}
		},

		notifyRuntime: function (name, ...args) {
			const ffi = globalThis['FFI'];
			if (ffi && typeof ffi[name] === 'function') {
				ffi[name](...args);
			}
		},

		contactEvents: [],
		contactSessionActive: true,
		contactSessionGeneration: 0,

		setContactSessionActive: function (active) {
			GodotGdspx.contactEvents = [];
			GodotGdspx.contactSessionGeneration += 1;
			GodotGdspx.contactSessionActive = active;
		},

		endContactSession: function (eventName) {
			GodotGdspx.setContactSessionActive(false);
			if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
				globalThis['GdspxFlushDeferredFrees']();
			}
			GodotGdspx.dispatch(eventName);
		},

		queueContact: function (type, selfID, otherID) {
			if (!GodotGdspx.contactSessionActive) {
				return;
			}
			const self = GodotGdspx.splitInt64(selfID);
			const other = GodotGdspx.splitInt64(otherID);
			const events = GodotGdspx.contactEvents;
			const belowWarningThreshold = events.length < GodotGdspx.contactEventWarnThreshold;
			events.push(type, self['low'], self['high'], other['low'], other['high']);
			if (belowWarningThreshold && events.length >= GodotGdspx.contactEventWarnThreshold) {
				GodotRuntime.error("gdspx contact event queue is growing large before flush.");
			}
		},

		flushContactEvents: function () {
			const generation = GodotGdspx.contactSessionGeneration;
			const events = GodotGdspx.contactEvents;
			if (events.length === 0) {
				return;
			}
			GodotGdspx.contactEvents = [];

			const batch = globalThis['gdspx_on_contact_events'];
			if (typeof batch === 'function') {
				batch(new Uint8Array(Uint32Array.from(events).buffer));
				return;
			}

			for (let i = 0; i + 4 < events.length && generation === GodotGdspx.contactSessionGeneration; i += 5) {
				const type = events[i] | 0;
				const self = { 'low': events[i + 1] >>> 0, 'high': events[i + 2] >>> 0 };
				const other = { 'low': events[i + 3] >>> 0, 'high': events[i + 4] >>> 0 };
				const eventName = GodotGdspx.contactCallbackEventNames[type - 1];
				if (eventName) {
					GodotGdspx.dispatch(eventName, self, other);
				}
			}
		},
	},

	godot_js_spx_request_reset_cb__proxy: 'sync',
	godot_js_spx_request_reset_cb__sig: 'vi',
	godot_js_spx_request_reset_cb: function (callback) {
		GodotGdspx.requestReset = GodotRuntime.get_func(callback);
	},

	godot_js_spx_game_data_cb__proxy: 'sync',
	godot_js_spx_game_data_cb__sig: 'vi',
	godot_js_spx_game_data_cb: function (callback) {
		const func = GodotRuntime.get_func(callback);
		GodotGdspx.gameDataCallback = function (path, files) {
			const args = files || [];
			if (!args.length) {
				return;
			}
			const pathPtr = GodotRuntime.allocString(path);
			const argv = GodotRuntime.allocStringArray(args);
			func(pathPtr, argv, args.length);
			GodotRuntime.freeStringArray(argv, args.length);
			GodotRuntime.free(pathPtr);
		};
		if (GodotGdspx.gameDatas) {
			GodotGdspx.gameDataCallback(GodotGdspx.gameDatas.path, GodotGdspx.gameDatas.files);
		}
	},

	godot_js_spx_window_size_get__proxy: 'sync',
	godot_js_spx_window_size_get__sig: 'vii',
	godot_js_spx_window_size_get: function (widthPtr, heightPtr) {
		const scale = GodotDisplayScreen.getPixelRatio();
		GodotRuntime.setHeapValue(widthPtr, Math.floor(window.innerWidth * scale), 'i32');
		GodotRuntime.setHeapValue(heightPtr, Math.floor(window.innerHeight * scale), 'i32');
	},

	// Internal session boundary, called again when a reset runtime restarts.
	godot_js_spx_contact_session_start__sig: 'v',
	godot_js_spx_contact_session_start: function () {
		GodotGdspx.setContactSessionActive(true);
	},

	// godot gdspx extensions
	godot_js_spx_on_engine_start__sig: 'v',
	godot_js_spx_on_engine_start: async function () {
		GodotGdspx.setContactSessionActive(true);
		globalThis['FFI'] = null;
		if (typeof self['initExtensionWasm'] === 'function') {
			await self['initExtensionWasm']();
			return;
		}
		GodotRuntime.error('Missing self.initExtensionWasm for gdspx web callbacks.');
	},

	godot_js_spx_on_engine_update__sig: 'vf',
	godot_js_spx_on_engine_update: function (delta) {
		// Reclaim transient arrays once per Update, across all FixedUpdate calls.
		if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
			globalThis['GdspxFlushDeferredFrees']();
		}
		GodotGdspx.flushContactEvents();
		GodotGdspx.dispatch("OnEngineUpdate", delta);
	},

	godot_js_spx_on_engine_fixed_update__sig: 'vf',
	godot_js_spx_on_engine_fixed_update: function (delta) {
		GodotGdspx.flushContactEvents();
		GodotGdspx.dispatch("OnEngineFixedUpdate", delta);
	},

	godot_js_spx_on_engine_destroy__sig: 'v',
	godot_js_spx_on_engine_destroy: function () {
		GodotGdspx.endContactSession("OnEngineDestroy");
	},

	godot_js_spx_on_engine_destroyed__sig: 'v',
	godot_js_spx_on_engine_destroyed: function () {
		GodotGdspx.dispatch("OnEngineDestroyed");
	},

	godot_js_spx_on_engine_reset__sig: 'v',
	godot_js_spx_on_engine_reset: function () {
		GodotGdspx.endContactSession("OnEngineReset");
	},

	godot_js_spx_on_reset_done__sig: 'vj',
	godot_js_spx_on_reset_done: function (code) {
		GodotGdspx.notifyRuntime("gdspx_on_runtime_reset", Number(code));
	},

	godot_js_spx_on_engine_pause__sig: 'vi',
	godot_js_spx_on_engine_pause: function (is_on) {
		GodotGdspx.dispatch("OnEnginePause", is_on);
	},

	godot_js_spx_on_scene_sprite_instantiated__sig: 'vji',
	godot_js_spx_on_scene_sprite_instantiated: function (obj, type_name) {
		GodotGdspx.dispatch(
			"OnSceneSpriteInstantiated",
			GodotGdspx.splitInt64(obj),
			GodotRuntime.parseString(type_name)
		);
	},

	godot_js_spx_on_runtime_panic__sig: 'vi',
	godot_js_spx_on_runtime_panic: function (msg) {
		GodotGdspx.notifyRuntime("gdspx_on_runtime_panic", GodotRuntime.parseString(msg));
	},

	godot_js_spx_on_runtime_exit__sig: 'vj',
	godot_js_spx_on_runtime_exit: function (code) {
		GodotGdspx.notifyRuntime("gdspx_on_runtime_exit", Number(code));
	},

	godot_js_spx_on_sprite_ready__sig: 'vj',
	godot_js_spx_on_sprite_ready: function (obj) {
		GodotGdspx.dispatch("OnSpriteReady", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_updated__sig: 'vf',
	godot_js_spx_on_sprite_updated: function (delta) {
		GodotGdspx.dispatch("OnSpriteUpdated", delta);
	},

	godot_js_spx_on_sprite_fixed_updated__sig: 'vf',
	godot_js_spx_on_sprite_fixed_updated: function (delta) {
		GodotGdspx.dispatch("OnSpriteFixedUpdated", delta);
	},

	godot_js_spx_on_sprite_destroyed__sig: 'vj',
	godot_js_spx_on_sprite_destroyed: function (obj) {
		GodotGdspx.dispatch("OnSpriteDestroyed", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_frames_set_changed__sig: 'vj',
	godot_js_spx_on_sprite_frames_set_changed: function (obj) {
		GodotGdspx.dispatch("OnSpriteFramesSetChanged", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_animation_changed__sig: 'vj',
	godot_js_spx_on_sprite_animation_changed: function (obj) {
		GodotGdspx.dispatch("OnSpriteAnimationChanged", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_frame_changed__sig: 'vj',
	godot_js_spx_on_sprite_frame_changed: function (obj) {
		GodotGdspx.dispatch("OnSpriteFrameChanged", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_animation_looped__sig: 'vj',
	godot_js_spx_on_sprite_animation_looped: function (obj) {
		GodotGdspx.dispatch("OnSpriteAnimationLooped", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_animation_finished__sig: 'vj',
	godot_js_spx_on_sprite_animation_finished: function (obj) {
		GodotGdspx.dispatch("OnSpriteAnimationFinished", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_vfx_finished__sig: 'vj',
	godot_js_spx_on_sprite_vfx_finished: function (obj) {
		GodotGdspx.dispatch("OnSpriteVfxFinished", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_screen_exited__sig: 'vj',
	godot_js_spx_on_sprite_screen_exited: function (obj) {
		GodotGdspx.dispatch("OnSpriteScreenExited", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_sprite_screen_entered__sig: 'vj',
	godot_js_spx_on_sprite_screen_entered: function (obj) {
		GodotGdspx.dispatch("OnSpriteScreenEntered", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_mouse_pressed__sig: 'vj',
	godot_js_spx_on_mouse_pressed: function (keyid) {
		GodotGdspx.dispatch("OnMousePressed", GodotGdspx.splitInt64(keyid));
	},

	godot_js_spx_on_mouse_released__sig: 'vj',
	godot_js_spx_on_mouse_released: function (keyid) {
		GodotGdspx.dispatch("OnMouseReleased", GodotGdspx.splitInt64(keyid));
	},

	godot_js_spx_on_key_pressed__sig: 'vj',
	godot_js_spx_on_key_pressed: function (keyid) {
		GodotGdspx.dispatch("OnKeyPressed", GodotGdspx.splitInt64(keyid));
	},

	godot_js_spx_on_key_released__sig: 'vj',
	godot_js_spx_on_key_released: function (keyid) {
		GodotGdspx.dispatch("OnKeyReleased", GodotGdspx.splitInt64(keyid));
	},

	godot_js_spx_on_action_pressed__sig: 'vi',
	godot_js_spx_on_action_pressed: function (action_name) {
		GodotGdspx.dispatch("OnActionPressed", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_action_just_pressed__sig: 'vi',
	godot_js_spx_on_action_just_pressed: function (action_name) {
		GodotGdspx.dispatch("OnActionJustPressed", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_action_just_released__sig: 'vi',
	godot_js_spx_on_action_just_released: function (action_name) {
		GodotGdspx.dispatch("OnActionJustReleased", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_axis_changed__sig: 'vif',
	godot_js_spx_on_axis_changed: function (action_name, value) {
		GodotGdspx.dispatch("OnAxisChanged", GodotRuntime.parseString(action_name), value);
	},

	godot_js_spx_on_collision_enter__sig: 'vjj',
	godot_js_spx_on_collision_enter: function (self_id, other_id) {
		GodotGdspx.queueContact(1, self_id, other_id);
	},

	godot_js_spx_on_collision_stay__sig: 'vjj',
	godot_js_spx_on_collision_stay: function (self_id, other_id) {
		GodotGdspx.queueContact(2, self_id, other_id);
	},

	godot_js_spx_on_collision_exit__sig: 'vjj',
	godot_js_spx_on_collision_exit: function (self_id, other_id) {
		GodotGdspx.queueContact(3, self_id, other_id);
	},

	godot_js_spx_on_trigger_enter__sig: 'vjj',
	godot_js_spx_on_trigger_enter: function (self_id, other_id) {
		GodotGdspx.queueContact(4, self_id, other_id);
	},

	godot_js_spx_on_trigger_stay__sig: 'vjj',
	godot_js_spx_on_trigger_stay: function (self_id, other_id) {
		GodotGdspx.queueContact(5, self_id, other_id);
	},

	godot_js_spx_on_trigger_exit__sig: 'vjj',
	godot_js_spx_on_trigger_exit: function (self_id, other_id) {
		GodotGdspx.queueContact(6, self_id, other_id);
	},

	godot_js_spx_on_ui_ready__sig: 'vj',
	godot_js_spx_on_ui_ready: function (obj) {
		GodotGdspx.dispatch("OnUiReady", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_updated__sig: 'vj',
	godot_js_spx_on_ui_updated: function (obj) {
		GodotGdspx.dispatch("OnUiUpdated", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_destroyed__sig: 'vj',
	godot_js_spx_on_ui_destroyed: function (obj) {
		GodotGdspx.dispatch("OnUiDestroyed", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_pressed__sig: 'vj',
	godot_js_spx_on_ui_pressed: function (obj) {
		GodotGdspx.dispatch("OnUiPressed", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_released__sig: 'vj',
	godot_js_spx_on_ui_released: function (obj) {
		GodotGdspx.dispatch("OnUiReleased", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_hovered__sig: 'vj',
	godot_js_spx_on_ui_hovered: function (obj) {
		GodotGdspx.dispatch("OnUiHovered", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_clicked__sig: 'vj',
	godot_js_spx_on_ui_clicked: function (obj) {
		GodotGdspx.dispatch("OnUiClicked", GodotGdspx.splitInt64(obj));
	},

	godot_js_spx_on_ui_toggle__sig: 'vji',
	godot_js_spx_on_ui_toggle: function (obj, is_on) {
		GodotGdspx.dispatch("OnUiToggle", GodotGdspx.splitInt64(obj), is_on);
	},

	godot_js_spx_on_ui_text_changed__sig: 'vji',
	godot_js_spx_on_ui_text_changed: function (obj, text) {
		GodotGdspx.dispatch("OnUiTextChanged", GodotGdspx.splitInt64(obj), GodotRuntime.parseString(text));
	},
};

autoAddDeps(GodotGdspx, '$GodotGdspx');
mergeInto(LibraryManager.library, GodotGdspx);
