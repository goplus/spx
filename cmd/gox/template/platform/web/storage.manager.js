class StorageManager {
    constructor(config) {
        this.webPersistentPath = config.webPersistentPath || '/home/web_user';
        this.projectInstallName = config.projectInstallName || 'Game';
        this.useAssetCache = config.useAssetCache !== false;
        this.assetURLs = config.assetURLs || {};
        this.debugInfo = config.debugInfo || "";
        this.logVerbose = config.logVerbose || console.log.bind(console);
    }

    //------------------ install project ------------------
    getInstallPath() {
        return `${this.webPersistentPath}/${this.projectInstallName}`;
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
            let cacheValue = await this.queryIndexDB(this.webPersistentPath, 'FILE_DATA', storeName);
            return cacheValue;
        } catch (error) {
            console.error(error);
            return undefined;
        }
    }

    async setCache(storeName, value) {
        try {
            let cacheValue = await this.updateIndexDB(this.webPersistentPath, 'FILE_DATA', storeName, value);
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
        return `${this.webPersistentPath}/.spx_cache_data/${this.projectInstallName}`
    }
    
    getProjectHashKey() {
        return `${this.webPersistentPath}/.spx_cache_hash/${this.projectInstallName}`
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
            this.clearPersistence(this.webPersistentPath);
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
        await this.getObjectStore(this.webPersistentPath, 'FILE_DATA', 'readonly')
    }

    getEngineHashKey(assetName) {
        return `${this.webPersistentPath}/.spx_engine_hash/${assetName}`
    }
    
    getEngineDataKey(assetName) {
        return `${this.webPersistentPath}/.spx_engine_data/${assetName}`
    }
    
    async checkEngineCache(hashes) {
        this.logVerbose("curHashes ", hashes)
        const wasmGdspx = await this.checkCacheAsset(hashes, "gdspx.wasm");
        const wasmEngine = await this.checkCacheAsset(hashes, "engine.wasm");
        console.log("checkEngineCache ", wasmGdspx, wasmEngine)
        return { wasmGdspx, wasmEngine };
    }

    async checkCacheAsset(hashes, assetName) {
        try {
            let url = this.assetURLs[assetName]
            if (!this.useAssetCache) {
                return await (await fetch(url)).arrayBuffer();
            }

            let curHash = hashes[assetName];
            await this.ensureCacheDB();

            const cachedHash = await this.getCache(this.getEngineHashKey(assetName));
            const isCacheValid = cachedHash !== undefined && curHash === cachedHash;

            if (isCacheValid) {
                const curData = await this.getCache(this.getEngineDataKey(assetName));
                if (curData != undefined && curData.byteLength != 0) {
                    this.logVerbose("Load cached engine asset:", assetName, curData.byteLength);
                    return curData;
                }
            }
            const curData = await (await fetch(url)).arrayBuffer();
            this.logVerbose("Download engine asset:", assetName, url, " curData.byteLength: ", curData.byteLength);
            await this.setCache(this.getEngineDataKey(assetName), curData);
            await this.setCache(this.getEngineHashKey(assetName), curHash);
            return curData;
        } catch (error) {
            console.error("Error checking engine cache asset:", error);
            throw error;
        }
    }

    // 提供更新调试信息的方法
    updateDebugInfo(info) {
        this.debugInfo += info;
    }

    // 获取当前项目哈希
    getCurrentProjectHash() {
        return this.curProjectHash;
    }
}

// export StorageManager to global
globalThis.StorageManager = StorageManager; 