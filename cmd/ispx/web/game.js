var Module = null

/**
 * @typedef {Object} FileMeta
 * @property {number} lastModified Last modified time in milliseconds since Unix epoch.
 */

/**
 * @typedef {Object} FileWithMeta
 * @property {number} lastModified Last modified time in milliseconds since Unix epoch.
 * @property {ArrayBuffer} content File content as ArrayBuffer.
 */

/**
 * @typedef {{ [path: string]: FileWithMeta }} Files - File entries only; directories should be omitted.
 * @typedef {{ [path: string]: FileMeta }} FilesMeta
 */

/**
 * @typedef {'f64' | 'bool' | 'gdint' | 'string'} DirectCallbackArgType
 */

/**
 * @typedef {Object} DirectCallbackSpec
 * @property {string} name
 * @property {string} fallbackEventName
 * @property {DirectCallbackArgType[]} argTypes
 */

/**
 * Immutable global wrappers dispatch through this shared registry so rebinding
 * a GameApp updates handlers without replacing worker globals.
 */
const DIRECT_CALLBACK_HANDLER_SLOTS = globalThis.__spxDirectCallbackHandlerSlots || (globalThis.__spxDirectCallbackHandlerSlots = Object.create(null))
const DIRECT_CALLBACK_BRIDGE_STATE = globalThis.__spxDirectCallbackBridgeState || (globalThis.__spxDirectCallbackBridgeState = {
    handlers: DIRECT_CALLBACK_HANDLER_SLOTS,
})
if (DIRECT_CALLBACK_BRIDGE_STATE.handlers !== DIRECT_CALLBACK_HANDLER_SLOTS) {
    Object.assign(DIRECT_CALLBACK_HANDLER_SLOTS, DIRECT_CALLBACK_BRIDGE_STATE.handlers)
    DIRECT_CALLBACK_BRIDGE_STATE.handlers = DIRECT_CALLBACK_HANDLER_SLOTS
}

/**
 * String-bearing callbacks intentionally stay on gdspx_dispatch until we have
 * a lower-overhead ABI than a JS handle lookup round-trip.
 * @type {DirectCallbackSpec[]}
 */
