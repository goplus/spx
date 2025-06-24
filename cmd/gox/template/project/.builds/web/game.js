
class GameApp {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.game = null;
        this.packName = 'engine.zip';
        this.projectDataName = 'game.zip';
        this.persistentPath = 'engine';
        this.logLevel = config.logLevel || LOG_LEVEL_NONE;
        this.projectData = config.projectData;
        this.oldData = config.projectData;
        this.gameCanvas = config.gameCanvas;
        this.assetURLs = config.assetURLs;
        this.useAssetCache = config.useAssetCache;
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
        let url = this.assetURLs["engine.wasm"]
        this.gameConfig.wasmEngine = url
        if (!miniEngine) {
            this.wasmEngine = await (await fetch(url)).arrayBuffer();
            this.gameConfig.wasmEngine = this.wasmEngine
        }

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

        if (miniEngine) {
            GameGlobal.engine = this.game;
            godotSdk.set_engine(this.game);
        }else{
            await this.loadWasm()
        }

        // register global functions
        const spxfuncs = new GdspxFuncs();
        const methodNames = Object.getOwnPropertyNames(Object.getPrototypeOf(spxfuncs));
        methodNames.forEach(key => {
            if (key.startsWith('gdspx_') && typeof spxfuncs[key] === 'function') {
                window[key] = spxfuncs[key].bind(spxfuncs);
            }
        });

        curGame.init().then(async () => {
            this.onProgress(0.7);
            await this.unpackGameData(curGame)

            curGame.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                if (miniEngine) {
                    await this.loadWasm()
                }
                this.onProgress(0.9);
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                this.gameCanvas.focus();
                this.logVerbose("==> game start done")
                resolve()
            });
        });
    }

    async loadWasm() {
        // load wasm
        let url = this.config.assetURLs["gdspx.wasm"];
        console.log("go wasm url===>", url);
        const go = new Go();
        if (miniEngine) {
            // load wasm in miniEngine
            const wasmResult = await WebAssembly.instantiate(url, go.importObject);
            // create compatible instance
            const compatibleInstance = Object.create(WebAssembly.Instance.prototype);
            compatibleInstance.exports = wasmResult.instance.exports;
            Object.defineProperty(compatibleInstance, 'constructor', {
                value: WebAssembly.Instance,
                writable: false,
                enumerable: false,
                configurable: true
            });

            console.log("[debug] go.run start");
            go.run(compatibleInstance);
            console.log("[debug] go.run end");
            return;
        } else {
            const { instance } = await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
            go.run(instance);
            if (this.config.onSpxReady != null) {
                this.config.onSpxReady()
            }
        }
    }

    async unpackGameData(curGame) {
        let packUrl = this.assetURLs[this.packName]
        let pckData = await (await fetch(packUrl)).arrayBuffer();
        await curGame.unpackGameData(this.persistentPath, this.projectDataName, this.projectData.buffer, this.packName, pckData)
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
}

// export GameApp to global
globalThis.GameApp = GameApp;