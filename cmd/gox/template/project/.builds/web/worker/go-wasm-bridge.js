/**
 * Go WASM Bridge for Godot Workers
 * 
 * This file provides a complete solution for integrating Go WASM modules in Godot Workers,
 * including module loading, function calling, error handling, and performance optimization.
 */

class GoWasmBridge {
    constructor() {
        this.goInstance = null;
        this.goRuntime = null;
        this.isReady = false;
        this.pendingCalls = [];
        this.callCounter = 0;
        this.activeCalls = new Map();
        
        // Configuration options
        this.config = {
            wasmPath: './main.wasm',
            timeout: 10000, // 10 second timeout
            enableDebug: false
        };
        
        // Bind methods
        this.loadGoModule = this.loadGoModule.bind(this);
        this.callGoFunction = this.callGoFunction.bind(this);
        this.handleGoMessage = this.handleGoMessage.bind(this);
    }
    
    /**
     * Initialize Go WASM module
     * @param {Object} options Configuration options
     * @returns {Promise} Initialization Promise
     */
    async initialize(options = {}) {
        // Merge options
        Object.assign(this.config, options);
        
        try {
            this.log('Initializing Go WASM module...');
            
            // Load Go runtime
            await this.loadGoRuntime();
            
            // Load Go WASM module
            await this.loadGoModule();
            
            this.log('Go WASM module initialization complete');
            return true;
            
        } catch (error) {
            this.error('Go WASM module initialization failed:', error);
            throw error;
        }
    }
    
    /**
     * Load Go runtime
     * @returns {Promise}
     */
    loadGoRuntime() {
        return new Promise((resolve, reject) => {
            try {
                // Import Go runtime script
                if (this.config.runtimePath !== undefined && this.config.runtimePath !== null && this.config.runtimePath !== '') {
                    importScripts(this.config.runtimePath);
                }
                
                // Create Go instance
                this.goRuntime = new Go();
                this.log('Go runtime loaded successfully');
                resolve();
                
            } catch (error) {
                reject(new Error(`Failed to load Go runtime: ${error.message}`));
            }
        });
    }
    
    /**
     * Load Go WASM module
     * @returns {Promise}
     */
    async loadGoModule() {
        try {
            // Fetch WASM bytes
            const wasmBytes = await this.fetchWasm(this.config.wasmPath);
            
            // Instantiate WASM module
            const wasmModule = await WebAssembly.instantiate(wasmBytes, this.goRuntime.importObject);
            this.goInstance = wasmModule.instance;
            
            // Set up message listener
            this.setupMessageHandling();
            
            // Create a Promise to wait for Go module readiness
            const readyPromise = new Promise((resolve, reject) => {
                // Setup timeout check
                const timeout = setTimeout(() => {
                    reject(new Error('Go module initialization timed out'));
                }, this.config.timeout || 15000);
                
                // Save resolve function for module-ready callback
                this._moduleReadyResolve = () => {
                    clearTimeout(timeout);
                    clearInterval(checkInterval);
                    resolve();
                };
                
                this._moduleReadyReject = (error) => {
                    clearTimeout(timeout);
                    clearInterval(checkInterval);
                    reject(error);
                };
                
                // Fallback mechanism: poll for Go functions availability
                const checkInterval = setInterval(() => {
                    const availableFunctions = this.getAvailableGoFunctions();
                    if (availableFunctions.length > 0) {
                        this.log('Detected Go functions available through polling:', availableFunctions);
                        this.isReady = true;
                        this._moduleReadyResolve();
                    }
                }, 100); // Check every 100ms
            });
            
            // Run Go program (asynchronously)
            this.goRuntime.run(this.goInstance).catch(error => {
                this.error('Go program execution failed:', error);
                if (this._moduleReadyReject) {
                    this._moduleReadyReject(error);
                }
            });
            
            this.log('Go WASM module initialization started, waiting for readiness...');
            
            // Await Go module readiness
            await readyPromise;
            
            this.log('Go WASM module loaded and initialized successfully');
            
        } catch (error) {
            throw new Error(`Failed to load Go WASM module: ${error.message}`);
        }
    }
    
