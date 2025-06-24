
class GameApp {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.game = null;
        this.persistentPath = '/home/web_user';
        this.tempZipPath = '/tmp/preload.zip';
        this.packName =  'godot.editor.pck';
        this.projectDataName = 'game.zip';
        this.isRuntimeMode = config.isRuntimeMode;
        this.tempGamePath = '/home/spx_game_cache';
        this.projectInstallName = config.projectName || "Game";
        this.logLevel = config.logLevel || 0;
        this.projectData = config.projectData;
        this.oldData = config.projectData;
        this.persistentPaths = [this.persistentPath];
        this.gameCanvas = config.gameCanvas;
        this.editorCanvas = config.editorCanvas || config.gameCanvas;
        this.exitFunc = null;
        this.basePath = 'godot.editor'
        this.isEditor = true;
        this.assetURLs = config.assetURLs;
        this.useAssetCache = config.useAssetCache;
        this.gameConfig = {
            "executable": "godot.editor",
            'persistentPaths': this.persistentPaths,
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
        let url = this.assetURLs["godot.editor.wasm"]
        this.wasmEngine = await (await fetch(url)).arrayBuffer(); 
        this.gameConfig.wasmEngine = this.wasmEngine

        this.runGameTask--
        // if stopGame is called before runing game, then do nothing
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game")
            resolve()
            return
        }

        this.isEditor = false
        let args = [
            '--main-pack', this.tempGamePath + "/" + this.packName,
            '--main-project-data', this.tempGamePath + "/" + this.projectDataName,
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
                this.onProgress(0.9);
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                this.gameCanvas.focus();
                this.logVerbose("==> game start done")
                resolve()
            });
        });
    }

    async unpackGameData(curGame) {
        let packUrl = this.assetURLs[this.packName]
        let pckData =  await (await fetch(packUrl)).arrayBuffer();
        await curGame.unpackGameData(this.tempGamePath, this.projectDataName, this.projectData.buffer, this.packName, pckData)
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
        this.isEditor = true
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
