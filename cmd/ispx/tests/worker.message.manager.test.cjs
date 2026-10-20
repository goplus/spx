const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const sourcePath = path.join(__dirname, '../web/worker.message.manager.js');
const source = fs.readFileSync(sourcePath, 'utf8');

function loadManager() {
    const window = {};
    const errors = [];
    const context = vm.createContext({
        window,
        Date: { now: () => 1788776630000 },
        console: { error: (...args) => errors.push(args), warn: () => {} },
    });
    vm.runInContext(source, context, { filename: sourcePath });
    const manager = new context.WorkerMessageManager();
    const messages = [];
    manager.setPThreads({
        runningWorkers: [{ postMessage: (message) => messages.push(message) }],
    });
    const engine = { rtenv: {} };
    manager.bindMainThreadCallbacks(engine);
    return { context, window, manager, messages, errors, call: engine.rtenv._spxOnMainCall };
}

const settleCallbacks = () => new Promise(setImmediate);

test('consecutive calls with a frozen clock keep distinct callback routes', async () => {
    const { manager, window, messages, call } = loadManager();
    const received = [];
    manager.callWorkerFunction('subscribe', 'first', (value) => received.push(['first', value]), 42);
    manager.callWorkerFunction('subscribe', 'second', (value) => received.push(['second', value]), null);

    const first = messages[0].data.args;
    const second = messages[1].data.args;
    assert.deepEqual(Array.from(first), ['first', '_SPX_CALLBACK_FUNC_', first[2], 42]);
    assert.deepEqual(Array.from(second), ['second', '_SPX_CALLBACK_FUNC_', second[2], null]);
    assert.notEqual(first[2], second[2]);
    assert.equal(Object.keys(window._spxMainCalls).length, 2);

    call(second[2], 'request-b', 'B');
    call(first[2], 'request-a', 'A');
    await settleCallbacks();
    assert.deepEqual(received, [['second', 'B'], ['first', 'A']]);
    assert.deepEqual(messages.slice(2).map((message) => message.responseId), ['request-b', 'request-a']);
});

test('async callbacks respond to their own requests when they finish out of order', async () => {
    const { manager, messages, call } = loadManager();
    let resolveFirst;
    let resolveSecond;
    const first = manager.processArguments(() => new Promise((resolve) => { resolveFirst = resolve; }))[1];
    const second = manager.processArguments(() => new Promise((resolve) => { resolveSecond = resolve; }))[1];
    call(first, 'request-a');
    call(second, 'request-b');

    resolveSecond('result-b');
    await settleCallbacks();
    resolveFirst('result-a');
    await settleCallbacks();
    assert.deepEqual(messages.map(({ responseId, result, error }) => ({ responseId, result, error })), [
        { responseId: 'request-b', result: 'result-b', error: null },
        { responseId: 'request-a', result: 'result-a', error: null },
    ]);
});

test('subscriptions remain callable until the callback registry is cleared', async () => {
    const { manager, window, messages, call } = loadManager();
    const received = [];
    const callbackName = manager.processArguments((value) => received.push(value))[1];
    call(callbackName, 'first', 1);
    call(callbackName, 'second', 2);
    await settleCallbacks();
    assert.deepEqual(received, [1, 2]);
    assert.equal(typeof window._spxMainCalls[callbackName], 'function');

    manager.bindMainCallHandler();
    assert.equal(Object.keys(window._spxMainCalls).length, 0);
    const nextCallback = manager.processArguments((value) => received.push(value))[1];
    assert.notEqual(nextCallback, callbackName);
    call(callbackName, 'stale', 3);
    call(nextCallback, 'current', 4);
    await settleCallbacks();
    assert.deepEqual(received, [1, 2, 4]);
    assert.deepEqual(messages.map((message) => message.responseId), ['first', 'second', 'current']);
});

