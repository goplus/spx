class GameApp {
    constructor(config) {
        this.appConfig = config || null;
        this.editor = null;
        this.game = null;
        this.persistentPath = '/home/web_user';
        this.tempZipPath = '/tmp/preload.zip';
        this.projectInstallName = config?.projectName || "Game";
        this.projectData = config.projectData;
        this.persistentPaths = [this.persistentPath];
        this.editorCanvas = config.editorCanvas;
        this.gameCanvas = config.gameCanvas;
        this.exitFunc = null;
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

    Start() {
        this.installProject();
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

    onProgress(value){
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

    async installProject() {
        try {
            console.log("merge zip files");
			const engineDataResp = fetch("engineres.zip");
			let engineData = await (await engineDataResp).arrayBuffer();
            this.projectData = await this.mergeZips(this.projectData, engineData);

            let dbExists = await this.checkDBExist(this.persistentPath, this.getInstallPath());
            console.log(this.getInstallPath(), " DBExist result= ", dbExists);
            if (dbExists) {
                this.runGame();
            } else {
                this.clearPersistence(this.tempZipPath);
                this.onProgress(0.1);
                this.editor = new Engine(this.editorConfig);
                this.exitFunc = this.importProject.bind(this);
                this.editor.init('godot.editor').then(() => {
                    this.editor.copyToFS(this.tempZipPath, this.projectData);
                    const args = ['--project-manager', '--single-window', "--install_project_name", this.projectInstallName];
                    this.editor.start({ 'args': args, 'persistentDrops': true });
                });
            }
        } catch (error) {
            console.error("Error checking database existence: ", error);
        }
    }

    importProject() {
        const args = [
            "--path",
            this.getInstallPath(),
            "--single-window",
            "--headless",
            "--editor",
            "--quit-after",
            "30"
        ];
        console.log("importProject ", args);
        this.exitFunc = this.runGame.bind(this);
        this.editor.init().then(() => {
            this.editor.start({ 'args': args, 'persistentDrops': false, 'canvas': this.editorCanvas });
        });
    }

    runGame() {
        const args = [
            "--path",
            this.getInstallPath(),
            "--editor-pid",
            "0",
            "res://main.tscn"
        ];
        const gameConfig = {
            'persistentPaths': this.persistentPaths,
            'unloadAfterInit': false,
            'canvas': this.gameCanvas,
            'canvasResizePolicy': 1,
            'onExit': () => {
                this.game = null;
            },
        };
        console.log("runGame ", args);
        if (this.game) {
            console.error('A game is already running. Close it first');
            return;
        }
        this.onProgress(0.5);
        this.exitFunc = null;
        this.game = new Engine(gameConfig);
        this.game.init('godot.editor').then(() => {
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
