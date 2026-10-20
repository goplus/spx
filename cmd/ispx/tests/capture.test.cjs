const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function capture(screenshot) {
    const events = [];
    const timers = new Map();
    let timerID = 0;
    const window = {
        dispatchEvent: event => events.push(event.detail),
        setTimeout: callback => { timers.set(++timerID, callback); return timerID; },
        clearTimeout: id => timers.delete(id),
    };
    const document = { visibilityState: 'hidden' };
    const context = vm.createContext({
        window, document,
        CustomEvent: class { constructor(type, { detail }) { this.type = type; this.detail = detail; } },
        console: { error() {}, warn() {} },
    });
    vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/capture.js'), 'utf8'), context);
    return {
        bridge: context.createCaptureBridge(screenshot), events, window, document, context, timers,
        render() {
            for (const [id, callback] of [...timers]) {
                if (timers.delete(id)) callback();
            }
        },
    };
}

const turn = () => new Promise(setImmediate);

test('capture queue orders requests per host and shares only same-session frame screenshots', async () => {
    let screenshots = 0;
    const c = capture(() => ({ size: ++screenshots, type: 'image/png' }));
    const deliveries = [];
    const first = (request, blob) => deliveries.push(['first', request.sequence, blob]);
    const second = { handleCapture(request, blob) { assert.equal(this, second); deliveries.push(['second', request.sequence, blob]); } };
    c.bridge.setHost(first);
    c.bridge.handleRequest({ frame: 8, sequence: 2 });
    c.bridge.handleRequest({ frame: 8, sequence: 1 });
    c.bridge.setHost(second);
    assert.equal(c.bridge.getHost(), second);
    c.bridge.handleRequest({ frame: 8, sequence: 0, inputTick: 12.8, name: ' a/b.PNG ' });
    c.render();
    await c.bridge.flush();
    assert.equal(screenshots, 2);
    assert.deepEqual(deliveries.map(([host, sequence]) => [host, sequence]), [['first', 1], ['first', 2], ['second', 0]]);
    assert.equal(deliveries[0][2], deliveries[1][2]);
    assert.notEqual(deliveries[1][2], deliveries[2][2]);
    assert.deepEqual(c.events.map(event => event.filename), ['frame_0008_0001.png', 'frame_0008_0002.png', '0012_0000_a_b.png']);
    assert.ok(c.events.every(event => event.ok && event.destination === 'host'));
    await c.bridge.flush();
});

test('flush waits for host persistence and includes requests queued during delivery', async () => {
    const c = capture(() => ({ size: 1 }));
    let complete;
    const delivered = [];
    c.bridge.setHost(request => {
        delivered.push(request.sequence);
        if (request.sequence === 1) return new Promise(resolve => { complete = resolve; });
    });
    c.bridge.handleRequest({ sequence: 1 });
    let flushed = 0;
    const waiters = [c.bridge.flush().then(() => flushed++), c.bridge.flush().then(() => flushed++)];
    c.render();
    await turn();
    assert.equal(flushed, 0);
    assert.deepEqual(delivered, [1]);
    c.bridge.handleRequest({ sequence: 2 });
    c.render();
    complete();
    await Promise.all(waiters);
    assert.equal(flushed, 2);
    assert.deepEqual(delivered, [1, 2]);
});

test('capture and host failures notify their original host without poisoning the queue', async () => {
    let screenshots = 0;
    const c = capture(() => ++screenshots === 1 ? null : { size: 2 });
    const failures = [];
    c.bridge.setHost({
        handleCapture() { return Promise.reject('persistence failed'); },
        handleCaptureFailure(request, error) { failures.push([request.sequence, String(error)]); throw new Error('notification failed'); },
    });
    c.bridge.handleRequest({ sequence: 1 });
    c.bridge.handleRequest({ sequence: 2 });
    c.bridge.setHost(() => {});
    c.bridge.handleRequest({ sequence: 3 });
    c.render();
    await c.bridge.flush();
    assert.deepEqual(failures.map(([sequence]) => sequence), [1, 2]);
    assert.match(failures[0][1], /failed to capture screenshot/);
    assert.equal(failures[1][1], 'persistence failed');
    assert.deepEqual(c.events.map(event => event.ok), [false, false, true]);
    for (const invalid of [null, [], 'capture']) assert.throws(() => c.bridge.handleRequest(invalid), /must be an object/);
});

for (const completion of ['animation frame', 'visibility change', 'timeout']) {
    test(`render fence ${completion} cleans up competing callbacks and captures exactly once`, async () => {
        let screenshots = 0;
        const c = capture(() => ({ size: ++screenshots }));
        const listeners = new Map();
        let frameCallback;
        const cancelled = [];
        c.document.visibilityState = 'visible';
        c.document.addEventListener = (name, callback) => listeners.set(name, callback);
        c.document.removeEventListener = name => listeners.delete(name);
        c.window.requestAnimationFrame = callback => { frameCallback = callback; return 42; };
        c.window.cancelAnimationFrame = id => cancelled.push(id);
        c.bridge.setHost(() => {});
        c.bridge.handleRequest({ frame: 1 });
        await turn();
        assert.equal(screenshots, 0, 'capture waits until a fence completes');
        if (completion === 'animation frame') frameCallback();
        else if (completion === 'visibility change') {
            c.document.visibilityState = 'hidden';
            listeners.get('visibilitychange')();
        } else c.render();
        await c.bridge.flush();
        assert.equal(screenshots, 1);
        assert.equal(listeners.size, 0);
        assert.equal(c.timers.size, 0);
        assert.deepEqual(cancelled, [42]);
        frameCallback();
        await turn();
        assert.equal(screenshots, 1);
    });
}

test('download fallback revokes its URL after delivery even when parent event access fails', async () => {
    const blob = { size: 7, type: 'image/png' };
    const c = capture(() => blob);
    const actions = [];
    const anchor = { click() { actions.push(['click', this.href, this.download]); } };
    c.document.createElement = tag => { assert.equal(tag, 'a'); return anchor; };
    c.document.body = {
        appendChild: element => { assert.equal(element, anchor); actions.push('append'); },
        removeChild: element => { assert.equal(element, anchor); actions.push('remove'); },
    };
    c.context.URL = {
        createObjectURL: value => { assert.equal(value, blob); return 'blob:test'; },
        revokeObjectURL: url => actions.push(['revoke', url]),
    };
    c.context.setTimeout = c.window.setTimeout;
    c.window.parent = { dispatchEvent() { throw new Error('cross origin'); } };
    c.bridge.handleRequest({ frame: 2, sequence: 3 });
    c.render();
    await c.bridge.flush();
    assert.deepEqual(actions, ['append', ['click', 'blob:test', 'frame_0002_0003.png'], 'remove']);
    assert.equal(c.events.length, 1);
    assert.equal(c.events[0].destination, 'download');
    c.render();
    assert.deepEqual(actions.at(-1), ['revoke', 'blob:test']);
    assert.equal(c.timers.size, 0);
    const completed = actions.length;
    c.render();
    assert.equal(actions.length, completed, 'completed timeouts cannot run again');
});