test('a replacement manager does not reuse cleared callback identifiers', async () => {
    const { context, manager, window, call } = loadManager();
    const oldCallback = manager.processArguments(() => assert.fail('cleared callback ran'))[1];
    const replacement = new context.WorkerMessageManager();
    let calls = 0;
    const newCallback = replacement.processArguments(() => { calls++; })[1];
    assert.notEqual(oldCallback, newCallback);
    assert.equal(window._spxMainCalls[oldCallback], undefined);

    call(oldCallback, 'stale');
    window._spxOnMainCall(newCallback, 'current');
    await settleCallbacks();
    assert.equal(calls, 1);
});

test('clearing the registry discards pending responses without disabling new callbacks', async () => {
    const { manager, messages, window, call } = loadManager();
    let resolveOld;
    const oldCallback = manager.processArguments(() => new Promise((resolve) => { resolveOld = resolve; }))[1];
    call(oldCallback, 'old-request');

    manager.bindMainCallHandler();
    const currentCallback = manager.processArguments(() => 'current-result')[1];
    window._spxOnMainCall(currentCallback, 'current-request');
    resolveOld('old-result');
    await settleCallbacks();
    assert.deepEqual(messages.map(({ responseId, result }) => ({ responseId, result })), [
        { responseId: 'current-request', result: 'current-result' },
    ]);
});

for (const asynchronous of [false, true]) {
    test(`callback errors preserve their request identity (async=${asynchronous})`, async () => {
        const { manager, messages, errors, call } = loadManager();
        const failure = new Error('sentinel');
        const callbackName = manager.processArguments(() => {
            if (asynchronous) return Promise.reject(failure);
            throw failure;
        })[1];
        call(callbackName, 'failed-request');
        await settleCallbacks();
        assert.equal(messages.length, 1);
        assert.equal(messages[0].responseId, 'failed-request');
        assert.equal(messages[0].error, 'sentinel');
        assert.equal(errors.length, 1);
    });
}

test('broadcasts survive invalid or throwing workers without mutating the source message', () => {
    const { manager, messages, errors } = loadManager();
    const message = { cmd: 'customCall', data: { value: 42 } };
    manager.setPThreads(null);
    assert.deepEqual({ ...manager.postMessageToWorkers(message) }, { successCount: 0, errorCount: 0, totalWorkers: 0 });
    manager.setPThreads({ runningWorkers: [
        null,
        { postMessage() { throw new Error('worker stopped'); } },
        { postMessage: value => messages.push(value) },
    ] });
    assert.deepEqual({ ...manager.postMessageToWorkers(message) }, { successCount: 1, errorCount: 2, totalWorkers: 3 });
    assert.deepEqual(message, { cmd: 'customCall', data: { value: 42 } });
    assert.equal(messages[0]._targetWorkerIndex, 2);
    assert.equal(messages[0]._gameRunnerMessageId, 2);
    assert.equal(messages[0].data, message.data);
    assert.equal(errors.length, 1);
    manager.postMessageToWorkers(message);
    assert.equal(messages[1]._gameRunnerMessageId, 4);
});

test('transfer broadcasts clone only the visible bytes independently for each worker', () => {
    const { manager } = loadManager();
    const received = [];
    manager.setPThreads({ runningWorkers: Array.from({ length: 2 }, () => ({
        postMessage(message, transfer) {
            received.push(structuredClone(message, { transfer }));
        },
    })) });
    const backing = new Uint8Array([99, 1, 2, 88]);
    const data = backing.subarray(1, 3);
    const result = manager.postMessageToWorkers({ cmd: 'bytes', data }, [data.buffer], true);
    assert.equal(result.successCount, 2);
    assert.deepEqual(Array.from(backing), [99, 1, 2, 88], 'caller storage must not be detached');
    assert.deepEqual(received.map(message => Array.from(message.data)), [[1, 2], [1, 2]]);
    assert.notEqual(received[0].data.buffer, received[1].data.buffer);
    received[0].data[0] = 42;
    assert.equal(received[1].data[0], 1);
});
