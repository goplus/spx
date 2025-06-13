
function handleGameAppMessage(data) {
  const workerId = (typeof Module !== 'undefined' && Module['workerID']) || 'unknown';
  const threadInfo = typeof importScripts !== 'undefined' ? 'Worker' : 'MainThread';
  try {
    switch (data.cmd) {
      case 'projectDataUpdate':
        handleProjectDataUpdate(data);
        break;
      case 'customCall':
        console.log("handleCustomCall", data)
        handleCustomCall(data);
        break;
      default:
        console.warn(`[Thread ${threadInfo}-${workerId}] Unknown GameApp command:`, data.cmd);
        break;
    }
  } catch (error) {
    console.error(`[Thread ${threadInfo}-${workerId}] Error handling GameApp message:`, error);
  }
}

async function handleProjectDataUpdate(data) {
  Module["gameProjectData"] = data.data;
  self.tryRunGoWasm()
}

async function handleCustomCall(data) {
  var infos = data.data
  if (infos.funcName === 'setAIInteractionAPIEndpoint') {
    console.log("handleCustomCall setAIInteractionAPIEndpoint", infos)
    try {
      await self.goBridge.callGoFunctionSafe('setAIInteractionAPIEndpoint', infos.args);
      postMessage({
        cmd: 'callHandler',
        handler: '_spxCbSetAIInteractionAPIEndpoint',
        args: ["success"]
      });
    } catch (error) {
      console.error("Error in setAIInteractionAPIEndpoint:", error);
    }
  }

  if (infos.funcName === 'setAIInteractionAPITokenProvider') {
    try {
      var result = await self.goBridge.callGoFunctionSafe('setAIInteractionAPITokenProvider', infos.args);
      var param = result == null ? "" : result
      postMessage({
        cmd: 'callHandler',
        handler: '_spxCbSetAIInteractionAPITokenProvider',
        args: [param]
      });
    } catch (error) {
      console.error("Error in setAIInteractionAPITokenProvider:", error);
    }
  }
}

function tryRunGoWasm() {
  const workerId = (typeof Module !== 'undefined' && Module['workerID']) || 'unknown';
  if (!Module["FFI"]) {
    return;
  }
  if (!Module["gameProjectData"]) {
    return;
  }

  if (self.goBridge && self.goBridge.isReady) {
    try {
      // If Go WASM is ready, can call related functions to process data
      self.goBridge.callGoFunctionSafe('goLoadData', Module["gameProjectData"]);
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
  var createWrapper = function (module, name) {
    return function () {
      return module["asm"][name].apply(null, arguments);
    };
  }
  const workerId = Module['workerID'] || 'main';
  const threadInfo = typeof importScripts !== 'undefined' ? 'Worker' : 'MainThread';
  Module._cmalloc = createWrapper(Module, "malloc");
  Module._cfree = createWrapper(Module, "free");
  FFI = null

  try {
    // Load Go WASM module
    await loadGoWasmModule();
    FFI = Module["FFI"];
    self.tryRunGoWasm()
    return true;
  } catch (error) {
    console.error(`[Thread ${threadInfo}-${workerId}] Go WASM initialization failed:`, error);
    return false;
  }
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

    // Initialize Go WASM module
    await goBridge.initialize({
      wasmPath: './gdspx.wasm',
      timeout: 15000,
      enableDebug: false
    });

    // Try to call Go initialization function (optional)
    try {
      const initResult = await goBridge.callGoFunctionSafe('goWasmInit');
      Module['FFI'] = BindFFI(goBridge);
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
