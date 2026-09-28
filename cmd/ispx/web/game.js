var Module = null

/**
 * @typedef {{ lastModified: number }} FileMeta - Timestamp in milliseconds since Unix epoch.
 * @typedef {{ [path: string]: FileMeta | null }} FilesMeta - Null entries need resync.
 */

class GameRunner {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.engine = null;
        this.packName = 'engine.zip';
        this.persistentPath = 'engine';
        this.logLevel = config.logLevel;
        this.useProfiler = this.logLevel == LOG_LEVEL_VERBOSE;
        this.canvas = config.gameCanvas;
        this.assetURLs = config.assetURLs;
        this.engineConfig = {
            executable: 'engine',
            unloadAfterInit: false,
            canvas: this.canvas,
            logLevel: this.logLevel,
            canvasResizePolicy: 2,
            onExit: (code) => {
                this.onGodotExit(code)
            },
        };
        this.recordingOnGameStart = config.recordingOnGameStart || false
        this.autoDownloadRecordedVideo = config.autoDownloadRecordedVideo || false
        this.taskTail = Promise.resolve();
        this.workerMode = EnginePackMode == "worker"
        this.minigameMode = EnginePackMode == "minigame"
        this.miniprogramMode = EnginePackMode == "miniprogram"
        this.normalMode = !this.workerMode && !this.minigameMode && !this.miniprogramMode
        this.logicWasmInstance = null
        this.go = null

        profiler.enabled = this.useProfiler;

        this.workerMessageManager = new globalThis.WorkerMessageManager();

        this.pendingStops = 0;
        this.gameLifecycle = null;
        this.logVerbose('Engine mode:', EnginePackMode)

