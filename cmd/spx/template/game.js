
class GameApp {
    constructor(config) {
        this.appConfig = config || null;
        this.editor = null;
        this.game = null;
        this.persistentPath = '/home/web_user';
        this.tempZipPath = '/tmp/preload.zip';
        this.projectInstallName = config?.projectName || "Game";
        this.projectData = config.projectData;
        this.oldData = config.projectData;
        this.persistentPaths = [this.persistentPath];
        this.gameCanvas = config.gameCanvas;
        this.editorCanvas = config.editorCanvas || config.gameCanvas;
        this.exitFunc = null;
        this.basePath = 'godot.editor'
        this.isEditor = true;
        this.editorConfig = {
            'unloadAfterInit': false,
            'canvas': this.editorCanvas,
            'canvasResizePolicy': 0,
            'persistentPaths': this.persistentPaths,
            'onExecute': (args) => {
                console.log("onExecute  ", args);
            },
            'onExit': () => {
                if (this.exitFunc) {
                    this.exitFunc();
                }
            }
        };
        this.logicPromise = Promise.resolve();
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
        try {
            this.onProgress(0.1);
            this.editor = new Engine(this.editorConfig);
            this.clearPersistence(this.tempZipPath);
            // TODO get the old project's zip and compare the hash
            let dbExists = await this.checkDBExist(this.persistentPath, this.getInstallPath());
            console.log(this.getInstallPath(), " DBExist result= ", dbExists, "this.isEditor", this.isEditor);
            if (!dbExists) {
                // TODO store project zip and hash to indexDB 
                this.exitFunc = () => {
                    this.runEditor(resolve, reject)
                };
                // install project
                this.editor.init(this.basePath).then(async () => {
                    await this.mergeProjectWithEngineRes()
                    this.editor.copyToFS(this.tempZipPath, this.projectData);
                    const args = ['--project-manager', '--single-window', "--install_project_name", this.projectInstallName];
                    this.editor.start({ 'args': args, 'persistentDrops': true });
                });
            } else {
                this.runEditor(resolve, reject, this.basePath)
            }
        } catch (error) {
            console.error("Error checking database existence: ", error);
        }
    }

    async updateProject(resolve, reject, newData, addInfos, deleteInfos, updateInfos) {
        if (addInfos == null) {
            addInfos = []
        }
        if (deleteInfos == null) {
            deleteInfos = []
        }
        if (updateInfos == null) {
            updateInfos = []
        }
        this.oldData = newData
        let mergedArray = addInfos.concat(updateInfos);
        const zip = new JSZip();
        const zipContent = await zip.loadAsync(newData);
        let datas = []
        for (let path of mergedArray) {
            const dstFile = zipContent.files[path];
            let data = await dstFile.async('arraybuffer');
            if (!dstFile.dir) {
                datas.push({ "path": path, "data": data })
            }
        }
        deleteInfos = deleteInfos.map(info => `res://${info}`);
        const evt = new CustomEvent('update_project', {
            detail: {
                "resolve": resolve,
                "dirtyInfos": datas,
                "deleteInfos": deleteInfos,
            }
        });
        this.editorCanvas.dispatchEvent(evt);
    }

    runEditor(resolve, reject, basePath) {
        let args = [
            "--path",
            this.getInstallPath(),
            "--single-window",
            "--editor",
        ];
        this.exitFunc = null;
        console.log("runEditor ", args);
        this.editor.init(basePath).then(() => {
            this.onProgress(0.4);
            this.editor.start({ 'args': args, 'persistentDrops': false, 'canvas': this.editorCanvas }).then(async () => {
                this.editorCanvas.focus();
                this.onProgress(0.9);
                await this.mergeProjectWithEngineRes()
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                resolve()
            });
        });
    }

