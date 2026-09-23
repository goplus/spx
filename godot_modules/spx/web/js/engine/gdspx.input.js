const inputActionRegistry = {
    module: null,
    epoch: 0,
    bridge: null,
};

function GdspxResetInputActions() {
    inputActionRegistry.epoch += 1;
    inputActionRegistry.bridge = null;
}

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
        GdspxResetInputActions();
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
    const bridge = GetInputBridge();
    const call = bridge && bridge['gdspx_input_register_action'];
    if (typeof call !== 'function') {
        return -1;
    }

    return ToStableActionID(call.call(bridge, action));
}

globalThis['GdspxResetInputActions'] = GdspxResetInputActions;
globalThis['GdspxGetInputActionEpoch'] = GdspxGetInputActionEpoch;
globalThis['GdspxGetInputActionID'] = GdspxGetInputActionID;
