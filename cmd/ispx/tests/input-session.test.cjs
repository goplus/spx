const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function session() {
    const context = vm.createContext({ window: {}, TextEncoder, Blob });
    vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/input-session.js'), 'utf8'), context);
    return { window: context.window, api: vm.runInContext('spxInputSession', context) };
}

const plain = value => JSON.parse(JSON.stringify(value));

test('runner configuration merges immutable defaults and rejects invalid updates atomically', () => {
    const { window } = session();
    const original = window.spxRunnerConfig;
    const overrides = { input: { defaultMode: 'record', record: { fps: 60, captureKey: null } } };
    const updated = window.spxConfigureRunner(overrides);
    overrides.input.record.fps = 1;
    assert.equal(original.input.record.fps, 30);
    assert.equal(updated.input.record.fps, 60);
    assert.equal(updated.input.replay.captureKey, 'P');
    for (const value of [updated, updated.input, updated.input.record, updated.input.replay, updated.limits]) {
        assert.equal(Object.isFrozen(value), true);
    }
    for (const invalid of [
        [], { limits: {} }, { input: null }, { input: { typo: true } },
        { input: { defaultMode: 'invalid' } }, { input: { record: { fps: 0 } } },
        { input: { record: { fps: Infinity } } }, { input: { replay: { captureKey: '' } } },
    ]) {
        assert.throws(() => window.spxConfigureRunner(invalid));
        assert.equal(window.spxRunnerConfig, updated);
    }
    assert.deepEqual(plain(window.spxCreateInputSession()), { mode: 'record', fps: 60, captureKey: null });
    assert.equal(window.spxCreateInputSession(null), null);
    assert.throws(() => window.spxCreateInputSession('normal', { fps: 30 }), /does not support fps/);
    assert.throws(() => window.spxCreateInputSession('replay', { fps: 30 }), /does not support fps/);
});

test('normalizing replay inputs copies only selected bytes and leaves caller options intact', async () => {
    const { api, window } = session();
    const backing = new Uint8Array([99, 1, 2, 88]);
    const options = { label: 'game', input: { mode: 'replay', data: new DataView(backing.buffer, 1, 2) } };
    const normalized = await api.normalizeStartOptions(options);
    assert.equal(normalized.label, 'game');
    assert.notEqual(normalized, options);
    assert.notEqual(normalized.input, options.input);
    assert.deepEqual(Array.from(normalized.input.data), [1, 2]);
    backing.fill(0);
    assert.deepEqual(Array.from(normalized.input.data), [1, 2]);
    window.spxConfigureRunner({ input: { defaultMode: 'record' } });
    assert.equal((await api.normalizeStartOptions({ input: null })).input, null);
    assert.equal((await api.normalizeStartOptions()).input.mode, 'record');
    for (const data of ['雪', { version: 1 }, new Blob(['雪']), new TextEncoder().encode('雪').buffer]) {
        const result = await api.normalizeStartOptions({ input: { mode: 'replay', data } });
        const expected = data && data.version ? '{"version":1}' : '雪';
        assert.equal(new TextDecoder().decode(result.input.data), expected);
    }
    const blob = api.replayBlob(normalized.input.data);
    assert.equal(blob.type, 'application/json');
    assert.deepEqual(Array.from(new Uint8Array(await blob.arrayBuffer())), [1, 2]);
    assert.equal(api.replayBlob(null), null);
});

test('replay validation rejects oversized inputs both before and after asynchronous reads', async () => {
    const { api, window } = session();
    const limit = window.spxRunnerConfig.limits.inputReplayBytes;
    const normalize = data => api.normalizeStartOptions({ input: { mode: 'replay', data } });
    await assert.rejects(normalize({ size: limit + 1, arrayBuffer() { assert.fail('oversized source was read'); } }), /exceeds/);
    await assert.rejects(normalize({ size: 1, async arrayBuffer() { return new ArrayBuffer(limit + 1); } }), /exceeds/);
    for (const data of [null, undefined, 42, true, { toJSON: () => undefined }]) {
        await assert.rejects(normalize(data), /input replay/);
    }
    const failure = new Error('read failed');
    await assert.rejects(normalize({ arrayBuffer: async () => { throw failure; } }), error => error === failure);
    await assert.rejects(api.normalizeStartOptions([]), /must be an object/);
});
