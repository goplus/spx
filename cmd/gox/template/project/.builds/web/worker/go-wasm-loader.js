
function handleGameAppMessage(data) {
  const workerId = (typeof Module !== 'undefined' && Module['workerID']) || 'unknown';
  const threadInfo = typeof importScripts !== 'undefined' ? 'Worker' : 'MainThread';
  try {
    switch (data.cmd) {
      case 'projectDataUpdate':
        handleProjectDataUpdate(data);
        break;
      case 'customCall':
        handleCustomCall(data);
        break;
      case 'callResponse':
        handleCallResponse(data);
        break;
      default:
        console.warn(`[Thread ${threadInfo}-${workerId}] Unknown GameApp command:`, data.cmd || data.type);
        break;
    }
  } catch (error) {
    console.error(`[Thread ${threadInfo}-${workerId}] Error handling GameApp message:`, error);
  }
}

async function handleProjectDataUpdate(data) {
  Module["gameProjectData"] = data.data;
  Module["gameAssetURLs"] = data.gameAssetURLs;
  initExtensionWasm()
}

async function handleCustomCall(data) {
  var infos = data.data
  var funcName = infos.funcName
  try {
    // check if there is a function pointer proxy requirement in the parameters
    var processedArgs = processMainThreadCallbacks(infos.args);

    var result = await self.goBridge.callGoFunctionSafe(funcName, ...processedArgs);
    var param = result == null ? "" : result
    // TODO implement return result
    //postMessage({
    //  cmd: 'callHandler',
    //  handler: '_onWorkerCb_' + funcName,
    //  args: [param]
    //});
  } catch (error) {
    console.error("Error in " + funcName + ":", error);
  }
}

// process main thread callback parameters, convert _SPX_CALLBACK_FUNC_ to actual proxy function
function processMainThreadCallbacks(args) {
  if (!args || !Array.isArray(args)) {
    return args;
  }

  var processedArgs = [];
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "_SPX_CALLBACK_FUNC_" && i + 1 < args.length) {
      // the next parameter is the callback function name
      var callbackName = args[i + 1];
      // create proxy function
      var proxyFunction = createMainThreadCallbackProxy(callbackName);
      processedArgs.push(proxyFunction);
      i++; // skip callback function name parameter
    } else {
      processedArgs.push(args[i]);
    }
  }
  return processedArgs;
}

function callMainThread(callbackName, args) {
  postMessage({
    cmd: 'callHandler',
    handler: "_spxOnMainCall",
    args: args ? [callbackName, ...args] : [callbackName]
  });
}

// create main thread callback proxy function
function createMainThreadCallbackProxy(callbackName) {
  return function (...args) {
    return new Promise((resolve, reject) => {
      const requestId = ++tokenRequestId;

      // save Promise resolve/reject
      pendingTokenRequests.set(requestId, { resolve, reject });

      // use callHandler mechanism to send callback request to main thread
      callMainThread(callbackName, [requestId, ...args]);

      // set timeout
      setTimeout(() => {
        if (pendingTokenRequests.has(requestId)) {
          pendingTokenRequests.delete(requestId);
          reject(new Error(`Callback ${callbackName} timeout`));
        }
      }, 10000); // 10 seconds timeout
    });
  };
}

function tryRunGoWasm() {
  const workerId = (typeof Module !== 'undefined' && Module['workerID']) || 'unknown';
  if (!Module["FFI"]) {
    return;
  }
  if (!Module["gameProjectData"]) {
    return;
  }
  
  const spxfuncs = new GdspxFuncs();
  const methodNames = Object.getOwnPropertyNames(Object.getPrototypeOf(spxfuncs));
  methodNames.forEach(key => {
      if (key.startsWith('gdspx_') && typeof spxfuncs[key] === 'function') {
          self[key] = spxfuncs[key].bind(spxfuncs);
      }
  });
  self.Module = Module;

  if (self.goBridge && self.goBridge.isReady) {
    try {
      // If Go WASM is ready, can call related functions to process data
      self.goBridge.callGoFunctionSafe('goLoadData', Module["gameProjectData"]);
      callMainThread('onGameStarted');
    } catch (error) {
      console.error(`[Worker ${workerId}] Error calling Go function to process project data:`, error);
    }
  }
}

/**
 * Initializes Go WASM on demand (callable from any thread)
 * This function will be called on godot_js_spx_on_engine_start callback
 */
async function initExtensionWasm() {
  if (Module["gameAssetURLs"] == undefined) {
    return;
  }
  const workerId = Module['workerID'] || 'main';
  const threadInfo = typeof importScripts !== 'undefined' ? 'Worker' : 'MainThread';
  FFI = null

  try {
    // Load Go WASM module
    await loadGoWasmModule();
    FFI = Module["FFI"];
    tryRunGoWasm()
    return true;
  } catch (error) {
    console.error(`[Thread ${threadInfo}-${workerId}] Go WASM initialization failed:`, error);
    return false;
  }
}

// AI Token Provider related variables and functions
let tokenRequestId = 0;
const pendingTokenRequests = new Map();

// requestTokenFromMainThread function is no longer needed, because now using the generic function proxy mechanism

function handleCallResponse(data) {
  if (data.responseId) {
    const requestId = parseInt(data.responseId);
    if (pendingTokenRequests.has(requestId)) {
      const { resolve, reject } = pendingTokenRequests.get(requestId);
      pendingTokenRequests.delete(requestId);

      if (data.error) {
        reject(new Error(data.error));
      } else {
        resolve(data.result || "");
      }
      return;
    }
    console.error("handleCallResponse: no pendingTokenRequests", data)
  }
  console.error("handleCallResponse: no responseId", data)
}

// Expose functions to global scope for godot.editor.js to call
if (typeof self !== 'undefined') {
  self.initExtensionWasm = initExtensionWasm;
}


/**
 * Simplified Go WASM module loading logic
 */
async function loadGoWasmModule() {
  // If already loaded, return immediately
  if (self.goBridge && self.goBridge.isReady) {
    console.log(`[Godot Worker ${Module['workerID']}] Go WASM is already loaded, using directly`);
    return;
  }

  try {
    // Create Go WASM Bridge instance
    const goBridge = new GoWasmBridge();

    let assetURLs = Module["gameAssetURLs"];

    // Initialize Go WASM module
    await goBridge.initialize({
      wasmPath: assetURLs["gdspx.wasm"],
      timeout: 15000,
      enableDebug: false
    });

    // Try to call Go initialization function (optional)
    try {
      const initResult = await goBridge.callGoFunctionSafe('goWasmInit');
      Module['FFI'] = BindFFI(goBridge);
      callMainThread('onWasmLoaded');
    } catch (goInitError) {
      console.warn(`[Godot Worker ${Module['workerID']}] Go initialization function call failed, but continuing execution:`, goInitError);
    }

    // Expose Go Bridge instance to global scope of current worker
    self.goBridge = goBridge;
  } catch (error) {
    console.error(`[Godot Worker ${Module['workerID']}] Go WASM module loading failed:`, error);
    throw error;
  }
}