        /** @type {FilesMeta} */
        this.projectFilesMeta = Object.create(null);
    }

    async InitEngine() {
        return this.enqueue(() => this.initEngine())
    }

    // Call after InitEngine and before StartGame.
    async InitGame(files) {
        return this.enqueue(() => this.initGame(files))
    }

    async StartGame(options = {}) {
        const inputSession = options && typeof options.then === 'function'
            ? Promise.resolve(options).then((resolved) => this.normalizeStartInput(resolved))
            : this.normalizeStartInput(options)
        return this.enqueue(async () => this.startGame(await inputSession))
    }

    async ResetGame() {
        return this.enqueueStop(false)
    }

    async StopGame(beforeStop = null) {
        if (beforeStop != null && typeof beforeStop !== 'function') {
            throw new TypeError('beforeStop must be a function')
        }
        return this.enqueueStop(true, beforeStop)
    }

    pause() {
        if (this.engine == null || this.engine.rtenv == null) return
        this.callRuntime('_gdspx_ext_pause')
    }

    isPaused() {
        if (this.engine == null || this.engine.rtenv == null) return false
        return !!this.callRuntime('_gdspx_ext_is_paused')
    }

    resume() {
        this.callRuntime('_gdspx_ext_resume')
    }

    stepNextFrame() {
        this.callRuntime('_gdspx_ext_next_frame')
    }

    startRecording() {
        Module.tryStartRecording()
    }

    async stopRecording() {
        return await Module.tryStopRecording()
    }

    getRecordedVideo() {
        return Module.getRecordedVideoBlob()
    }

    downloadRecordedVideo(fileName) {
        Module.downloadRecordedVideo(fileName)
    }

    getInputSessionStatus() {
        return this.callInputFunction('ispx_input_session_status')
    }

    async waitInputSessionCompleted(options = {}) {
        this.ensureInputSessionSupported()
        if (options == null) options = {}
        if (typeof options !== 'object' || Array.isArray(options)) {
            throw new TypeError('Wait options must be an object')
        }
        const pollIntervalMs = options.pollIntervalMs == null ? 50 : options.pollIntervalMs
        const timeoutMs = options.timeoutMs == null ? 0 : options.timeoutMs
        if (!Number.isFinite(pollIntervalMs) || pollIntervalMs < 0) {
            throw new RangeError('pollIntervalMs must be a finite, non-negative number')
        }
        if (!Number.isFinite(timeoutMs) || timeoutMs < 0) {
            throw new RangeError('timeoutMs must be a finite, non-negative number')
        }

        const startedAt = Date.now()
        while (true) {
            if (options.signal && options.signal.aborted) {
                throw options.signal.reason || new DOMException('Input replay wait aborted', 'AbortError')
            }

            const status = this.getInputSessionStatus()
            if (status.completed === true || status.phase === 'completed') {
                return status
            }
            if (status.phase === 'aborted') {
                throw new Error(status.error || 'Input replay aborted')
            }
            if (status.mode !== 'replay' && status.mode !== 'replaying') {
                throw new Error(`Input replay is not running (mode: ${status.mode})`)
            }
            if (timeoutMs > 0 && Date.now() - startedAt >= timeoutMs) {
                throw new Error(`Input replay did not complete within ${timeoutMs} ms`)
            }
            await new Promise((resolve) => setTimeout(resolve, pollIntervalMs))
        }
    }

    onRuntimeReset(code) {
        this.completeGameLifecycle(code)
    }

    onGodotExit(code) {
        this.completeGameLifecycle(code)
        this.engine = null
        if (this.config.handleGodotExit != null) {
            this.config.handleGodotExit(code);
        }
    }

    callWorkerFunction(name, ...args) {
        this.workerMessageManager.callWorkerFunction(name, ...args)
    }

    enqueue(task) {
        const result = this.taskTail.then(() => task());
        this.taskTail = result;
        result.catch(() => {
            if (this.taskTail === result) this.taskTail = Promise.resolve();
        });
        return result;
    }

    async enqueueStop(finishRecording, beforeStop = null) {
        this.pendingStops++;
        try {
            return await this.enqueue(() => this.stopGame(finishRecording, beforeStop));
        } finally {
            this.pendingStops--;
        }
    }

    async initEngine() {
        if (this.pendingStops > 0) {
            this.logVerbose('Engine initialization skipped: stop requested');
            return;
        }
        if (this.engine) {
            this.logVerbose('Engine initialization skipped: engine already exists');
            return;
        }

        const engineReady = profiler.profile('loadEngineWasm', () => this.loadEngineWasm());
        const logicReady = !this.workerMode && !this.minigameMode
            ? fetch(this.wasmURL('ispx.wasm')) : null;
        const packReady = fetch(this.assetURLs[this.packName]).then(response => response.arrayBuffer());
        // Handle early rejections until these downloads are awaited.
        if (logicReady) logicReady.catch(() => {});
        packReady.catch(() => {});
        await engineReady;
        if (this.pendingStops > 0) return;

        const args = ['--main-pack', `${this.persistentPath}/${this.packName}`];
        if (this.recordingOnGameStart) {
            args.push('--write-movie', `${this.persistentPath}/movie.avi`);
        }

        this.logVerbose('Starting engine:', args);
        this.onProgress(0.5);
        this.engine = new Engine(this.engineConfig);
        const engine = this.engine;
        this.bindRuntimeCallbacks();
        await profiler.profile('prepareRuntime', () => this.prepareRuntime(logicReady));
        this.onProgress(0.5);

        await profiler.profile('engine.init', () => engine.init());

        this.onProgress(0.6);

        await profiler.profile('unpackEngineData', () => packReady.then(data =>
            engine.unpackEngineData(this.persistentPath, this.packName, data)));

        this.onProgress(0.7);

        await profiler.profile('initRuntime', () => this.initRuntime(engine));

        this.onProgress(0.8);

        await profiler.profile('engine.start', () => engine.start({ args, canvas: this.canvas }));

        this.onProgress(1.0);
        this.logVerbose('Engine started');
    }

    async loadEngineWasm() {
        const url = this.wasmURL('engine.wasm')
        if (this.minigameMode) {
            this.engineConfig.wasmEngine = url
        } else if (!this.engineConfig.wasmEngine) {
            this.engineConfig.wasmEngine = await (await fetch(url)).arrayBuffer();
        }
    }

    bindRuntimeCallbacks() {
        window.go_wasm_init = function () { }
        window.gdspx_dispatch = function () { }
        const callbacks = new GdspxFuncs();
        const names = Object.getOwnPropertyNames(Object.getPrototypeOf(callbacks));
        for (const name of names) {
            if (name.startsWith('gdspx_') && typeof callbacks[name] === 'function') {
                globalThis[name] = callbacks[name].bind(callbacks);
            }
        }
    }

    async prepareRuntime(logicReady) {
        if (this.minigameMode) {
            GameGlobal.engine = this.engine;
            godotSdk.set_engine(this.engine);
        } else if (this.workerMode) {
            return;
        } else {
            await profiler.profile('loadLogicWasm', () => this.loadLogicWasm(logicReady));
            await profiler.profile('runLogicWasm', () => this.runLogicWasm());
        }
        self.initExtensionWasm = function () { }
    }

    async initRuntime(engine) {
        if (this.workerMode) {
            this.workerMessageManager.bindMainThreadCallbacks(engine)
        }
        if (this.minigameMode) {
            await this.loadLogicWasm()
        }
    }

    async initGame(files) {
        await profiler.profile('updateEngineFiles', () => this.updateEngineFiles(files));
        await profiler.profile('buildGame', () => this.buildGame(files));
    }

    updateEngineFiles(files) {
        /** @type {Array<{ name: string, data: Uint8Array }>} */
        const updatedFiles = [];
        const previousMeta = this.projectFilesMeta;
        /** @type {FilesMeta} */
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
            this.engine.updateAssetsData(this.persistentPath, updatedFiles);
            this.engine.deleteAssetsData(this.persistentPath, removedPaths);
        } catch (error) {
            // Retry all known paths after partial changes.
            const paths = Object.keys({ ...previousMeta, ...nextMeta });
            this.projectFilesMeta = Object.fromEntries(paths.map(path => [path, null]));
            throw error;
        }
        this.projectFilesMeta = nextMeta;
    }

    buildGame(files) {
        if (this.pendingStops > 0) {
            this.logVerbose('Game build skipped: stop requested');
            return;
        }
        /** @type {{ [path: string]: Uint8Array }} */
        const nonAssetFiles = {};
        for (const [path, file] of Object.entries(files)) {
            if (path.endsWith(".spx") || path.endsWith('.json')) {
                nonAssetFiles[path] = new Uint8Array(file.content);
            }
        }
        if (!this.workerMode) {
            const result = window.ispx_build(nonAssetFiles);
            if (result instanceof Error) throw result;
        } else {
            this.nonAssetFiles = nonAssetFiles;
        }
    }

    async startGame(inputSession) {
        if (this.pendingStops > 0) {
            this.logVerbose('Game start skipped: stop requested');
            return;
        }

        const engine = this.engine;
        profiler.mark('game.start.begin');
        await profiler.profile('restart', () => this.restart());
        const lifecycle = this.beginGameLifecycle(inputSession)
        try {
            await profiler.profile('runProject', () => this.runProject(engine, inputSession));
        } catch (error) {
            this.completeGameLifecycle(undefined, lifecycle)
            throw error
        }
        this.canvas.focus();
        profiler.mark('game.start.end');
        profiler.measure('game.start.begin', 'game.start.end');
    }

    async runProject(engine, inputSession) {
        if (this.minigameMode) {
            globalThis.FFI = self;
            this.runLogicWasm()
        }
        if (this.workerMode) {
            const threads = engine.getPThread()
            this.workerMessageManager.setPThreads(threads)
            this.workerMessageManager.callWorkerProjectDataUpdate(this.nonAssetFiles, this.assetURLs)
        } else {
            Module = engine.rtenv;
            globalThis.FFI = self;
            const result = window.ispx_start(inputSession);
            if (result instanceof Error) throw result;
            await this.waitInputSessionStarted(inputSession)
        }
    }

    async stopGame(finishRecording, beforeStop = null) {
        const lifecycle = this.gameLifecycle
        if (this.engine == null || lifecycle == null || lifecycle.completed) {
            this.logVerbose('No game is running')
            return { inputReplay: lifecycle?.inputReplay ?? null, stopped: false }
        }

        let inputReplay = null
        let stopError = null
        if (finishRecording && this.normalMode && lifecycle.inputSession?.mode === 'record') {
            try {
                inputReplay = this.finishInputRecording()
                lifecycle.inputReplay = inputReplay
            } catch (error) {
                stopError = error
            }
        }

        if (beforeStop != null && stopError == null) {
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

        if (this.recordingOnGameStart && this.autoDownloadRecordedVideo) {
            const fileName = `spx_${new Date().getTime()}.webm`;
            this.downloadRecordedVideo(fileName)
        }
        if (stopError != null) throw stopError
        return { inputReplay, stopped: true }
    }

    beginGameLifecycle(inputSession) {
        if (this.gameLifecycle != null && !this.gameLifecycle.completed) {
            throw new Error('Game is already running')
        }
        let resolveExit
        const exit = new Promise((resolve) => {
            resolveExit = resolve
        })
        const lifecycle = {
            completed: false,
            inputSession,
            inputReplay: null,
            exit,
            resolveExit,
        }
        this.gameLifecycle = lifecycle
        return lifecycle
    }

    completeGameLifecycle(code, lifecycle = this.gameLifecycle) {
        if (lifecycle == null || lifecycle.completed) return false
        lifecycle.completed = true
        lifecycle.resolveExit(code)
        return true
    }

    restart() {
        this.callRuntime('_gdspx_ext_request_restart')
    }

    callRuntime(name) {
        const fn = this.engine.rtenv[name]
        if (fn != null) return fn()
    }

    async loadLogicWasm(logicReady) {
        const url = this.wasmURL('ispx.wasm')
        this.go = new Go();
        if (this.minigameMode) {
            const wasmResult = await WebAssembly.instantiate(url, this.go.importObject);
            // Adapt the mini-engine instance.
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

    runLogicWasm() {
        this.go.exit = (code) => {
            this.notifyExit(code);
        };
        // go.run resolves when the runtime exits.
        this.go.run(this.logicWasmInstance);
    }

    wasmURL(name) {
        const url = this.assetURLs[name]
        return isWasmCompressed ? url + '.br' : url
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

    normalizeStartInput(options) {
        if (options == null) options = {}
        if (typeof options !== 'object' || Array.isArray(options)) {
            throw new TypeError('StartGame options must be an object')
        }
        return this.normalizeInputSession(options.input)
    }

    normalizeInputSession(input) {
        if (input == null) return null
        this.ensureInputSessionSupported()
        if (typeof input !== 'object' || Array.isArray(input)) {
            throw new TypeError('Input session must be an object')
        }
        if (input.mode === 'record') {
            // Default for hosts that bypass the runner.
            const fps = input.fps == null ? 30 : input.fps
            if (!Number.isFinite(fps) || fps <= 0) {
                throw new RangeError('Input recording FPS must be a positive finite number')
            }
            return { mode: 'record', fps, captureKey: this.normalizeCaptureKey(input.captureKey) }
        }
        if (input.mode === 'replay') {
            if (input.data == null) {
                throw new TypeError('Input replay data is required')
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
                throw new TypeError('Input replay data must be a string, ArrayBuffer, or ArrayBuffer view')
            }
            return { mode: 'replay', data, captureKey: this.normalizeCaptureKey(input.captureKey) }
        }
        throw new Error(`Unsupported input session mode: ${input.mode}`)
    }

    normalizeCaptureKey(value) {
        if (value == null) return null
        if (typeof value !== 'string' || value.length === 0) {
            throw new TypeError('Input session captureKey must be a non-empty string')
        }
        return value
    }

    ensureInputSessionSupported() {
        if (!this.normalMode) {
            throw new Error('Input recording and replay require normal Web mode')
        }
    }

    callInputFunction(name, ...args) {
        this.ensureInputSessionSupported()
        const fn = window[name]
        if (typeof fn !== 'function') {
            throw new Error(`${name} is unavailable`)
        }
        const result = fn(...args)
        if (result instanceof Error) throw result
        return result
    }

    finishInputRecording() {
        return this.callInputFunction('ispx_input_recording_finish')
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
                throw new Error(status.error || 'Input session startup aborted')
            }
            if (status.mode === 'idle') {
                throw new Error('No input session is attached to the game')
            }
            if (Date.now() - startedAt >= timeoutMs) {
                throw new Error(`Input session did not start within ${timeoutMs} ms`)
            }
            await new Promise((resolve) => setTimeout(resolve, 10))
        }
    }

    onProgress(value) {
        if (this.config.onProgress != null) {
            this.config.onProgress(value);
        }
    }

    logVerbose(...args) {
        if (this.logLevel == LOG_LEVEL_VERBOSE) {
            console.log(...args);
        }
    }
}

globalThis.GameRunner = GameRunner;
