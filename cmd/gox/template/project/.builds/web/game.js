

class GameApp {
    constructor(config) {
        config = config || {};
        this.config = config;
        this.editor = null;
        this.game = null;
        this.persistentPath = '/home/web_user';
        this.tempZipPath = '/tmp/preload.zip';
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
        this.editorConfig = {
            "executable": "godot.editor",
            'unloadAfterInit': false,
            'canvas': this.editorCanvas,
            'canvasResizePolicy': 0,
            "logLevel": this.logLevel,
            'persistentPaths': this.persistentPaths,
            'onExecute': (args) => {
                this.logVerbose("onExecute  ", args);
            },
            'onExit': () => {
                if (this.exitFunc) {
                    this.exitFunc();
                }
            }
        };
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

    async StartProject() {
        return this.startTask(null, this.startProject)
    }

    async UpdateProject(newData, addInfos, deleteInfos, updateInfos) {
        return this.startTask(null, this.updateProject, newData, addInfos, deleteInfos, updateInfos)
    }

    async StopProject() {
        return this.startTask(null, this.stopProject)
    }

    async RunGame() {
        return this.startTask(() => { this.runGameTask++ }, this.runGame)
    }

    async StopGame() {
        return this.startTask(() => { this.stopGameTask++ }, this.stopGame)
    }

    async startProject(resolve, reject) {
        if (this.editor != null) {
            console.error("project already loaded!")
        }
        this.isEditor = true

        let url = this.assetURLs["engineres.zip"]
        let engineData = await (await fetch(url)).arrayBuffer();

        try {
            this.onProgress(0.1);
            this.clearPersistence(this.tempZipPath);
            let isCacheValid = await this.checkAndUpdateCache(engineData, true);
            await this.checkEngineCache()
            this.editor = new Engine(this.editorConfig);
            if (!isCacheValid) {
                this.exitFunc = () => {
                    this.exitFunc = null
                    this.editor = new Engine(this.editorConfig);
                    this.runEditor(resolve, reject)
                };
                // install project
                this.editor.init().then(async () => {
                    this.writePersistence(this.editor, this.tempZipPath, engineData);
                    const args = ['--project-manager', '--single-window', "--install_project_name", this.projectInstallName];
                    this.editor.start({
                        'args': args, 'persistentDrops': true,
                        "logLevel": this.logLevel
                    }).then(async () => {
                        this.editorCanvas.focus();
                    })
                });
            } else {
                this.logVerbose("cache is valid, skip it")
                resolve()
            }
        } catch (error) {
            console.error("Error checking database existence: ", error);
        }
    }

    async updateProject(resolve, reject, newData, addInfos, deleteInfos, updateInfos) {
        this.projectData = newData
        resolve()
    }

    async stopProject(resolve, reject) {
        if (this.editor == null) {
            resolve()
            return
        }
        this.stopGameTask++
        await this.stopGame(() => {
            this.isEditor = true
            this.onProgress(1.0);
            this.editor.requestQuit()
            this.logVerbose("on editor quit")
            this.editor = null
            this.exitFunc = null
            resolve();
        }, null)
    }

    runEditor(resolve, reject) {
        let args = [
            "--path",
            this.getInstallPath(),
            "--single-window",
            "--editor",
        ];
        this.exitFunc = null;
        this.logVerbose("runEditor ", args);
        this.onProgress(0.2);
        this.editor.init().then(() => {
            this.onProgress(0.4);
            this.editor.start({
                'args': args, 'persistentDrops': false,
                'canvas': this.editorCanvas,
                "logLevel": this.logLevel
            }).then(async () => {
                this.editorCanvas.focus();
                await this.waitFsSyncDone(this.editorCanvas)
                this.onProgress(0.9);
                await this.mergeProjectWithEngineRes()
                this.onProgress(1.0);
                await this.updateProjectHash(this.curProjectHash)
                this.logVerbose("==> editor start done")
                resolve()
            });
        });
    }

    async runGame(resolve, reject) {
        this.runGameTask--
        // if stopGame is called before runing game, then do nothing
        if (this.stopGameTask > 0) {
            this.logVerbose("stopGame is called before runing game")
            resolve()
            return
        }

        this.isEditor = false
        const args = [
            "--path",
            this.getInstallPath(),
            "--editor-pid",
            "0",
            "res://main.tscn",
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
        curGame.init().then(async () => {
            this.onProgress(0.7);
            await this.unpackGameData(curGame)

            curGame.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                this.gameCanvas.focus();
                await this.waitFsSyncDone(this.gameCanvas)
                this.onProgress(0.9);
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                this.logVerbose("==> game start done")
                resolve()
            });
        });
    }