    /**
     * Fetch WASM bytes
     * @param {string} wasmPath WASM file path
     * @returns {Promise<ArrayBuffer>}
     */
    async fetchWasm(wasmPath) {
        try {
            const response = await fetch(wasmPath);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return await response.arrayBuffer();
        } catch (error) {
            throw new Error(`Failed to fetch WASM file: ${error.message}`);
        }
    }
    
    /**
     * Set up message handling mechanism
     */
    setupMessageHandling() {
        // Listen for messages from Go
        const originalPostMessage = self.postMessage;
        self.postMessage = (data) => {
            if (this.isGoMessage(data)) {
                this.handleGoMessage(data);
            } else {
                originalPostMessage.call(self, data);
            }
        };
    }
    
    /**
     * Check if message is from Go
     * @param {*} data Message data
     * @returns {boolean}
     */
    isGoMessage(data) {
        return data && typeof data === 'object' && 
               (data.cmd === 'goReady' || data.source === 'go-wasm');
    }
    
    /**
     * Handle message from Go
     * @param {Object} data Message data
     */
    handleGoMessage(data) {
        switch (data.cmd) {
            case 'goReady':
                this.handleGoReady(data);
                break;
            case 'goFunction':
                this.handleGoFunctionCall(data);
                break;
            default:
                this.log('Received unknown Go message:', data);
        }
    }
    
    /**
     * Handle Go module readiness message
     * @param {Object} data Message data
     */
    handleGoReady(data) {
        this.isReady = true;
        this.log('Go module is ready, available functions:', data.functions);
        
        // Process pending function calls
        this.processPendingCalls();
        
        // If there's a pending init Promise, resolve it
        if (this._moduleReadyResolve) {
            this._moduleReadyResolve();
            this._moduleReadyResolve = null;
            this._moduleReadyReject = null;
        }
        
        // Notify main thread
        self.postMessage({
            cmd: 'goModuleReady',
            availableFunctions: data.functions || [],
            source: 'go-wasm-bridge'
        });
    }
    
    /**
     * Process pending function calls
     */
    processPendingCalls() {
        while (this.pendingCalls.length > 0) {
            const call = this.pendingCalls.shift();
            this.executeGoFunction(call.funcName, call.args, call.resolve, call.reject);
        }
    }
    
    /**
     * Call Go function
     * @param {string} funcName Function name
     * @param {...*} args Arguments
     * @returns {Promise} Call result
     */
    callGoFunction(funcName, ...args) {
        return new Promise((resolve, reject) => {
            if (!this.isReady) {
                // Module isn't ready, queue the call
                this.pendingCalls.push({ funcName, args, resolve, reject });
                return;
            }
            
            this.executeGoFunction(funcName, args, resolve, reject);
        });
    }
    
    getGoFunction(funcName){
        const goFunc = self[funcName];
        if (typeof goFunc !== 'function') {
            console.error(`Go function ${funcName} does not exist`);
            return null
        }
        return goFunc
    }
    /**
     * Execute Go function
     * @param {string} funcName Function name
     * @param {Array} args Argument array
     * @param {Function} resolve Resolve callback
     * @param {Function} reject Reject callback
     */
    executeGoFunction(funcName, args, resolve, reject) {
        try {
            // Verify function exists
            const goFunc = self[funcName];
            if (typeof goFunc !== 'function') {
                reject(new Error(`Go function ${funcName} does not exist`));
                return;
            }
            
            // Set timeout
            const timeoutId = setTimeout(() => {
                reject(new Error(`Go function ${funcName} call timed out`));
            }, this.config.timeout);
            
            // Call function
            const result = goFunc(...args);
            
            // Handle return value
            if (result && typeof result.then === 'function') {
                // Promise return value
                result
                    .then(value => {
                        clearTimeout(timeoutId);
                        resolve(value);
                    })
                    .catch(error => {
                        clearTimeout(timeoutId);
                        reject(error);
                    });
            } else {
                // Synchronous return value
                clearTimeout(timeoutId);
                resolve(result);
            }
            
        } catch (error) {
            reject(new Error(`Failed to execute Go function ${funcName}: ${error.message}`));
        }
    }
    
    /**
     * Call multiple Go functions
     * @param {Array} calls Call configuration array [{funcName, args}, ...]
     * @returns {Promise<Array>} Result array
     */
    async callGoFunctions(calls) {
        const promises = calls.map(call => 
            this.callGoFunction(call.funcName, ...(call.args || []))
        );
        return await Promise.all(promises);
    }
    
