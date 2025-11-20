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
            'onExit': () => {
                this.onGameExit()
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

        this.useAssetCache = config.useAssetCache || this.miniprogramMode;
        profiler.enabled = this.useProfiler;

        // init worker message manager
        this.workerMessageManager = new globalThis.WorkerMessageManager();

        // init storage manager
        this.storageManager = new StorageManager({
            webPersistentPath: '/home/web_user',
            projectInstallName: config.projectName || "Game",
            useAssetCache: true,
            assetURLs: this.assetURLs,
            logVerbose: this.logVerbose.bind(this)
        });

        this.initGameTask = 0;
        this.buildGameTask = 0;
        this.runGameTask = 0;
        this.stopGameTask = 0;  
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

    startTask(prepareFunc, taskFunc, ...args) {
        if (prepareFunc) prepareFunc()
        this.logicPromise = this.logicPromise.then(() => taskFunc.call(this, ...args))
        return this.logicPromise
    }


    async InitEngine() {
        return this.startTask(() => { this.initGameTask++ }, this.initEngine)
    }

    /**
     * Initialize game with given game files. It is expected to be called after `InitEngine`, while before `StartGame`.
     * @param {Files} files 
     * @returns Promise<void>
     */
    async InitGame(files) {
        return this.startTask(() => { this.buildGameTask++ }, this.initGame, files)
    }

    async StopGame() {
        return this.startTask(() => { this.stopGameTask++ }, this.stopGame)
    }

    async StartGame() {
        return this.startTask(() => { this.runGameTask++ }, this.startGame)
    }

    async ResetGame() {
        return this.startTask(() => { this.stopGameTask++ }, this.reset)
    }

    async initEngine() {
        await profiler.profile('onRunPrepareEngineWasm', () => this.onRunPrepareEngineWasm());

        this.initGameTask--;
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
        window.gdspx_on_engine_start = function () { }
        window.gdspx_on_engine_update = function () { }
        window.gdspx_on_engine_fixed_update = function () { }
        window.goWasmInit = function () { }
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
        this.logVerbose("==> game start done");
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
        this.buildGameTask--;
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
        window.ixgo_build(nonAssetFiles);
    }

    async startGame() {
        this.runGameTask--;
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game");
            return;
        }

        let curGame = this.game;
        profiler.mark('RunGame Start');
        await profiler.profile('restart', () => this.restart());
        await profiler.profile('onRunAfterStart', () => this.onRunAfterStart(curGame));
        this.gameCanvas.focus();
        profiler.mark('RunGame Done');
        profiler.measure('RunGame Start', 'RunGame Done');
    }

    async stopGame() {
        this.stopGameTask--
        if (this.game == null) {
            // no game is running, do nothing
            this.logVerbose("no game is running")
            return
        }
        this.stopGameResolve = () => {
            this.game = null
            this.stopGameResolve = null
        }
        this.onProgress(1.0);
        this.game.requestQuit()

        if(this.recordingOnGameStart && this.autoDownloadRecordedVideo){
            let fileName = `spx_${new Date().getTime()}.webm`;
            this.downloadRecordedVideo(fileName)
        } 
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

    onGameExit() {
        this.game = null
        this.logVerbose("on game quit")
        if (this.stopGameResolve) {
            this.stopGameResolve()
        }
    }
    async reset() {
        this.stopGameTask--
        if (this.game == null) {
            this.logVerbose("no game is running")
            return
        }
        let funPtr = this.game.rtenv["_gdspx_ext_request_reset"]
        if(funPtr != null){
            funPtr()
        }
    }
    restart() {
        let funPtr = this.game.rtenv["_gdspx_ext_request_restart"]
        if(funPtr != null){
            funPtr()
        }
    }

    pause() {
        let funPtr = this.game.rtenv["_gdspx_ext_pause"]
        if(funPtr != null){
            funPtr()
        }
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


    //------------------ onRun ------------------
    async onRunPrepareEngineWasm() {
        let url = this.assetURLs["engine.wasm"]
        if (isWasmCompressed) {
            url += ".br"
        }

        if (this.minigameMode) {
            this.gameConfig.wasmEngine = url
        } else {
            if (this.useAssetCache) {
                const engineCacheResult = await this.storageManager.checkEngineCache(GetEngineHashes());
                this.gameConfig.wasmGdspx = engineCacheResult.wasmGdspx;
                this.gameConfig.wasmEngine = engineCacheResult.wasmEngine;
            } else {
                if (!this.gameConfig.wasmEngine) {
                    this.gameConfig.wasmEngine = await (await fetch(url)).arrayBuffer();
                }
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

    async onRunAfterStart(game) {
        if (this.minigameMode) {
            FFI = self;
            await this.runLogicWasm()
        }
        if (this.workerMode) {
            let pthreads = game.getPThread()
            this.workerMessageManager.setPThreads(pthreads)
            this.workerMessageManager.callWorkerProjectDataUpdate(this.projectData, this.assetURLs)
        } else {
            // register global functions
            Module = game.rtenv;
            FFI = self;
            window.ixgo_run();
        }
    }

    //------------------ logic wasm ------------------
    async loadLogicWasm() {
        // load wasm
        let url = this.config.assetURLs["gdspx.wasm"];
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
            if (this.useAssetCache) {
                const { instance } = await WebAssembly.instantiate(this.gameConfig.wasmGdspx, this.go.importObject);
                this.logicWasmInstance = instance;
            } else {
                const { instance } = await WebAssembly.instantiateStreaming(fetch(url), this.go.importObject);
                this.logicWasmInstance = instance;
            }
        }
    }
    async runLogicWasm() {
        this.go.run(this.logicWasmInstance);
    }
}

// export GameApp to global
globalThis.GameApp = GameApp;
