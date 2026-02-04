
// PThread Worker 消息管理器类
class WorkerMessageManager {
    constructor() {
        this.pthreads = null;
        this.workerMessageId = 0;
        this.bindMainCallHandler();
    }

    setPThreads(pthreads) {
        this.pthreads = pthreads;
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
    
    callWorkerProjectDataUpdate(projectData, assetURLs) {
        const message = {
            cmd: 'projectDataUpdate',
            data: projectData,
            gameAssetURLs: assetURLs,
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

// export StorageManager to global
globalThis.WorkerMessageManager = WorkerMessageManager; 