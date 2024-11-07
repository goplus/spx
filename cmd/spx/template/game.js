
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
    }

    StartProject() {
        this.isEditor = true
        if (this.editor != null) {
            return new Promise((resolve, reject) => {
                let error = new Error("project already loaded!");
                console.log(error)
                reject(error)
            });
        }
        let promise = new Promise((resolve, reject) => {
            this.startProjectResolve = resolve
            this.startProject();
        });
        this.startProjectPromise = promise
        return promise
    }

    async UpdateProject(newData, addInfos, deleteInfos, updateInfos) {
        if (this.startProjectPromise != null) {
            await this.startProjectPromise
        }
        if (this.updateProjectPromise != null) {
            await this.updateProjectPromise
        }
        let promise = new Promise(async (resolve, reject) => {
            this.updateProjectResolve = resolve
            this.updateProject(newData, addInfos, deleteInfos, updateInfos)
        })
        this.updateProjectPromise = promise
        return promise
    }

    async RunGame() {
        this.isEditor = false
        if (this.startProjectPromise != null) {
            await this.startProjectPromise
        }
        if (this.updateProjectPromise != null) {
            await this.updateProjectPromise
        }
        if (this.runGamePromise != null) {
            return this.runGamePromise
        }
        let promise = new Promise(async (resolve, reject) => {
            this.runGameResolve = resolve
            this.runGameReject = reject
            this.runGame()
        })
        this.runGamePromise = promise
        return promise
    }

    async StopGame() {
        this.isEditor = true
        return new Promise((resolve) => {
            if (this.game != null) {
                this.game.requestQuit()
                if (this.runGameReject != null) {
                    this.runGameReject()
                    this.runGameResolve = null
                    this.runGamePromise = null
                    this.runGameReject = null
                }
            }
            resolve();
            this.onProgress(1.0);
        });
    }

    async updateProject(newData, addInfos, deleteInfos, updateInfos) {
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
        await this.addOrUpdateFiles(mergedArray, newData);
        this.deleteFiles(deleteInfos);
        if (this.updateProjectResolve != null) {
            this.updateProjectResolve()
            this.updateProjectResolve = null
            this.updateProjectPromise = null
        }
    }

    async addOrUpdateFiles(paths, zipData) {
        const zip = new JSZip();
        const zipContent = await zip.loadAsync(zipData);
        let datas = []
        for (let path of paths) {
            const dstFile = zipContent.files[path];
            let data = await dstFile.async('arraybuffer');
            if (!dstFile.dir) {
                datas.push({ "path": path, "data": data })
            }
        }
        const evt = new CustomEvent('add_files', {
            detail: datas
        });
        this.editorCanvas.dispatchEvent(evt);
    }

    deleteFiles(paths) {
        paths = paths.map(info => `res://${info}`);
        const evt = new CustomEvent('delete_files', {
            detail: paths
        });
        this.editorCanvas.dispatchEvent(evt);
    }

    refresh_fs() {
        const evt = new CustomEvent('refresh_fs', {
            detail: ""
        });
        this.editorCanvas.dispatchEvent(evt);
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

    onProgress(value) {
        if (this.appConfig.onProgress != null) {
            this.appConfig.onProgress(value);
        }
    }
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

    async startProject() {
        try {
            this.onProgress(0.1);
            this.editor = new Engine(this.editorConfig);
            this.clearPersistence(this.tempZipPath);
            let dbExists = await this.checkDBExist(this.persistentPath, this.getInstallPath());
            console.log(this.getInstallPath(), " DBExist result= ", dbExists, "this.isEditor", this.isEditor);
            if (!dbExists) {
                // install project
                // TODO store project zip and hash to indexDB 
                this.exitFunc = this.runEditor.bind(this);
                this.editor.init(this.basePath).then(async () => {
                    await this.mergeProjectWithEngineRes()
                    this.editor.copyToFS(this.tempZipPath, this.projectData);
                    const args = ['--project-manager', '--single-window', "--install_project_name", this.projectInstallName];
                    this.editor.start({ 'args': args, 'persistentDrops': true });
                });
            } else {
                this.runEditor(this.basePath)
            }
        } catch (error) {
            console.error("Error checking database existence: ", error);
        }
    }

    runEditor(basePath) {
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
                if (this.startProjectResolve != null) {
                    this.startProjectResolve()
                    this.startProjectResolve = null
                    this.startProjectPromise = null
                }
            });
        });
    }

    async runGame() {
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
                this.game = null
                console.log("on game quit")
            },
        };
        console.log("RunGame ", args);
        if (this.game) {
            console.error('A game is already running. Close it first');
            return;
        }
        this.onProgress(0.5);
        this.exitFunc = null;
        this.game = new Engine(gameConfig);
        this.game.init(this.basePath).then(() => {
            this.onProgress(0.7);
            this.game.start({ 'args': args, 'canvas': this.gameCanvas }).then(async () => {
                this.gameCanvas.focus();
                this.onProgress(0.9);
                window.goLoadData(new Uint8Array(this.projectData));
                this.onProgress(1.0);
                if (this.runGameResolve != null) {
                    this.runGameResolve()
                    this.runGameResolve = null
                    this.runGamePromise = null
                    this.runGameReject = null
                }
            });
        });
    }
}