const DIRECT_CALLBACK_SPECS = [
    { name: "gdspx_on_engine_update", fallbackEventName: "OnEngineUpdate", argTypes: ["f64"] },
    { name: "gdspx_on_engine_fixed_update", fallbackEventName: "OnEngineFixedUpdate", argTypes: ["f64"] },
    { name: "gdspx_on_engine_destroy", fallbackEventName: "OnEngineDestroy", argTypes: [] },
    { name: "gdspx_on_engine_reset", fallbackEventName: "OnEngineReset", argTypes: [] },
    { name: "gdspx_on_engine_pause", fallbackEventName: "OnEnginePause", argTypes: ["bool"] },
    { name: "gdspx_on_scene_sprite_instantiated", fallbackEventName: "OnSceneSpriteInstantiated", argTypes: ["gdint", "string"] },
    { name: "gdspx_on_sprite_ready", fallbackEventName: "OnSpriteReady", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_updated", fallbackEventName: "OnSpriteUpdated", argTypes: ["f64"] },
    { name: "gdspx_on_sprite_fixed_updated", fallbackEventName: "OnSpriteFixedUpdated", argTypes: ["f64"] },
    { name: "gdspx_on_sprite_destroyed", fallbackEventName: "OnSpriteDestroyed", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_frames_set_changed", fallbackEventName: "OnSpriteFramesSetChanged", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_animation_changed", fallbackEventName: "OnSpriteAnimationChanged", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_frame_changed", fallbackEventName: "OnSpriteFrameChanged", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_animation_looped", fallbackEventName: "OnSpriteAnimationLooped", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_animation_finished", fallbackEventName: "OnSpriteAnimationFinished", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_vfx_finished", fallbackEventName: "OnSpriteVfxFinished", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_screen_exited", fallbackEventName: "OnSpriteScreenExited", argTypes: ["gdint"] },
    { name: "gdspx_on_sprite_screen_entered", fallbackEventName: "OnSpriteScreenEntered", argTypes: ["gdint"] },
    { name: "gdspx_on_mouse_pressed", fallbackEventName: "OnMousePressed", argTypes: ["gdint"] },
    { name: "gdspx_on_mouse_released", fallbackEventName: "OnMouseReleased", argTypes: ["gdint"] },
    { name: "gdspx_on_key_pressed", fallbackEventName: "OnKeyPressed", argTypes: ["gdint"] },
    { name: "gdspx_on_key_released", fallbackEventName: "OnKeyReleased", argTypes: ["gdint"] },
    { name: "gdspx_on_action_pressed", fallbackEventName: "OnActionPressed", argTypes: ["string"] },
    { name: "gdspx_on_action_just_pressed", fallbackEventName: "OnActionJustPressed", argTypes: ["string"] },
    { name: "gdspx_on_action_just_released", fallbackEventName: "OnActionJustReleased", argTypes: ["string"] },
    { name: "gdspx_on_axis_changed", fallbackEventName: "OnAxisChanged", argTypes: ["string", "f64"] },
    { name: "gdspx_on_collision_enter", fallbackEventName: "OnCollisionEnter", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_collision_stay", fallbackEventName: "OnCollisionStay", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_collision_exit", fallbackEventName: "OnCollisionExit", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_trigger_enter", fallbackEventName: "OnTriggerEnter", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_trigger_stay", fallbackEventName: "OnTriggerStay", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_trigger_exit", fallbackEventName: "OnTriggerExit", argTypes: ["gdint", "gdint"] },
    { name: "gdspx_on_ui_ready", fallbackEventName: "OnUiReady", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_updated", fallbackEventName: "OnUiUpdated", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_destroyed", fallbackEventName: "OnUiDestroyed", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_pressed", fallbackEventName: "OnUiPressed", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_released", fallbackEventName: "OnUiReleased", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_hovered", fallbackEventName: "OnUiHovered", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_clicked", fallbackEventName: "OnUiClicked", argTypes: ["gdint"] },
    { name: "gdspx_on_ui_toggle", fallbackEventName: "OnUiToggle", argTypes: ["gdint", "bool"] },
    { name: "gdspx_on_ui_text_changed", fallbackEventName: "OnUiTextChanged", argTypes: ["gdint", "string"] },
]

class GameApp {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.game = null;
        this.packName = 'engine.zip';
        this.projectDataName = 'game.zip';
        this.persistentPath = 'engine';
        this.logLevel = config.logLevel;
        this.useProfiler = this.logLevel == LOG_LEVEL_VERBOSE;
        this.gameCanvas = config.gameCanvas;
        this.assetURLs = config.assetURLs;
        this.gameConfig = {
            "executable": "engine",
            'unloadAfterInit': false,
            'canvas': this.gameCanvas,
            'logLevel': this.logLevel,
            'canvasResizePolicy': 2,
            'onExit': (code) => {
                this.onGodotExit(code)
            },
        };
        this.recordingOnGameStart = config.recordingOnGameStart || false
        this.autoDownloadRecordedVideo = config.autoDownloadRecordedVideo || false
        this.logicPromise = Promise.resolve();
        // web worker mode
        this.workerMode = EnginePackMode == "worker"
        this.minigameMode = EnginePackMode == "minigame"
        this.miniprogramMode = EnginePackMode == "miniprogram"
        this.normalMode = !this.workerMode && !this.minigameMode && !this.miniprogramMode

        profiler.enabled = this.useProfiler;

        // init worker message manager
        this.workerMessageManager = new globalThis.WorkerMessageManager();

        this.stopGameTask = 0;  
        this.gameLifecycle = null;
        this.logVerbose("EnginePackMode: ", EnginePackMode)

        /**
         * Project files meta
         * @type FilesMeta
         */
        this.projectFilesMeta = {};
    }
    logVerbose(...args) {
        if (this.logLevel == LOG_LEVEL_VERBOSE) {
            console.log(...args);
        }
    }

    startTask(taskFunc) {
        const originalPromise = this.logicPromise;
        const newPromise = this.logicPromise.then(() => taskFunc());
        this.logicPromise = newPromise;
        newPromise.catch((err) => {
            // If an error occurs, reset logicPromise to originalPromise to avoid blocking subsequent tasks.
            if (this.logicPromise === newPromise) this.logicPromise = originalPromise;
        })
        return this.logicPromise
    }


    async InitEngine() {
        return this.startTask(() => this.initEngine())
    }

    /**
     * Initialize game with given game files. It is expected to be called after `InitEngine`, while before `StartGame`.
     * @param {Files} files 
     * @returns Promise<void>
     */
    async InitGame(files) {
        return this.startTask(() => this.initGame(files))
    }

    async StartGame(options = {}) {
        const inputSession = options && typeof options.then === 'function'
            ? Promise.resolve(options).then((resolved) => this.normalizeStartGameInput(resolved))
            : this.normalizeStartGameInput(options)
        return this.startTask(async () => this.startGame(await inputSession))
    }

    normalizeStartGameInput(options) {
        if (options == null) options = {}
        if (typeof options !== 'object' || Array.isArray(options)) {
            throw new TypeError('StartGame options must be an object')
        }
        return this.normalizeInputSession(options.input)
    }

    async ResetGame() {
        this.stopGameTask++;
        return this.startTask(() => this.stopGame(false))
    }

    async StopGame(beforeStop = null) {
        if (beforeStop != null && typeof beforeStop !== 'function') {
            throw new TypeError('beforeStop must be a function')
        }
        this.stopGameTask++;
        return this.startTask(() => this.stopGame(true, beforeStop))
    }

    async initEngine() {
        await profiler.profile('onRunPrepareEngineWasm', () => this.onRunPrepareEngineWasm());

        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game");
            return;
        }

        let args = [
            '--main-pack', this.persistentPath + "/" + this.packName,
            '--main-project-data', this.persistentPath + "/" + this.projectDataName,
        ];
        if (this.recordingOnGameStart) {
            args.push('--write-movie', this.persistentPath + "/" + "movie.avi");
        }

        this.logVerbose("RunGame ", args);
        if (this.game) {
            this.logVerbose('A game is already running. Close it first');
            resolve();
            return;
        }

        this.onProgress(0.5);
        this.game = new Engine(this.gameConfig);
        let curGame = this.game;

        // register global functions
        window.go_wasm_init = function () { }
        window.gdspx_dispatch = function () { }
        const spxfuncs = new GdspxFuncs();
        const methodNames = Object.getOwnPropertyNames(Object.getPrototypeOf(spxfuncs));
        methodNames.forEach(key => {
            if (key.startsWith('gdspx_') && typeof spxfuncs[key] === 'function') {
                globalThis[key] = spxfuncs[key].bind(spxfuncs);
            }
        });

        await profiler.profile('onRunBeforeInit', () => this.onRunBeforeInit());
        this.onProgress(0.5);

        await profiler.profile('curGame.init',  () => curGame.init());

        this.onProgress(0.6);

        await profiler.profile('unpackData', () => this.unpackEngineData(curGame));

        this.onProgress(0.7);

        await profiler.profile('onRunAfterInit', () => this.onRunAfterInit(curGame));

        this.onProgress(0.8);

        await profiler.profile('curGame.start', () => curGame.start({ 'args': args, 'canvas': this.gameCanvas }));

        this.onProgress(1.0);
        this.logVerbose("==> engine start done");
    }

    /**
     * @private Initialize game with given game files
     * @param {Files} files 
     * @returns Promise<void>
     */
    async initGame(files) {
        await profiler.profile('updateEngineFiles', () => this.updateEngineFiles(files));
        await profiler.profile('buildGame', () => this.buildGame(files));
    }

    /**
     * (Incrementally) Update engine files with given game files.
     * @param {Files} files
     */
    updateEngineFiles(files) {
        /** @type Array<{ name: string, data: Uint8Array }> */
        const updatedFiles = [];
        const savedFilesMeta = this.projectFilesMeta;
        /** @type FilesMeta */
        const filesMeta = {};
        Object.entries(files).forEach(([path, { lastModified, content }]) => {
            filesMeta[path] = { lastModified };
            const savedFileMeta = savedFilesMeta[path];
            if (savedFileMeta != null && savedFileMeta.lastModified === lastModified) {
                return; // file not changed, skip
            }
            updatedFiles.push({ name: path, data: new Uint8Array(content) });
        });
        this.game.updateAssetsData(this.persistentPath, updatedFiles)
        this.projectFilesMeta = filesMeta;

        /** @type Array<string> */
        const removedFilePaths = [];
        Object.entries(savedFilesMeta).forEach(([path, _]) => {
            if (filesMeta[path] == null) {
                removedFilePaths.push(path);
            }
        });
        this.game.deleteAssetsData(this.persistentPath, removedFilePaths);
    }

    /**
     * Do spx build with given game files
     * @param {Files} files
     */
    buildGame(files) {
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game");
            return;
        }
        /** @type {{ [path: string]: Uint8Array }} */
        const nonAssetFiles = {};
        Object.entries(files).forEach(([path, file]) => {
            // `.spx` and `.json` files are treated as non-asset files
            if (path.endsWith(".spx") || path.endsWith('.json')) {
                nonAssetFiles[path] = new Uint8Array(file.content);
            }
        });
        if (!this.workerMode) {
            const res = window.ispx_build(nonAssetFiles);
            if (res instanceof Error) throw res;
        }else{
            this.nonAssetFiles = nonAssetFiles;
        }
    }

    async startGame(inputSession) {
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game");
            return;
        }

        let curGame = this.game;
        profiler.mark('RunGame Start');
        await profiler.profile('restart', () => this.restart());
        const lifecycle = this.beginGameLifecycle(inputSession)
        try {
            await profiler.profile('onRunAfterStart', () => this.onRunAfterStart(curGame, inputSession));
        } catch (error) {
            this.completeGameLifecycle(undefined, lifecycle)
            throw error
        }
        this.gameCanvas.focus();
        profiler.mark('RunGame Done');
        profiler.measure('RunGame Start', 'RunGame Done');
    }

    downloadRecordedVideo(fileName) { 
        Module.downloadRecordedVideo(fileName)
    }

    getRecordedVideo() { 
        return Module.getRecordedVideoBlob()
    }

    startRecording() {
        Module.tryStartRecording()
    }

    async stopRecording() {
        return await Module.tryStopRecording()
    } 

    onGodotExit(code) {
        this.completeGameLifecycle(code)
        this.game = null
        if (this.config.handleGodotExit != null) {
            this.config.handleGodotExit(code);
        }
 
    }
    async stopGame(finishInputRecording, beforeStop = null) {
        this.stopGameTask--
        const lifecycle = this.gameLifecycle
        if (this.game == null || lifecycle == null || lifecycle.completed) {
            this.logVerbose("No Game Is Running")
            return { inputReplay: lifecycle?.inputReplay ?? null, stopped: false }
        }

        let inputReplay = null
        let inputReplayError = null
        if (finishInputRecording && this.normalMode && lifecycle.inputSession?.mode === 'record') {
            try {
                inputReplay = this.finishInputRecording()
                lifecycle.inputReplay = inputReplay
            } catch (error) {
                inputReplayError = error
            }
        }

        let stopError = inputReplayError
        if (beforeStop != null && inputReplayError == null) {
            try {
                await beforeStop({ inputReplay })
            } catch (error) {
                stopError = error
            }
        }

        const result = window.ispx_stop()
        if (result instanceof Error) {
            if (stopError == null) stopError = result
        } else {
            try {
                await lifecycle.exit
            } catch (error) {
                if (stopError == null) stopError = error
            }
        }

        if(this.recordingOnGameStart && this.autoDownloadRecordedVideo){
            let fileName = `spx_${new Date().getTime()}.webm`;
            this.downloadRecordedVideo(fileName)
        }
        if (stopError != null) throw stopError
        return { inputReplay, stopped: true }
    }

    beginGameLifecycle(inputSession) {
        if (this.gameLifecycle != null && !this.gameLifecycle.completed) {
            throw new Error('A game is already running')
        }
        let resolveExit
        const lifecycle = {
            completed: false,
            inputSession,
            inputReplay: null,
            exit: new Promise((resolve) => {
                resolveExit = resolve
            }),
            resolveExit: null,
        }
        lifecycle.resolveExit = resolveExit
        this.gameLifecycle = lifecycle
        return lifecycle
    }

    completeGameLifecycle(code, lifecycle = this.gameLifecycle) {
        if (lifecycle == null || lifecycle.completed) return false
        lifecycle.completed = true
        lifecycle.resolveExit(code)
        return true
    }

    onRuntimeReset(code) {
        this.completeGameLifecycle(code)
    }

    restart() {
        let funPtr = this.game.rtenv["_gdspx_ext_request_restart"]
        if(funPtr != null){
            funPtr()
        }
    }

    pause() {
        if (this.game == null || this.game.rtenv == null) return
        let funPtr = this.game.rtenv["_gdspx_ext_pause"]
        if(funPtr != null){
            funPtr()
        }
    }

    isPaused() {
        if (this.game == null || this.game.rtenv == null) return false
        const funPtr = this.game.rtenv["_gdspx_ext_is_paused"]
        return funPtr != null && !!funPtr()
    }

    resume() {
        let funPtr = this.game.rtenv["_gdspx_ext_resume"]
        if(funPtr != null){
            funPtr()
        }
    }

    stepNextFrame() {
        let funPtr = this.game.rtenv["_gdspx_ext_next_frame"]
        if(funPtr != null){
            funPtr()
        }
    }
    //------------------ misc ------------------
    onProgress(value) {
        if (this.config.onProgress != null) {
            this.config.onProgress(value);
        }
    }

    async unpackEngineData(game) {
        let packUrl = this.assetURLs[this.packName]
        let pckData = await (await fetch(packUrl)).arrayBuffer()
        await game.unpackEngineData(this.persistentPath, this.packName, pckData)
    }

    callWorkerFunction(funcName, ...args) {
        this.workerMessageManager.callWorkerFunction(funcName, ...args)
    }

    ensureInputReplaySupported() {
        if (!this.normalMode) {
            throw new Error('Input recording and replay are only supported in normal Web mode')
        }
    }

    callInputReplayFunction(funcName, ...args) {
        this.ensureInputReplaySupported()
        const fn = window[funcName]
        if (typeof fn !== 'function') {
            throw new Error(`${funcName} is not available`)
        }
        const result = fn(...args)
        if (result instanceof Error) throw result
        return result
    }

    normalizeInputSession(input) {
        if (input == null) return null
        this.ensureInputReplaySupported()
        if (typeof input !== 'object' || Array.isArray(input)) {
            throw new TypeError('input session must be an object')
        }
        if (input.mode === 'record') {
            // Keep the low-level fallback for hosts that instantiate GameApp directly.
            // The runner facade supplies its configured value before reaching this layer.
            const fps = input.fps == null ? 30 : input.fps
            if (!Number.isFinite(fps) || fps <= 0) {
                throw new RangeError('input recording FPS must be greater than zero')
            }
            return { mode: 'record', fps, captureKey: this.normalizeCaptureKey(input.captureKey) }
        }
        if (input.mode === 'replay') {
            if (input.data == null) {
                throw new TypeError('input replay data is required')
            }
            const objectTag = Object.prototype.toString.call(input.data)
            let data
            if (ArrayBuffer.isView(input.data)) {
                data = new Uint8Array(input.data.buffer, input.data.byteOffset, input.data.byteLength).slice()
            } else if (objectTag === '[object ArrayBuffer]') {
                data = input.data.slice(0)
            } else if (typeof input.data === 'string') {
                data = input.data
            } else {
                throw new TypeError('input replay data must be a string, ArrayBuffer, or Uint8Array')
            }
            return { mode: 'replay', data, captureKey: this.normalizeCaptureKey(input.captureKey) }
        }
        throw new Error(`Unsupported input session mode: ${input.mode}`)
    }

    normalizeCaptureKey(value) {
        if (value == null) return null
        if (typeof value !== 'string' || value.length === 0) {
            throw new TypeError('input session captureKey must be a non-empty key name string')
        }
        return value
    }

    finishInputRecording() {
        return this.callInputReplayFunction('ispx_input_recording_finish')
    }

    getInputSessionStatus() {
        return this.callInputReplayFunction('ispx_input_session_status')
    }

    async waitInputSessionStarted(input, timeoutMs = 30000) {
        if (input == null) return
        const startedAt = Date.now()
        while (true) {
            const status = this.getInputSessionStatus()
            if (status.phase === 'running' || status.phase === 'finishing' || status.phase === 'completed') {
                return status
            }
            if (status.phase === 'aborted') {
                throw new Error(status.error || 'input session was aborted during startup')
            }
            if (status.mode === 'idle') {
                throw new Error('input session was not attached to the game')
            }
            if (Date.now() - startedAt >= timeoutMs) {
                throw new Error(`timed out waiting for input session startup after ${timeoutMs}ms`)
            }
            await new Promise((resolve) => setTimeout(resolve, 10))
        }
    }

    async waitInputSessionCompleted(options = {}) {
        this.ensureInputReplaySupported()
        if (options == null) options = {}
        if (typeof options !== 'object' || Array.isArray(options)) {
            throw new TypeError('wait options must be an object')
        }
        const pollIntervalMs = options.pollIntervalMs == null ? 50 : options.pollIntervalMs
        const timeoutMs = options.timeoutMs == null ? 0 : options.timeoutMs
        if (!Number.isFinite(pollIntervalMs) || pollIntervalMs < 0) {
            throw new RangeError('pollIntervalMs must be a non-negative number')
        }
        if (!Number.isFinite(timeoutMs) || timeoutMs < 0) {
            throw new RangeError('timeoutMs must be a non-negative number')
        }

        const startedAt = Date.now()
        while (true) {
            if (options.signal && options.signal.aborted) {
                throw options.signal.reason || new DOMException('The operation was aborted', 'AbortError')
            }

            const status = this.getInputSessionStatus()
            const completed = status.completed === true || status.phase === 'completed'
            if (completed) {
                return status
            }
            if (status.phase === 'aborted') {
                throw new Error(status.error || 'input replay was aborted')
            }
            if (status.mode !== 'replay' && status.mode !== 'replaying') {
                throw new Error(`input replay is not running: ${status.mode}`)
            }
            if (timeoutMs > 0 && Date.now() - startedAt >= timeoutMs) {
                throw new Error(`timed out waiting for input replay after ${timeoutMs}ms`)
            }
            await new Promise((resolve) => setTimeout(resolve, pollIntervalMs))
        }
    }

    bindDirectCallbackBridge() {
        if (!this.logicWasmInstance || !this.logicWasmInstance.exports) {
            return
        }

        DIRECT_CALLBACK_SPECS.forEach((spec) => this.bindDirectCallback(spec))
    }

    installDirectCallbackGlobal(name, fallbackEventName) {
        const descriptor = Object.getOwnPropertyDescriptor(globalThis, name)
        if (descriptor && !descriptor.configurable) {
            return
        }

        Object.defineProperty(globalThis, name, {
            value: (...args) => {
                const handler = DIRECT_CALLBACK_HANDLER_SLOTS[name]
                if (typeof handler === 'function') {
                    return handler(...args)
                }
                return globalThis.gdspx_dispatch(fallbackEventName, ...args)
            },
            writable: false,
            enumerable: true,
            configurable: false,
        })
    }

    bindDirectCallback(spec) {
        const { name, fallbackEventName } = spec
        this.installDirectCallbackGlobal(name, fallbackEventName)
        DIRECT_CALLBACK_BRIDGE_STATE.handlers[name] = this.createDirectCallbackHandler(spec)
    }

    createDirectCallbackHandler(spec) {
        const { name, argTypes = [] } = spec
        const directExport = this.logicWasmInstance.exports[name]
        if (typeof directExport !== 'function' || this.shouldUseDispatchFallback(argTypes)) {
            return null
        }

        return (...args) => directExport(...this.encodeDirectCallbackArgs(argTypes, args))
    }

    shouldUseDispatchFallback(argTypes) {
        return Array.isArray(argTypes) && argTypes.includes('string')
    }

    encodeDirectCallbackArgs(argTypes, args) {
        if (!argTypes || argTypes.length === 0) {
            return args
        }
        return args.map((arg, index) => this.encodeDirectCallbackArg(argTypes[index], arg))
    }

    encodeDirectCallbackArg(argType, arg) {
        switch (argType) {
            case 'bool':
                return arg ? 1 : 0
            case 'gdint':
                return this.gdIntToBigInt(arg)
            default:
                return arg
        }
    }

    gdIntToBigInt(value) {
        if (typeof value === 'bigint') {
            return value
        }
        // Older callback bridges still pass the legacy {low, high} int64
        // encoding, so keep this coercion as a compatibility backstop.
        if (value && typeof value.low === 'number' && typeof value.high === 'number') {
            return BigInt.asIntN(64, (BigInt(value.high >>> 0) << 32n) | BigInt(value.low >>> 0))
        }
        throw new TypeError(`unsupported gdint value: ${value}`)
    }


    //------------------ onRun ------------------
    async onRunPrepareEngineWasm() {
        let url = this.assetURLs["engine.wasm"]
        if (isWasmCompressed) {
            url += ".br"
        }

        if (this.minigameMode) {
            this.gameConfig.wasmEngine = url
        } else {
            if (!this.gameConfig.wasmEngine) {
                this.gameConfig.wasmEngine = await (await fetch(url)).arrayBuffer();
            }
        }
    }

    async onRunBeforeInit() {
        if (this.minigameMode) {
            GameGlobal.engine = this.game;
            godotSdk.set_engine(this.game);
            self.initExtensionWasm = function () { }
        } else {
            if (!this.workerMode) {
                await profiler.profile('loadLogicWasm', () => this.loadLogicWasm());
                await profiler.profile('runLogicWasm', () => this.runLogicWasm());
                self.initExtensionWasm = function () { }
            }
        }
    }

    async onRunAfterInit(game) {
        if (this.workerMode) {
            this.workerMessageManager.bindMainThreadCallbacks(game)
        }
        if (this.minigameMode) {
            await this.loadLogicWasm()
        }
    }

    async onRunAfterStart(game, inputSession) {
        if (this.minigameMode) {
            FFI = self;
            await this.runLogicWasm()
        }
        if (this.workerMode) {
            let pthreads = game.getPThread()
            this.workerMessageManager.setPThreads(pthreads)
            this.workerMessageManager.callWorkerProjectDataUpdate(this.nonAssetFiles, this.assetURLs)
        } else {
            // register global functions
            Module = game.rtenv;
            FFI = self;
            const res = window.ispx_start(inputSession);
            if (res instanceof Error) throw res;
            await this.waitInputSessionStarted(inputSession)
        }
    }

    //------------------ logic wasm ------------------
    async loadLogicWasm() {
        // load wasm
        let url = this.config.assetURLs["ispx.wasm"];
        if (isWasmCompressed) {
            url += ".br"
        }
        this.go = new Go();
        if (this.minigameMode) {
            // load wasm in miniEngine
            const wasmResult = await WebAssembly.instantiate(url, this.go.importObject);
            // create compatible instance
            this.logicWasmInstance = Object.create(WebAssembly.Instance.prototype);
            this.logicWasmInstance.exports = wasmResult.instance.exports;
            Object.defineProperty(this.logicWasmInstance, 'constructor', {
                value: WebAssembly.Instance,
                writable: false,
                enumerable: false,
                configurable: true
            });
        } else {
            const { instance } = await WebAssembly.instantiateStreaming(fetch(url), this.go.importObject);
            this.logicWasmInstance = instance;
        }
    }

    notifyExit(code) {
        if (typeof window.onGoWasmExit === "function") {
            window.onGoWasmExit(code);
        }

        window.dispatchEvent(new CustomEvent("logicWasmExit", { detail: { code } }));

        if (window.parent !== window) {
            window.parent.postMessage({ type: "EngineCrash", code }, "*");
        }
    }

    async runLogicWasm() {
        this.go.exit = (code) => {
            this.notifyExit(code);
        };
        this.bindDirectCallbackBridge();
        this.go.run(this.logicWasmInstance);
    }
}

// export GameApp to global
globalThis.GameApp = GameApp;
