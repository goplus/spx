"use strict";

// Owns the Web runner's input-session defaults and the browser/WASM data boundary.
// Persistence, baseline directories, and UI mode labels remain host concerns.
const spxInputSession = (() => {
	const INPUT_MODES = new Set(['normal', 'record', 'replay'])
	const MAX_REPLAY_BYTES = 16 * 1024 * 1024
	const hasOwn = (value, key) => Object.prototype.hasOwnProperty.call(value, key)

	function deepFreeze(value) {
		if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value
		Object.values(value).forEach(deepFreeze)
		return Object.freeze(value)
	}

	function assertObject(value, name) {
		if (!value || typeof value !== 'object' || Array.isArray(value)) {
			throw new TypeError(`${name} must be an object`)
		}
	}

	function assertKnownKeys(value, keys, name) {
		const unknown = Object.keys(value).find((key) => !keys.has(key))
		if (unknown) throw new TypeError(`${name} does not support ${unknown}`)
	}

	function captureKey(value, name) {
		if (value == null) return null
		if (typeof value !== 'string' || value.length === 0) {
			throw new TypeError(`${name} must be a non-empty key name string or null`)
		}
		return value
	}

	function recordingFPS(value) {
		if (!Number.isFinite(value) || value <= 0) {
			throw new RangeError('input recording FPS must be greater than zero')
		}
		return value
	}

	function valueOr(current, overrides, key) {
		return hasOwn(overrides, key) && overrides[key] !== undefined ? overrides[key] : current
	}

	const limits = deepFreeze({
		inputReplayBytes: MAX_REPLAY_BYTES,
	})

	let runnerConfig = deepFreeze({
		input: {
			defaultMode: 'normal',
			record: {
				fps: 30,
				captureKey: 'P',
			},
			replay: {
				captureKey: 'P',
			},
		},
		limits,
	})

	function configure(overrides = {}) {
		if (overrides == null) overrides = {}
		assertObject(overrides, 'runner config overrides')
		assertKnownKeys(overrides, new Set(['input']), 'runner config overrides')

		const inputOverrides = hasOwn(overrides, 'input') ? overrides.input : {}
		assertObject(inputOverrides, 'runner input config overrides')
		assertKnownKeys(inputOverrides, new Set(['defaultMode', 'record', 'replay']), 'runner input config overrides')

		const recordOverrides = hasOwn(inputOverrides, 'record') ? inputOverrides.record : {}
		const replayOverrides = hasOwn(inputOverrides, 'replay') ? inputOverrides.replay : {}
		assertObject(recordOverrides, 'record input config overrides')
		assertObject(replayOverrides, 'replay input config overrides')
		assertKnownKeys(recordOverrides, new Set(['fps', 'captureKey']), 'record input config overrides')
		assertKnownKeys(replayOverrides, new Set(['captureKey']), 'replay input config overrides')

		const current = runnerConfig.input
		const defaultMode = valueOr(current.defaultMode, inputOverrides, 'defaultMode')
		if (!INPUT_MODES.has(defaultMode)) {
			throw new Error(`Unsupported default input mode: ${defaultMode}`)
		}

		runnerConfig = deepFreeze({
			input: {
				defaultMode,
				record: {
					fps: recordingFPS(valueOr(current.record.fps, recordOverrides, 'fps')),
					captureKey: captureKey(valueOr(current.record.captureKey, recordOverrides, 'captureKey'), 'record captureKey'),
				},
				replay: {
					captureKey: captureKey(valueOr(current.replay.captureKey, replayOverrides, 'captureKey'), 'replay captureKey'),
				},
			},
			limits,
		})
		return runnerConfig
	}

	function create(mode = runnerConfig.input.defaultMode, overrides = {}) {
		if (mode == null) mode = 'normal'
		if (!INPUT_MODES.has(mode)) throw new Error(`Unsupported input session mode: ${mode}`)
		if (overrides == null) overrides = {}
		assertObject(overrides, 'input session overrides')

		if (mode === 'normal') {
			assertKnownKeys(overrides, new Set(['mode']), 'normal input session')
			return null
		}
		if (mode === 'record') {
			assertKnownKeys(overrides, new Set(['mode', 'fps', 'captureKey']), 'record input session')
			return {
				mode,
				fps: recordingFPS(valueOr(runnerConfig.input.record.fps, overrides, 'fps')),
				captureKey: captureKey(valueOr(runnerConfig.input.record.captureKey, overrides, 'captureKey'), 'record captureKey'),
			}
		}

		assertKnownKeys(overrides, new Set(['mode', 'data', 'captureKey']), 'replay input session')
		const input = {
			mode,
			captureKey: captureKey(valueOr(runnerConfig.input.replay.captureKey, overrides, 'captureKey'), 'replay captureKey'),
		}
		if (hasOwn(overrides, 'data')) input.data = overrides.data
		return input
	}

	async function replayBytes(data) {
		let bytes
		if (data && typeof data.size === 'number' && data.size > MAX_REPLAY_BYTES) {
			throw new RangeError(`input replay exceeds ${MAX_REPLAY_BYTES} bytes`)
		}

		const objectTag = data == null ? '' : Object.prototype.toString.call(data)
		if (ArrayBuffer.isView(data)) {
			bytes = new Uint8Array(data.buffer, data.byteOffset, data.byteLength)
		} else if (objectTag === '[object ArrayBuffer]') {
			bytes = new Uint8Array(data)
		} else if (data && typeof data.arrayBuffer === 'function') {
			bytes = new Uint8Array(await data.arrayBuffer())
		} else if (typeof data === 'string') {
			bytes = new TextEncoder().encode(data)
		} else if (data && typeof data === 'object') {
			const json = JSON.stringify(data)
			if (typeof json !== 'string') throw new TypeError('input replay object must be JSON serializable')
			bytes = new TextEncoder().encode(json)
		} else {
			throw new TypeError('input replay must be a Blob, string, object, ArrayBuffer, or Uint8Array')
		}

		if (bytes.byteLength > MAX_REPLAY_BYTES) {
			throw new RangeError(`input replay exceeds ${MAX_REPLAY_BYTES} bytes`)
		}
		return bytes.slice()
	}

	async function normalizeStartOptions(options = {}) {
		if (options == null) options = {}
		assertObject(options, 'startGame options')

		const requested = hasOwn(options, 'input') ? options.input : undefined
		if (requested === null) return { ...options, input: null }
		if (requested !== undefined) assertObject(requested, 'input session')

		const mode = requested && requested.mode !== undefined
			? requested.mode
			: runnerConfig.input.defaultMode
		const input = create(mode, requested || {})
		if (input && input.mode === 'replay') {
			input.data = await replayBytes(input.data)
		}
		return { ...options, input }
	}

	function replayBlob(data) {
		if (data == null) return null
		return new Blob([data], { type: 'application/json' })
	}

	Object.defineProperty(window, 'spxRunnerConfig', {
		enumerable: true,
		get: () => runnerConfig,
	})
	window.spxConfigureRunner = configure
	window.spxCreateInputSession = create

	return Object.freeze({
		normalizeStartOptions,
		replayBlob,
	})
})()