    async unpackGameData(curGame) {
        const zip1 = new JSZip();
        const zip1Content = await zip1.loadAsync(this.projectData);
        let datas = []
        for (const [filePath, file] of Object.entries(zip1Content.files)) {
            const content = await file.async('arraybuffer');
            if (!file.dir) {
                datas.push({ "path": filePath, "data": content })
            }
        }
        curGame.unpackGameData(this.tempGamePath, datas)
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

    //------------------ update project ------------------
    async waitFsSyncDone(canvas) {
        return new Promise((resolve, reject) => {
            this.logVerbose("waitFsSyncDone start")
            const evt = new CustomEvent('spx_wait_fs_sync_done', {
                detail: {
                    "resolve": async () => {
                        this.logVerbose("waitFsSyncDone done")
                        resolve()
                    },
                }
            });
            canvas.dispatchEvent(evt);
        })
    }

    //------------------ install project ------------------
    getInstallPath() {
        return `${this.persistentPath}/${this.projectInstallName}`;
    }

    writePersistence(engine, targetPath, value) {
        if (engine == null) {
            console.error("please init egnine first!")
            return
        }
        engine.copyToFS(targetPath, value);
    }
    clearPersistence(targetPath) {
        const req = indexedDB.deleteDatabase(targetPath);
        req.onerror = (err) => {
            alert('Error deleting local files. Please retry after reloading the page.');
        };
        this.logVerbose("clear persistence cache", targetPath);
    }

    getObjectStore(dbName, storeName, mode, storeKeyPath) {
        return new Promise((resolve, reject) => {
            let request = indexedDB.open(dbName);

            request.onupgradeneeded = function (event) {
                let db = event.target.result;
                if (!db.objectStoreNames.contains(storeName)) {
                    if (storeKeyPath) {
                        db.createObjectStore(storeName, { keyPath: storeKeyPath });
                    } else {
                        db.createObjectStore(storeName);
                    }

                }
            };

            request.onsuccess = function (event) {
                let db = event.target.result;
                if (!db.objectStoreNames.contains(storeName)) {
                    reject(`Object store "${storeName}" not found`);
                    db.close();
                    return;
                }

                let transaction = db.transaction(storeName, mode);
                let objectStore = transaction.objectStore(storeName);
                resolve({ db, objectStore, transaction });
            };

            request.onerror = function (event) {
                reject('Error opening database: ' + dbName + " " + storeName + " " + event.target.error);
            };

            request.onblocked = function (event) {
                reject('Database is blocked. Please close other tabs or windows using this database. ', dbName + " " + storeName + " " + event.target.error);
            }
        });
    }

    queryIndexDB(dbName, storeName, key) {
        return this.getObjectStore(dbName, storeName, 'readonly').then(({ db, objectStore, transaction }) => {
            return new Promise((resolve, reject) => {
                let getRequest = objectStore.get(key);

                getRequest.onsuccess = function () {
                    resolve(getRequest.result);
                };

                getRequest.onerror = function () {
                    reject('Error checking key existence');
                };

                transaction.oncomplete = function () {
                    db.close();
                };
            });
        });
    }

    updateIndexDB(dbName, storeName, key, value) {
        return this.getObjectStore(dbName, storeName, 'readwrite', key).then(({ db, objectStore, transaction }) => {
            return new Promise((resolve, reject) => {
                let putRequest = objectStore.put(value, key);

                putRequest.onsuccess = function () {
                    resolve('Value successfully written to the database');
                };

                putRequest.onerror = function () {
                    reject('Error writing value to the database');
                };

                transaction.oncomplete = function () {
                    db.close();
                };
            });
        });
    }
    async getCache(storeName) {
        try {
            let cacheValue = await this.queryIndexDB(this.persistentPath, 'FILE_DATA', storeName);
            return cacheValue;
        } catch (error) {
            console.error(error);
            return undefined;
        }
    }

    async setCache(storeName, value) {
        try {
            let cacheValue = await this.updateIndexDB(this.persistentPath, 'FILE_DATA', storeName, value);
            return cacheValue;
        } catch (error) {
            console.error(error);
            return undefined;
        }
    }

    async computeHash(data) {
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(byte => byte.toString(16).padStart(2, '0')).join('');
    }
    getProjectDataKey() {
        return `${this.persistentPath}/.spx_cache_data/${this.projectInstallName}`
    }
    getProjectHashKey() {
        return `${this.persistentPath}/.spx_cache_hash/${this.projectInstallName}`
    }

    async updateProjectHash(hash) {
        this.logVerbose("updateProjectHash ", hash)
        await this.setCache(this.getProjectHashKey(), hash);
    }
    async checkAndUpdateCache(curData, isClearIfDirty = false) {
        // TODO only cache art resources
        let curHash = await this.computeHash(curData);
        let cachedHash = await this.getCache(this.getProjectHashKey());
        this.curProjectHash = curHash
        this.logVerbose("checkAndUpdateCache ", this.getProjectHashKey(), curHash, " old_hash = ", cachedHash)
        if (cachedHash != undefined && curHash === cachedHash) {
            return true;
        }
        if (isClearIfDirty) {
            await this.updateProjectHash('')
            // clear the dirty cache
            // TOOD only clear the current project's cache data
            this.clearPersistence(this.persistentPath);
            // create a default indexDB
            await this.ensureCacheDB()
        } else {
            await this.updateProjectHash(this.curProjectHash)
        }
        // cache is dirty, update it 
        await this.setCache(this.getProjectDataKey(), curData);
        return false;
    }

    async ensureCacheDB() {
        await this.getObjectStore(this.persistentPath, 'FILE_DATA', 'readonly')
    }

    getEngineHashKey(assetName) {
        return `${this.persistentPath}/.spx_engine_hash/${assetName}`
    }
    getEngineDataKey(assetName) {
        return `${this.persistentPath}/.spx_engine_data/${assetName}`
    }
    async checkEngineCache() {
        let hashes = GetEngineHashes()
        this.logVerbose("curHashes ", hashes)
        this.wasmGdspx = await this.checkEngineCacheAsset(hashes, "gdspx.wasm");
        this.wasmEngine = await this.checkEngineCacheAsset(hashes, "godot.editor.wasm");
        this.editorConfig.wasmGdspx = this.wasmGdspx
        this.editorConfig.wasmEngine = this.wasmEngine
        this.gameConfig.wasmGdspx = this.wasmGdspx
        this.gameConfig.wasmEngine = this.wasmEngine
    }

    async checkEngineCacheAsset(hashes, assetName) {
        try {
            let url = this.assetURLs[assetName]
            if (!this.useAssetCache) {
                return await (await fetch(url)).arrayBuffer();
            }

            let curHash = hashes[assetName];
            await this.ensureCacheDB();

            const cachedHash = await this.getCache(this.getEngineHashKey(assetName));
            const isCacheValid = cachedHash !== undefined && curHash === cachedHash;

            if (!isCacheValid) {
                this.logVerbose("Download engine asset:", assetName, url);
                const curData = await (await fetch(url)).arrayBuffer();
                await this.setCache(this.getEngineDataKey(assetName), curData);
                await this.setCache(this.getEngineHashKey(assetName), curHash);

                return curData;
            } else {
                this.logVerbose("Load cached engine asset:", assetName);
                const curData = await this.getCache(this.getEngineDataKey(assetName));
                return curData;
            }
        } catch (error) {
            console.error("Error checking engine cache asset:", error);
            throw error;
        }
    }


    //------------------ res merge ------------------
    async mergeZips(zipFile1, zipFile2) {
        const zip1 = new JSZip();
        const zip2 = new JSZip();

        const zip1Content = await zip1.loadAsync(zipFile1);
        const zip2Content = await zip2.loadAsync(zipFile2);

        const newZip = new JSZip();

        for (const [filePath, file] of Object.entries(zip1Content.files)) {
            const content = await file.async('arraybuffer');
            newZip.file(filePath, content);
        }

        for (const [filePath, file] of Object.entries(zip2Content.files)) {
            const content = await file.async('arraybuffer');
            newZip.file(filePath, content);
        }

        return newZip.generateAsync({ type: 'arraybuffer' });
    }

    async mergeProjectWithEngineRes() {
        if (this.hasMergedProject) {
            return
        }
        this.logVerbose("merge zip files");
        const engineDataResp = fetch("engineres.zip");
        let engineData = await (await engineDataResp).arrayBuffer();
        this.projectData = await this.mergeZips(this.projectData, engineData);
        this.hasMergedProject = true
    }

    //------------------ misc ------------------
    onProgress(value) {
        if (this.config.onProgress != null) {
            this.config.onProgress(value);
        }
    }
}
