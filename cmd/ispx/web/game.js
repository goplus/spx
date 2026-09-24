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
 * @typedef {{ [path: string]: FileMeta | null }} FilesMeta - Null entries need resync.
 */

class GameApp {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.game = null;
        this.packName = 'engine.zip';
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
        this.workerMode = EnginePackMode == "worker"
        this.minigameMode = EnginePackMode == "minigame"
        this.miniprogramMode = EnginePackMode == "miniprogram"
        this.normalMode = !this.workerMode && !this.minigameMode && !this.miniprogramMode
        this.logicWasmInstance = null
        this.go = null

        profiler.enabled = this.useProfiler;

        this.workerMessageManager = new globalThis.WorkerMessageManager();

        this.stopGameTask = 0;
        this.gameLifecycle = null;
        this.logVerbose("EnginePackMode: ", EnginePackMode)

        /**
         * Project files meta
         * @type FilesMeta
         */
        this.projectFilesMeta = Object.create(null);
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

    callWorkerFunction(funcName, ...args) {
        this.workerMessageManager.callWorkerFunction(funcName, ...args)
    }

    getInputSessionStatus() {
        return this.callInputReplayFunction('ispx_input_session_status')
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

    normalizeStartGameInput(options) {
        if (options == null) options = {}
        if (typeof options !== 'object' || Array.isArray(options)) {
            throw new TypeError('StartGame options must be an object')
        }
        return this.normalizeInputSession(options.input)
    }

    async initEngine() {
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame was called before engine initialization");
            return;
        }
        if (this.game) {
            this.logVerbose('A game is already running. Close it first');
            return;
        }

        const engineReady = profiler.profile('onRunPrepareEngineWasm', () => this.onRunPrepareEngineWasm());
        const logicReady = !this.workerMode && !this.minigameMode
            ? fetch(this.wasmURL('ispx.wasm')) : null;
        const packReady = fetch(this.assetURLs[this.packName]).then(response => response.arrayBuffer());
        // Handle early rejections until these downloads are awaited.
        if (logicReady) logicReady.catch(() => {});
        packReady.catch(() => {});
        await engineReady;
        if (this.stopGameTask > 0) return;

        let args = [
            '--main-pack', this.persistentPath + "/" + this.packName,
        ];
        if (this.recordingOnGameStart) {
            args.push('--write-movie', this.persistentPath + "/" + "movie.avi");
        }

        this.logVerbose("RunGame ", args);
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

        await profiler.profile('onRunBeforeInit', () => this.onRunBeforeInit(logicReady));
        this.onProgress(0.5);

        await profiler.profile('curGame.init',  () => curGame.init());

        this.onProgress(0.6);

        await profiler.profile('unpackData', () => packReady.then(data =>
            curGame.unpackEngineData(this.persistentPath, this.packName, data)));

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
        const previousMeta = this.projectFilesMeta;
        /** @type FilesMeta */
        const nextMeta = Object.create(null);
        for (const [path, { lastModified, content }] of Object.entries(files)) {
            // ZIP archives may include directory entries.
            if (path.endsWith('/')) {
                continue;
            }
            nextMeta[path] = { lastModified };
            const saved = previousMeta[path];
            if (saved != null && saved.lastModified === lastModified) {
                continue;
            }
            updatedFiles.push({ name: path, data: new Uint8Array(content) });
        }
        const removedPaths = Object.keys(previousMeta).filter(path => nextMeta[path] == null);

        try {
            this.game.updateAssetsData(this.persistentPath, updatedFiles);
            this.game.deleteAssetsData(this.persistentPath, removedPaths);
        } catch (error) {
            // Retry all known paths after partial changes.
            const paths = Object.keys({ ...previousMeta, ...nextMeta });
            this.projectFilesMeta = Object.fromEntries(paths.map(path => [path, null]));
            throw error;
        }
        this.projectFilesMeta = nextMeta;
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

    onProgress(value) {
        if (this.config.onProgress != null) {
            this.config.onProgress(value);
        }
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

    wasmURL(name) {
        const url = this.assetURLs[name]
        return isWasmCompressed ? url + '.br' : url
    }

    async onRunPrepareEngineWasm() {
        const url = this.wasmURL('engine.wasm')
        if (this.minigameMode) {
            this.gameConfig.wasmEngine = url
        } else if (!this.gameConfig.wasmEngine) {
            this.gameConfig.wasmEngine = await (await fetch(url)).arrayBuffer();
        }
    }

    async onRunBeforeInit(logicReady) {
        if (this.minigameMode) {
            GameGlobal.engine = this.game;
            godotSdk.set_engine(this.game);
            self['initExtensionWasm'] = function () { }
        } else if (!this.workerMode) {
            await profiler.profile('loadLogicWasm', () => this.loadLogicWasm(logicReady));
            await profiler.profile('runLogicWasm', () => this.runLogicWasm());
            self['initExtensionWasm'] = function () { }
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
            globalThis['FFI'] = self;
            await this.runLogicWasm()
        }
        if (this.workerMode) {
            let pthreads = game.getPThread()
            this.workerMessageManager.setPThreads(pthreads)
            this.workerMessageManager.callWorkerProjectDataUpdate(this.nonAssetFiles, this.assetURLs)
        } else {
            // register global functions
            Module = game.rtenv;
            globalThis['FFI'] = self;
            const res = window.ispx_start(inputSession);
            if (res instanceof Error) throw res;
            await this.waitInputSessionStarted(inputSession)
        }
    }

    async loadLogicWasm(logicReady) {
        const url = this.wasmURL('ispx.wasm')
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
            const { instance } = await WebAssembly.instantiateStreaming(logicReady || fetch(url), this.go.importObject);
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
        this.go.run(this.logicWasmInstance);
    }

}

globalThis.GameApp = GameApp;
