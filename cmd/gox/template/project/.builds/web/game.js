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
        // web worker mode
        this.workerMode = EnginePackMode == "worker"
        this.minigameMode = EnginePackMode == "minigame"
        this.normalMode = !this.workerMode && !this.minigameMode

        this.pthreads = null;
        this.workerMessageId = 0;
        if (this.workerMode) {
            this.bindMainCallHandler()
        }
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
        let url = this.assetURLs["engine.wasm"]
        if (isWasmCompressed) {
            url += ".br"
        }
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
        let funcMap = null;
        if (this.minigameMode) {
            GameGlobal.engine = this.game;
            godotSdk.set_engine(this.game);
            funcMap = globalThis
            self.initExtensionWasm = function () { }
        } else {
            if (!this.workerMode) {
                await this.loadLogicWasm()
                await this.runLogicWasm()
                self.initExtensionWasm = function () { }
            }
            funcMap = window
        }
        const spxfuncs = new GdspxFuncs();
        const methodNames = Object.getOwnPropertyNames(Object.getPrototypeOf(spxfuncs));
        methodNames.forEach(key => {
            if (key.startsWith('gdspx_') && typeof spxfuncs[key] === 'function') {
                funcMap[key] = spxfuncs[key].bind(spxfuncs);
            }
        });

        curGame.init().then(async () => {
            this.onProgress(0.6);
            if (this.workerMode) {
                this.bindMainThreadCallbacks(curGame)
            }
            await this.unpackGameData(curGame)
            this.onProgress(0.7);
            if (this.minigameMode) {
                await this.loadLogicWasm()
            }
            this.onProgress(0.80);
            curGame.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                if (this.minigameMode) {
                    FFI = self;
                    await this.runLogicWasm()
                }

                this.onProgress(0.9);
                this.gameCanvas.focus();
                if (this.workerMode) {
                    this.pthreads = curGame.getPThread()
                    this.callWorkerProjectDataUpdate(this.projectData)
                } else {
                    // register global functions
                    Module = curGame.rtenv;
                    FFI = self;
                    window.goLoadData(new Uint8Array(this.projectData));
                }
                this.onProgress(1.0);
                this.gameCanvas.focus();
                this.logVerbose("==> game start done")
                resolve()
            });
        });
    }

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
            const { instance } = await WebAssembly.instantiateStreaming(fetch(url), this.go.importObject);
            this.logicWasmInstance = instance;
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
        this.pthreads = null
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

    // === PThread Worker message sending related methods ===
    bindMainThreadCallbacks(game) {
        game.rtenv["_spxOnMainCall"] = window._spxOnMainCall
    }

    bindMainCallHandler() {
        window._spxMainCalls = {}
        window._spxOnMainCall = function (...params) {
            let funcName = params[0]
            let args = params.slice(1)
            if (window._spxMainCalls.hasOwnProperty(funcName)) {
                let callback = window._spxMainCalls[funcName]
                if (callback != null) {
                    callback(...args)
                }
            } else {
                let func = window[funcName]
                if (func != null) {
                    func(...args)
                } else {
                    console.error("no such function: ", funcName)
                }
            }
        }
    }
    callWorkerProjectDataUpdate(projectData) {
        const message = {
            cmd: 'projectDataUpdate',
            data: projectData,
            timestamp: Date.now()
        };
        return this.postMessageToWorkers(message);
    }

    callWorkerFunction(funcName, ...args) {
        // auto process arguments, convert function to main thread callback
        const processedArgs = this.processArguments(...args);

        const message = {
            cmd: 'customCall',
            data: {
                funcName: funcName,
                args: processedArgs
            },
            timestamp: Date.now()
        };
        return this.postMessageToWorkers(message);
    }

    // process arguments, auto convert function to main thread callback
    processArguments(...args) {

        const processedArgs = [];
        let callbackCounter = 0;

        for (let arg of args) {
            if (typeof arg === 'function') {
                // generate unique callback name
                const callbackName = `_onSpxCall_${Date.now()}_${callbackCounter++}`;

                // register callback function
                this.registerWorkerCallback(callbackName, arg);

                // replace with main thread callback identifier
                processedArgs.push("_SPX_CALLBACK_FUNC_", callbackName);
            } else {
                processedArgs.push(arg);
            }
        }

        return processedArgs;
    }

    // register worker callback function
    registerWorkerCallback(callbackName, userFunction) {
        // create callback handler function
        window._spxMainCalls[callbackName] = async function (requestId, ...args) {
            let errorMsg = null;
            let result = null;

            try {
                if (userFunction) {
                    result = userFunction(...args);
                    // if return Promise, wait for it to complete
                    if (result && typeof result.then === 'function') {
                        result = await result;
                    }
                } else {
                    errorMsg = `No function registered for ${callbackName}`;
                }
            } catch (error) {
                console.error(`Error in ${callbackName}:`, error);
                errorMsg = error.message;
            }

            // send response to worker
            this.postMessageToWorkers({
                cmd: 'callResponse',
                responseId: requestId,
                result: result,
                error: errorMsg
            });
        }.bind(this);
    }

    postMessageToWorkers(message, transferList = null, cloneForEach = false) {
        const workers = [];
        if (this.pthreads) {
            workers.push(...this.pthreads.runningWorkers);
        }

        let successCount = 0;
        let errorCount = 0;

        workers.forEach((worker, index) => {
            try {
                if (worker && typeof worker.postMessage === 'function') {
                    // Adds unique identifier and target info to each message
                    let enhancedMessage = {
                        ...message,
                        _gameAppMessageId: ++this.workerMessageId,
                        _targetWorkerIndex: index,
                        _timestamp: Date.now()
                    };

                    // Special handling required when cloning data or using transferList
                    if (transferList && cloneForEach) {
                        if (message.data && message.data.buffer) {
                            const clonedData = new Uint8Array(message.data);
                            enhancedMessage.data = clonedData;
                            worker.postMessage(enhancedMessage, [clonedData.buffer]);
                        } else {
                            worker.postMessage(enhancedMessage);
                        }
                    } else {
                        worker.postMessage(enhancedMessage);
                    }

                    successCount++;
                } else {
                    console.warn(`Worker ${index} is invalid or does not have postMessage method`);
                    errorCount++;
                }
            } catch (error) {
                console.error(`Failed to send message to worker ${index}:`, error);
                errorCount++;
            }
        });

        return { successCount, errorCount, totalWorkers: workers.length };
    }
}

// export GameApp to global
globalThis.GameApp = GameApp;