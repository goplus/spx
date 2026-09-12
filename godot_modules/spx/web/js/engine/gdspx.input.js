// Action IDs are valid only for the module that registered them.
const inputActionRegistry = {
    module: null,
    epoch: 0,
    ids: new Map(),
    bridge: null,
};

function GetInputBridge() {
    if (typeof globalThis['gdspx_input_register_action'] === 'function') {
        return globalThis;
    }
    if (!inputActionRegistry.bridge && typeof GdspxFuncs === 'function') {
        inputActionRegistry.bridge = new GdspxFuncs();
    }
    return inputActionRegistry.bridge;
}

function ToStableActionID(value) {
    if (value && typeof value['low'] === 'number') {
        return value['low'] | 0;
    }
    if (typeof value === 'bigint') {
        return Number(BigInt.asIntN(32, value));
    }
    return Number(value) | 0;
}

function EnsureInputActionRegistry() {
    if (!HasActiveModule()) {
        return false;
    }
    if (inputActionRegistry.module !== Module) {
        inputActionRegistry.module = Module;
        inputActionRegistry.epoch += 1;
        inputActionRegistry.ids.clear();
        inputActionRegistry.bridge = null;
    }
    return true;
}

function GdspxGetInputActionEpoch() {
    EnsureInputActionRegistry();
    return inputActionRegistry.epoch;
}

function GdspxGetInputActionID(action) {
    if (!EnsureInputActionRegistry()) {
        return -1;
    }
    if (inputActionRegistry.ids.has(action)) {
        return inputActionRegistry.ids.get(action);
    }

    const bridge = GetInputBridge();
    const call = bridge && bridge['gdspx_input_register_action'];
    if (typeof call !== 'function') {
        return -1;
    }

    const id = ToStableActionID(call.call(bridge, action));
    if (id >= 0) {
        inputActionRegistry.ids.set(action, id);
    }
    return id;
}

globalThis['GdspxGetInputActionEpoch'] = GdspxGetInputActionEpoch;
globalThis['GdspxGetInputActionID'] = GdspxGetInputActionID;