    /**
     * Get available Go functions
     * @returns {Array} Function name array
     */
    getAvailableGoFunctions() {
        const functions = [];
        for (const key in self) {
            if (typeof self[key] === 'function' && key.startsWith('go')) {
                functions.push(key);
            }
        }
        return functions;
    }
    
    /**
     * Safely call Go function (with complete error handling)
     * @param {string} funcName Function name
     * @param {...*} args Arguments
     * @returns {Promise} Call result
     */
    async callGoFunctionSafe(funcName, ...args) {
        try {
            // Validate parameters
            if (!funcName || typeof funcName !== 'string') {
                throw new Error('Function name must be a valid string');
            }
            
            if (!this.isReady) {
                throw new Error('Go module is not ready');
            }
            
            // Call function
            const result = await this.callGoFunction(funcName, ...args);
            
            // Validate result
            if (result && typeof result === 'object' && result.error) {
                throw new Error(`Go function execution error: ${result.error}`);
            }
            
            return result;
            
        } catch (error) {
            this.error(`Failed to safely call Go function ${funcName}:`, error);
            
            // Record debug information
            if (this.config.enableDebug) {
                this.log('Debug information:', {
                    funcName,
                    args,
                    isReady: this.isReady,
                    availableFunctions: this.getAvailableGoFunctions()
                });
            }
            
            throw error;
        }
    }
    
    /**
     * Call Go function with transferable data
     * @param {string} funcName Function name
     * @param {ArrayBuffer} transferableData Transferable data
     * @param {...*} args Other arguments
     * @returns {Promise} Call result
     */
    async callGoFunctionWithTransfer(funcName, transferableData, ...args) {
        // Note: Optimizing transferable objects inside worker is limited
        // But this interface reserves space for future optimizations
        return this.callGoFunction(funcName, transferableData, ...args);
    }
    
    /**
     * Destroy Go module instance
     */
    destroy() {
        this.log('Destroying Go WASM module instance');
        
        // Clean up pending calls
        this.pendingCalls.forEach(call => {
            call.reject(new Error('Go module has been destroyed'));
        });
        this.pendingCalls = [];
        
        // Clean up active calls
        this.activeCalls.forEach(call => {
            call.reject(new Error('Go module has been destroyed'));
        });
        this.activeCalls.clear();
        
        // Reset state
        this.isReady = false;
        this.goInstance = null;
        this.goRuntime = null;
    }
    
    /**
     * Log output
     * @param {...*} args Log arguments
     */
    log(...args) {
        if (this.config.enableDebug) {
            console.log('[GoWasmBridge]', ...args);
        }
    }
    
    /**
     * Error log output
     * @param {...*} args Error arguments
     */
    error(...args) {
        console.error('[GoWasmBridge]', ...args);
    }
}

// Export for Worker usage
if (typeof self !== 'undefined' && typeof module === 'undefined') {
    // Directly use in Worker environment
    self.GoWasmBridge = GoWasmBridge;
} else if (typeof module !== 'undefined' && module.exports) {
    // Node.js environment
    module.exports = GoWasmBridge;
} else if (typeof window !== 'undefined') {
    // Browser environment
    window.GoWasmBridge = GoWasmBridge;
}

/**
 * Usage example:
 * 
 * // Usage in Worker
 * const bridge = new GoWasmBridge();
 * 
 * // Initialize
 * await bridge.initialize({
 *     wasmPath: './main.wasm',
 *     runtimePath: './wasm_exec.js',
 *     timeout: 5000,
 *     enableDebug: true
 * });
 * 
 * // Call Go function
 * const result = await bridge.callGoFunction('goCalculateSum', 10, 20);
 * console.log('Calculation result:', result);
 * 
 * // Safe call
 * try {
 *     const safeResult = await bridge.callGoFunctionSafe('goProcessData', data);
 *     console.log('Processing result:', safeResult);
 * } catch (error) {
 *     console.error('Call failed:', error);
 * }
 * 
 * // Batch call
 * const batchResults = await bridge.callGoFunctions([
 *     { funcName: 'goFunc1', args: [1, 2] },
 *     { funcName: 'goFunc2', args: ['hello'] }
 * ]);
 */