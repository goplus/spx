
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
        this.editorCanvas = config.gameCanvas;// use the same canvas
        this.exitFunc = null;
        this.basePath = 'godot.editor'
        this.isEditor = config.isEditor;
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
        this.installProject();
    }

    async UpdateProject(newData, addInfos, deleteInfos, updateInfos) {
        console.log("UpdateProject", this.oldData, newData)
        if (addInfos == undefined) {
            const diffInfos = await this.getZipDiffInfos(this.oldData, newData);
            addInfos = diffInfos.addInfos;
            deleteInfos = diffInfos.deleteInfos;
            updateInfos = diffInfos.updateInfos;
        }
        this.oldData = newData
        console.log('DiffInfos :', addInfos, deleteInfos, updateInfos);
        let mergedArray = addInfos.concat(updateInfos);
        await this.addOrUpdateFiles(mergedArray, newData);
        this.deleteFiles(deleteInfos);
    }

    StopGame() {
        if(this.game != null){
            this.game.requestQuit()
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
        this.gameCanvas.dispatchEvent(evt);
    }

    deleteFiles(paths) {
        paths = paths.map(info => `res://${info}`);
        const evt = new CustomEvent('delete_files', {
            detail: paths
        });
        this.gameCanvas.dispatchEvent(evt);
    }

    async getZipDiffInfos(srcZip, dstZip) {
        function areBuffersEqual(buffer1, buffer2) {
            if (buffer1.byteLength !== buffer2.byteLength) return false;
            const view1 = new Uint8Array(buffer1);
            const view2 = new Uint8Array(buffer2);
            for (let i = 0; i < view1.length; i++) {
                if (view1[i] !== view2[i]) return false;
            }
            return true;
        }
        function mergeDeleteInfos(deleteInfos) {
            deleteInfos.sort();
            const merged = [];

            for (let i = 0; i < deleteInfos.length; i++) {
                const currentPath = deleteInfos[i];
                if (
                    merged.length === 0 ||
                    !currentPath.startsWith(merged[merged.length - 1])
                ) {
                    merged.push(currentPath);
                }
            }

            return merged;
        }
        let addInfos = [];
        let deleteInfos = [];
        let updateInfos = [];

        const zip1 = new JSZip();
        const zip2 = new JSZip();

        const srcZipContent = await zip1.loadAsync(srcZip);
        const dstZipContent = await zip2.loadAsync(dstZip);

        const srcFiles = new Set(Object.keys(srcZipContent.files));
        const dstFiles = new Set(Object.keys(dstZipContent.files));

        for (const filePath of dstFiles) {
            if (!srcFiles.has(filePath)) {
                addInfos.push(filePath);
            }
        }

        for (const filePath of srcFiles) {
            if (!dstFiles.has(filePath)) {
                deleteInfos.push(filePath);
            }
        }

        for (const filePath of dstFiles) {
            if (srcFiles.has(filePath)) {
                const srcFile = srcZipContent.files[filePath];
                const dstFile = dstZipContent.files[filePath];

                if (!srcFile.dir && !dstFile.dir) {
                    const srcContent = await srcFile.async('arraybuffer');
                    const dstContent = await dstFile.async('arraybuffer');

                    if (!areBuffersEqual(srcContent, dstContent)) {
                        updateInfos.push(filePath);
                    }
                }
            }
        }
        addInfos.sort();
        deleteInfos.sort();
        updateInfos.sort();

        deleteInfos = mergeDeleteInfos(deleteInfos);

        return { addInfos, deleteInfos, updateInfos }
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

    async installProject() {
        try {
            let dbExists = await this.checkDBExist(this.persistentPath, this.getInstallPath());
            console.log(this.getInstallPath(), " DBExist result= ", dbExists, "this.isEditor", this.isEditor);
            if (dbExists && !this.isEditor) {
                this.RunGame();
            } else {
                this.onProgress(0.1);
                this.editor = new Engine(this.editorConfig);
                this.clearPersistence(this.tempZipPath);
                if (!dbExists) {
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
        if (!this.isEditor) {
            args.push("--headless");
            args.push("--quit-after");
            args.push("30");
            this.exitFunc = this.RunGame.bind(this);
        }

        console.log("runEditor ", args);
        this.editor.init(basePath).then(() => {
            this.onProgress(0.4);
            this.editor.start({ 'args': args, 'persistentDrops': false, 'canvas': this.editorCanvas }).then(async () => {
                if (this.isEditor) {
                    this.editorCanvas.focus();
                    this.onProgress(0.9);
                    await this.mergeProjectWithEngineRes()
                    window.goLoadData(new Uint8Array(this.projectData));
                    this.onProgress(1.0);
                }
            });
        });
    }

    RunGame() {
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
            });
        });
    }
}