    async runGame(resolve, reject) {
        this.runGameTask--
        // if stopGame is called before runing game, then do nothing
        if (this.stopGameTask > 0) {
            console.log("stopGame is called before runing game")
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
        const gameConfig = {
            'persistentPaths': this.persistentPaths,
            'unloadAfterInit': false,
            'canvas': this.gameCanvas,
            'canvasResizePolicy': 1,
            'onExit': () => {
                this.onGameExit()
            },
        };
        console.log("RunGame ", args);
        if (this.game) {
            reject(new Error('A game is already running. Close it first'));
            return;
        }
        this.onProgress(0.5);
        this.exitFunc = null;
        this.game = new Engine(gameConfig);
        let curGame = this.game
        curGame.init(this.basePath).then(() => {
            this.onProgress(0.7);
            curGame.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                this.gameCanvas.focus();
                this.onProgress(0.9);
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                resolve()
            });
        });
    }


    async stopGame(resolve, reject) {
        this.stopGameTask--
        if (this.game == null) {
            // no game is running, do nothing
            resolve()
            console.log("no game is running")
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
        console.log("on game quit")
        if (this.stopGameResolve) {
            this.stopGameResolve()
        }
    }

    //------------------ update project ------------------
    async addOrUpdateFiles(resolve, reject, paths, zipData) {

        const evt = new CustomEvent('add_files', {
            detail: datas
        });
        this.editorCanvas.dispatchEvent(evt);
        const handler = () => {
            callback();
            this.editorCanvas.removeEventListener('update_files_handled', handler);
        };
        this.editorCanvas.addEventListener('update_files_handled', handler);
    }

    deleteFiles(paths) {
        paths = paths.map(info => `res://${info}`);
        const evt = new CustomEvent('delete_files', {
            detail: paths
        });
        this.editorCanvas.dispatchEvent(evt);
    }



    //------------------ install project ------------------
    getInstallPath() {
        return `${this.persistentPath}/${this.projectInstallName}`;
    }

    clearPersistence(targetPath) {
        const req = indexedDB.deleteDatabase(targetPath);
        req.onerror = (err) => {
            alert('Error deleting local files. Please retry after reloading the page.');
        };
        console.log("clear persistence cache", targetPath);
    }


    checkKeyExists(dbName, storeName, key) {
        return new Promise((resolve, reject) => {
            let request = indexedDB.open(dbName);
            request.onupgradeneeded = function (event) {
                let db = event.target.result;
                if (!db.objectStoreNames.contains(storeName)) {
                    db.createObjectStore(storeName);
                }
            };

            request.onsuccess = function (event) {
                let db = event.target.result;
                if (!db.objectStoreNames.contains(storeName)) {
                    reject(`Object store "${storeName}" not found`);
                    db.close();
                    return;
                }
                let transaction = db.transaction(storeName, 'readonly');
                let objectStore = transaction.objectStore(storeName);
                let getRequest = objectStore.getKey(key);
                getRequest.onsuccess = function () {
                    if (getRequest.result !== undefined) {
                        resolve(true);
                    } else {
                        resolve(false);
                    }
                };

                getRequest.onerror = function () {
                    reject('Error checking key existence', dbName);
                };

                transaction.oncomplete = function () {
                    db.close();
                };
            };

            request.onerror = function (event) {
                reject('Error opening database: ' + dbName + " " + storeName + " " + event.target.error);
            };
        });
    }

    async checkDBExist(dbName, storeName) {
        try {
            let exists = await this.checkKeyExists(dbName, 'FILE_DATA', storeName);
            return exists;
        } catch (error) {
            console.error(error);
            return false;
        }
    }

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
        console.log("merge zip files");
        const engineDataResp = fetch("engineres.zip");
        let engineData = await (await engineDataResp).arrayBuffer();
        this.projectData = await this.mergeZips(this.projectData, engineData);
        this.hasMergedProject = true
    }

    //------------------ misc ------------------
    onProgress(value) {
        if (this.appConfig.onProgress != null) {
            this.appConfig.onProgress(value);
        }
    }
}
