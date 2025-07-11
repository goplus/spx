var Module = null

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
        this.projectData = config.projectData;
        this.oldData = config.projectData;
        this.gameCanvas = config.gameCanvas;
        this.assetURLs = config.assetURLs;
        this.gameConfig = {
            "executable": "engine",
            'unloadAfterInit': false,
            'canvas': this.gameCanvas,
            'logLevel': this.logLevel,
            'canvasResizePolicy': 1,
            'onExit': () => {
                this.onGameExit()
            },
        };
        this.logicPromise = Promise.resolve();
        this.curProjectHash = ''
        // web worker mode
        this.workerMode = EnginePackMode == "worker"
        this.minigameMode = EnginePackMode == "minigame"
        this.miniprogramMode = EnginePackMode == "miniprogram"
        this.normalMode = !this.workerMode && !this.minigameMode && !this.miniprogramMode

        this.useAssetCache = config.useAssetCache || this.miniprogramMode;

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

        this.logVerbose("EnginePackMode: ", EnginePackMode)
    }
    logVerbose(...args) {
        if (this.logLevel == LOG_LEVEL_VERBOSE) {
            console.log(...args);
        }
    }
    startTask(prepareFunc, taskFunc, ...args) {
        if (prepareFunc != null) {
            prepareFunc()
        }
        this.logicPromise = this.logicPromise.then(async () => {
            let promise = new Promise(async (resolve, reject) => {
                await taskFunc.call(this, resolve, reject, ...args);
            })
            await promise
        })
        return this.logicPromise
    }

    async RunGame() {
        return this.startTask(() => { this.runGameTask++ }, this.runGame)
    }

    async StopGame() {
        return this.startTask(() => { this.stopGameTask++ }, this.stopGame)
    }


    async runGame(resolve, reject) {
        await this.onRunPrepareEngineWasm()

        this.runGameTask--
        // if stopGame is called before runing game, then do nothing
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game")
            resolve()
            return
        }

        let args = [
            '--main-pack', this.persistentPath + "/" + this.packName,
            '--main-project-data', this.persistentPath + "/" + this.projectDataName,
        ];

        this.logVerbose("RunGame ", args);
        if (this.game) {
            this.logVerbose('A game is already running. Close it first');
            resolve()
            return;
        }

        this.onProgress(0.5);
        this.game = new Engine(this.gameConfig);
        let curGame = this.game

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


        await this.onRunBeforInit()
        this.onProgress(0.5);

        curGame.init().then(async () => {
            this.onProgress(0.6);
            await this.unpackGameData(curGame)
            this.onProgress(0.7);
            await this.onRunAfterInit(curGame)
            this.onProgress(0.80);
            curGame.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                this.onProgress(0.9);
                this.gameCanvas.focus();
                await this.onRunAfterStart(curGame)
                this.onProgress(1.0);
                this.gameCanvas.focus();
                this.logVerbose("==> game start done")
                resolve()
            });
        });
    }



    async stopGame(resolve, reject) {
        this.stopGameTask--
        if (this.game == null) {
            // no game is running, do nothing
            resolve()
            this.logVerbose("no game is running")
            return
        }
        this.stopGameResolve = () => {
            this.game = null
            resolve();
            this.stopGameResolve = null
        }
        this.onProgress(1.0);
        this.game.requestQuit()
    }

    onGameExit() {
        this.game = null
        this.logVerbose("on game quit")
        if (this.stopGameResolve) {
            this.stopGameResolve()
        }
    }

    //------------------ misc ------------------
    onProgress(value) {
        if (this.config.onProgress != null) {
            this.config.onProgress(value);
        }
    }
    async unpackGameData(game) {
        let packUrl = this.assetURLs[this.packName]
        let pckData = await (await fetch(packUrl)).arrayBuffer();
        await game.unpackGameData(this.persistentPath, this.projectDataName, this.projectData.buffer, this.packName, pckData)
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

    async onRunBeforInit() {
        if (this.minigameMode) {
            GameGlobal.engine = this.game;
            godotSdk.set_engine(this.game);
            self.initExtensionWasm = function () { }
        } else {
            if (!this.workerMode) {
                await this.loadLogicWasm()
                await this.runLogicWasm()
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
            window.goLoadData(new Uint8Array(this.projectData));
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
        if (!this.minigameMode) {
            if (this.config.onSpxReady != null) {
                this.config.onSpxReady()
            }
        }
    }

}

// export GameApp to global
globalThis.GameApp = GameApp;