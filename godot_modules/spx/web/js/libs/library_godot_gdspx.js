// SPX-owned Emscripten bridge library.
const GodotGdspx = {
	$GodotGdspx__deps: ['$GodotConfig', '$GodotRuntime', '$GodotFS', '$GodotDisplayScreen'],
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
		contactCallbackExportNames: [
			"gdspx_on_collision_enter",
			"gdspx_on_collision_stay",
			"gdspx_on_collision_exit",
			"gdspx_on_trigger_enter",
			"gdspx_on_trigger_stay",
			"gdspx_on_trigger_exit",
		],
		contactCallbackEventNames: [
			"OnCollisionEnter",
			"OnCollisionStay",
			"OnCollisionExit",
			"OnTriggerEnter",
			"OnTriggerStay",
			"OnTriggerExit",
		],
		contactEventFields: 5,
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

		getDirectHandler: function (exportName) {
			// Read the shared slots at call time. Emscripten serializes library object
			// properties while linking, which would detach a stored object from the
			// global table populated later by the SPX Web runtime.
			const slots = globalThis['__spxDirectCallbackHandlerSlots'];
			const directHandler = slots && slots[exportName];
			return typeof directHandler === 'function' ? directHandler : null;
		},

		// Direct callbacks use native BigInt object ids to avoid boxing on the hot
		// path. The FFI fallback keeps the legacy { low, high } shape for
		// compatibility with older callback bridges.
		toCallbackInt: function (ptr, directHandler) {
			if (typeof directHandler === 'function') {
				return GodotRuntime.ToJsBigInt(ptr);
			}
			return GodotRuntime.ToJsInt(ptr);
		},

		toCallbackObj: function (ptr, directHandler) {
			if (typeof directHandler === 'function') {
				return GodotRuntime.ToJsBigObj(ptr);
			}
			return GodotRuntime.ToJsObj(ptr);
		},

		callN: function (exportName, eventName, directHandler, ...args) {
			if (typeof directHandler === 'function') {
				directHandler(...args);
				return;
			}
			const ffi = globalThis['FFI'];
			if (!ffi) {
				return;
			}
			const direct = ffi[exportName];
			if (typeof direct === 'function') {
				direct.call(ffi, ...args);
				return;
			}
			if (eventName) {
				ffi['gdspx_dispatch'](eventName, ...args);
			}
		},

		callDirect: function (exportName, ...args) {
			GodotGdspx.callN(exportName, null, null, ...args);
		},

		call0: function (exportName, eventName, directHandler = null) {
			GodotGdspx.callN(exportName, eventName, directHandler);
		},

		call1: function (exportName, eventName, arg0, directHandler = null) {
			GodotGdspx.callN(exportName, eventName, directHandler, arg0);
		},

		call2: function (exportName, eventName, arg0, arg1, directHandler = null) {
			GodotGdspx.callN(exportName, eventName, directHandler, arg0, arg1);
		},

		toContactLowHigh: function (ptr) {
			const value = GodotRuntime.ToJsInt(ptr);
			if (value && typeof value === 'object' && typeof value['low'] === 'number' && typeof value['high'] === 'number') {
				return value;
			}
			if (typeof value === 'bigint') {
				return {
					'low': Number(value & 0xffffffffn) >>> 0,
					'high': Number((value >> 32n) & 0xffffffffn) >>> 0,
				};
			}
			const numberValue = Number(value) || 0;
			return {
				'low': numberValue >>> 0,
				'high': (numberValue / 0x100000000) >>> 0,
			};
		},

		contactEvents: [],
		contactEventsWarned: false,

		queueContact: function (type, selfPtr, otherPtr) {
			const self = GodotGdspx.toContactLowHigh(selfPtr);
			const other = GodotGdspx.toContactLowHigh(otherPtr);
			const events = GodotGdspx.contactEvents;
			events.push(type, self['low'], self['high'], other['low'], other['high']);
			if (!GodotGdspx.contactEventsWarned && events.length >= GodotGdspx.contactEventWarnThreshold) {
				GodotGdspx.contactEventsWarned = true;
				GodotRuntime.error("gdspx contact event queue is growing large before flush.");
			}
		},

		flushContactEvents: function () {
			const events = GodotGdspx.contactEvents;
			if (events.length === 0) {
				return;
			}
			GodotGdspx.contactEvents = [];
			GodotGdspx.contactEventsWarned = false;

			const direct = typeof globalThis['gdspx_on_contact_events'] === 'function' ? globalThis['gdspx_on_contact_events'] : null;
			if (direct) {
				direct(new Uint8Array(Uint32Array.from(events).buffer));
				return;
			}

			for (let i = 0; i + 4 < events.length; i += 5) {
				const type = events[i] | 0;
				const self = { 'low': events[i + 1] >>> 0, 'high': events[i + 2] >>> 0 };
				const other = { 'low': events[i + 3] >>> 0, 'high': events[i + 4] >>> 0 };
				const exportName = GodotGdspx.contactCallbackExportNames[type - 1];
				const eventName = GodotGdspx.contactCallbackEventNames[type - 1];
				if (exportName && eventName) {
					const directHandler = GodotGdspx.getDirectHandler(exportName);
					GodotGdspx.call2(exportName, eventName, self, other, directHandler);
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

	// godot gdspx extensions
	godot_js_spx_on_engine_start__sig: 'v',
	godot_js_spx_on_engine_start: async function () {
		globalThis['FFI'] = null;
		if (typeof self['initExtensionWasm'] === 'function') {
			await self['initExtensionWasm']();
			return;
		}
		GodotRuntime.error('Missing self.initExtensionWasm for gdspx web callbacks.');
	},

	godot_js_spx_on_engine_update__sig: 'vf',
	godot_js_spx_on_engine_update: function (delta) {
		if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
			globalThis['GdspxFlushDeferredFrees']();
		}
		GodotGdspx.flushContactEvents();
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_engine_update");
		GodotGdspx.call1("gdspx_on_engine_update", "OnEngineUpdate", delta, directHandler);
	},

	godot_js_spx_on_engine_fixed_update__sig: 'vf',
	godot_js_spx_on_engine_fixed_update: function (delta) {
		if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
			globalThis['GdspxFlushDeferredFrees']();
		}
		GodotGdspx.flushContactEvents();
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_engine_fixed_update");
		GodotGdspx.call1("gdspx_on_engine_fixed_update", "OnEngineFixedUpdate", delta, directHandler);
	},

	godot_js_spx_on_engine_destroy__sig: 'v',
	godot_js_spx_on_engine_destroy: function () {
		if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
			globalThis['GdspxFlushDeferredFrees']();
		}
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_engine_destroy");
		GodotGdspx.call0("gdspx_on_engine_destroy", "OnEngineDestroy", directHandler);
	},

	godot_js_spx_on_engine_reset__sig: 'v',
	godot_js_spx_on_engine_reset: function () {
		if (typeof globalThis['GdspxFlushDeferredFrees'] === 'function') {
			globalThis['GdspxFlushDeferredFrees']();
		}
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_engine_reset");
		GodotGdspx.call0("gdspx_on_engine_reset", "OnEngineReset", directHandler);
	},

	godot_js_spx_on_reset_done__sig: 'vi',
	godot_js_spx_on_reset_done: function (code) {
		// Reset completion uses a dedicated export and has no dispatch-based fallback.
		GodotGdspx.callDirect("gdspx_on_runtime_reset", code);
	},

	godot_js_spx_on_engine_pause__sig: 'vi',
	godot_js_spx_on_engine_pause: function (is_on) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_engine_pause");
		GodotGdspx.call1("gdspx_on_engine_pause", "OnEnginePause", is_on, directHandler);
	},

	godot_js_spx_on_scene_sprite_instantiated__sig: 'vii',
	godot_js_spx_on_scene_sprite_instantiated: function (obj, type_name) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_scene_sprite_instantiated");
		GodotGdspx.call2(
			"gdspx_on_scene_sprite_instantiated",
			"OnSceneSpriteInstantiated",
			GodotGdspx.toCallbackObj(obj, directHandler),
			GodotRuntime.parseString(type_name),
			directHandler
		);
	},

	godot_js_spx_on_runtime_panic__sig: 'vi',
	godot_js_spx_on_runtime_panic: function (msg) {
		GodotGdspx.callDirect("gdspx_on_runtime_panic", GodotRuntime.parseString(msg));
	},

	godot_js_spx_on_runtime_exit__sig: 'vi',
	godot_js_spx_on_runtime_exit: function (code) {
		GodotGdspx.callDirect("gdspx_on_runtime_exit", code);
	},

	godot_js_spx_on_sprite_ready__sig: 'vi',
	godot_js_spx_on_sprite_ready: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_ready");
		GodotGdspx.call1("gdspx_on_sprite_ready", "OnSpriteReady", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_updated__sig: 'vf',
	godot_js_spx_on_sprite_updated: function (delta) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_updated");
		GodotGdspx.call1("gdspx_on_sprite_updated", "OnSpriteUpdated", delta, directHandler);
	},

	godot_js_spx_on_sprite_fixed_updated__sig: 'vf',
	godot_js_spx_on_sprite_fixed_updated: function (delta) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_fixed_updated");
		GodotGdspx.call1("gdspx_on_sprite_fixed_updated", "OnSpriteFixedUpdated", delta, directHandler);
	},

	godot_js_spx_on_sprite_destroyed__sig: 'vi',
	godot_js_spx_on_sprite_destroyed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_destroyed");
		GodotGdspx.call1("gdspx_on_sprite_destroyed", "OnSpriteDestroyed", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_frames_set_changed__sig: 'vi',
	godot_js_spx_on_sprite_frames_set_changed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_frames_set_changed");
		GodotGdspx.call1("gdspx_on_sprite_frames_set_changed", "OnSpriteFramesSetChanged", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_animation_changed__sig: 'vi',
	godot_js_spx_on_sprite_animation_changed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_animation_changed");
		GodotGdspx.call1("gdspx_on_sprite_animation_changed", "OnSpriteAnimationChanged", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_frame_changed__sig: 'vi',
	godot_js_spx_on_sprite_frame_changed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_frame_changed");
		GodotGdspx.call1("gdspx_on_sprite_frame_changed", "OnSpriteFrameChanged", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_animation_looped__sig: 'vi',
	godot_js_spx_on_sprite_animation_looped: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_animation_looped");
		GodotGdspx.call1("gdspx_on_sprite_animation_looped", "OnSpriteAnimationLooped", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_animation_finished__sig: 'vi',
	godot_js_spx_on_sprite_animation_finished: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_animation_finished");
		GodotGdspx.call1("gdspx_on_sprite_animation_finished", "OnSpriteAnimationFinished", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_vfx_finished__sig: 'vi',
	godot_js_spx_on_sprite_vfx_finished: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_vfx_finished");
		GodotGdspx.call1("gdspx_on_sprite_vfx_finished", "OnSpriteVfxFinished", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_screen_exited__sig: 'vi',
	godot_js_spx_on_sprite_screen_exited: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_screen_exited");
		GodotGdspx.call1("gdspx_on_sprite_screen_exited", "OnSpriteScreenExited", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_sprite_screen_entered__sig: 'vi',
	godot_js_spx_on_sprite_screen_entered: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_sprite_screen_entered");
		GodotGdspx.call1("gdspx_on_sprite_screen_entered", "OnSpriteScreenEntered", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_mouse_pressed__sig: 'vi',
	godot_js_spx_on_mouse_pressed: function (keyid) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_mouse_pressed");
		GodotGdspx.call1("gdspx_on_mouse_pressed", "OnMousePressed", GodotGdspx.toCallbackInt(keyid, directHandler), directHandler);
	},

	godot_js_spx_on_mouse_released__sig: 'vi',
	godot_js_spx_on_mouse_released: function (keyid) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_mouse_released");
		GodotGdspx.call1("gdspx_on_mouse_released", "OnMouseReleased", GodotGdspx.toCallbackInt(keyid, directHandler), directHandler);
	},

	godot_js_spx_on_key_pressed__sig: 'vi',
	godot_js_spx_on_key_pressed: function (keyid) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_key_pressed");
		GodotGdspx.call1("gdspx_on_key_pressed", "OnKeyPressed", GodotGdspx.toCallbackInt(keyid, directHandler), directHandler);
	},

	godot_js_spx_on_key_released__sig: 'vi',
	godot_js_spx_on_key_released: function (keyid) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_key_released");
		GodotGdspx.call1("gdspx_on_key_released", "OnKeyReleased", GodotGdspx.toCallbackInt(keyid, directHandler), directHandler);
	},

	godot_js_spx_on_action_pressed__sig: 'vi',
	godot_js_spx_on_action_pressed: function (action_name) {
		GodotGdspx.call1("gdspx_on_action_pressed", "OnActionPressed", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_action_just_pressed__sig: 'vi',
	godot_js_spx_on_action_just_pressed: function (action_name) {
		GodotGdspx.call1("gdspx_on_action_just_pressed", "OnActionJustPressed", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_action_just_released__sig: 'vi',
	godot_js_spx_on_action_just_released: function (action_name) {
		GodotGdspx.call1("gdspx_on_action_just_released", "OnActionJustReleased", GodotRuntime.parseString(action_name));
	},

	godot_js_spx_on_axis_changed__sig: 'vif',
	godot_js_spx_on_axis_changed: function (action_name, value) {
		GodotGdspx.call2("gdspx_on_axis_changed", "OnAxisChanged", GodotRuntime.parseString(action_name), value);
	},

	godot_js_spx_on_collision_enter__sig: 'vii',
	godot_js_spx_on_collision_enter: function (self_id, other_id) {
		GodotGdspx.queueContact(1, self_id, other_id);
	},

	godot_js_spx_on_collision_stay__sig: 'vii',
	godot_js_spx_on_collision_stay: function (self_id, other_id) {
		GodotGdspx.queueContact(2, self_id, other_id);
	},

	godot_js_spx_on_collision_exit__sig: 'vii',
	godot_js_spx_on_collision_exit: function (self_id, other_id) {
		GodotGdspx.queueContact(3, self_id, other_id);
	},

	godot_js_spx_on_trigger_enter__sig: 'vii',
	godot_js_spx_on_trigger_enter: function (self_id, other_id) {
		GodotGdspx.queueContact(4, self_id, other_id);
	},

	godot_js_spx_on_trigger_stay__sig: 'vii',
	godot_js_spx_on_trigger_stay: function (self_id, other_id) {
		GodotGdspx.queueContact(5, self_id, other_id);
	},

	godot_js_spx_on_trigger_exit__sig: 'vii',
	godot_js_spx_on_trigger_exit: function (self_id, other_id) {
		GodotGdspx.queueContact(6, self_id, other_id);
	},

	godot_js_spx_on_ui_ready__sig: 'vi',
	godot_js_spx_on_ui_ready: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_ready");
		GodotGdspx.call1("gdspx_on_ui_ready", "OnUiReady", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_updated__sig: 'vi',
	godot_js_spx_on_ui_updated: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_updated");
		GodotGdspx.call1("gdspx_on_ui_updated", "OnUiUpdated", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_destroyed__sig: 'vi',
	godot_js_spx_on_ui_destroyed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_destroyed");
		GodotGdspx.call1("gdspx_on_ui_destroyed", "OnUiDestroyed", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_pressed__sig: 'vi',
	godot_js_spx_on_ui_pressed: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_pressed");
		GodotGdspx.call1("gdspx_on_ui_pressed", "OnUiPressed", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_released__sig: 'vi',
	godot_js_spx_on_ui_released: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_released");
		GodotGdspx.call1("gdspx_on_ui_released", "OnUiReleased", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_hovered__sig: 'vi',
	godot_js_spx_on_ui_hovered: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_hovered");
		GodotGdspx.call1("gdspx_on_ui_hovered", "OnUiHovered", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_clicked__sig: 'vi',
	godot_js_spx_on_ui_clicked: function (obj) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_clicked");
		GodotGdspx.call1("gdspx_on_ui_clicked", "OnUiClicked", GodotGdspx.toCallbackObj(obj, directHandler), directHandler);
	},

	godot_js_spx_on_ui_toggle__sig: 'vii',
	godot_js_spx_on_ui_toggle: function (obj, is_on) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_toggle");
		GodotGdspx.call2("gdspx_on_ui_toggle", "OnUiToggle", GodotGdspx.toCallbackObj(obj, directHandler), is_on, directHandler);
	},

	godot_js_spx_on_ui_text_changed__sig: 'vii',
	godot_js_spx_on_ui_text_changed: function (obj, text) {
		const directHandler = GodotGdspx.getDirectHandler("gdspx_on_ui_text_changed");
		GodotGdspx.call2("gdspx_on_ui_text_changed", "OnUiTextChanged", GodotGdspx.toCallbackObj(obj, directHandler), GodotRuntime.parseString(text), directHandler);
	},
};

autoAddDeps(GodotGdspx, '$GodotGdspx');
mergeInto(LibraryManager.library, GodotGdspx);
