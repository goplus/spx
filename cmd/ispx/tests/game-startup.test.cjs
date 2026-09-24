const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const test = require('node:test')
const vm = require('node:vm')

const source = readFileSync(path.join(__dirname, '../web/game.js'), 'utf8')

function deferred() {
    let resolve
    let reject
    const promise = new Promise((res, rej) => {
        resolve = res
        reject = rej
    })
    return { promise, resolve, reject }
}

function newRunner(fetch, calls) {
    const context = {
        fetch,
        EnginePackMode: 'normal',
        LOG_LEVEL_VERBOSE: 1,
        isWasmCompressed: false,
        profiler: { profile: (_, fn) => fn() },
        WorkerMessageManager: class {},
        GdspxFuncs: class {},
        Engine: class {
            async init() { calls.push('init') }
            async unpackEngineData(_, __, data) {
                calls.push('unpack')
                assert.equal(data, 'pack bytes')
            }
            async start() { calls.push('start') }
        },
    }
    context.window = context
    context.self = context
    vm.runInNewContext(source, context)
    const runner = new context.GameRunner({
        assetURLs: {
            'engine.wasm': '/engine.wasm',
            'ispx.wasm': '/ispx.wasm',
            'engine.zip': '/engine.zip',
        },
    })
    runner.loadLogicWasm = async response => {
        calls.push('logic')
        assert.equal((await response).url, '/ispx.wasm')
    }
    runner.runLogicWasm = () => {}
    return runner
}

function startRunner(queued = false) {
    const engine = deferred()
    const pack = deferred()
    const calls = []
    const runner = newRunner(url => {
        calls.push(url)
        if (url === '/engine.wasm') return engine.promise
        if (url === '/engine.zip') return Promise.resolve({ arrayBuffer: () => pack.promise })
        return Promise.resolve({ url })
    }, calls)
    return { engine, pack, calls, runner, started: queued ? runner.InitEngine() : runner.initEngine() }
}

test('starts all downloads before engine initialization and waits for the pack', async () => {
    const { engine, pack, calls, started } = startRunner()
    assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])
    engine.resolve({ arrayBuffer: async () => 'engine bytes' })
    await new Promise(setImmediate)
    assert.deepEqual(calls.slice(3), ['logic', 'init'])
    pack.resolve('pack bytes')
    await started
    assert.deepEqual(calls.slice(3), ['logic', 'init', 'unpack', 'start'])
})

test('reports an early pack download failure when unpacking', async () => {
    const { engine, pack, started } = startRunner()
    pack.reject(new Error('pack failed'))
    await new Promise(setImmediate)
    engine.resolve({ arrayBuffer: async () => 'engine bytes' })
    await assert.rejects(started, /pack failed/)
})

test('skips downloads for a stopped or running runner', async () => {
    const calls = []
    const runner = newRunner(url => calls.push(url), [])
    runner.pendingStops = 1
    await runner.initEngine()
    runner.pendingStops = 0
    runner.engine = {}
    await runner.initEngine()
    assert.deepEqual(calls, [])
})

for (const stop of ['StopGame', 'ResetGame']) {
    test(`does not initialize after ${stop} during engine download`, async () => {
        const { engine, pack, calls, runner, started } = startRunner(true)
        await new Promise(setImmediate)
        assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])

        const stopped = runner[stop]()
        pack.resolve('pack bytes')
        engine.resolve({ arrayBuffer: async () => 'engine bytes' })
        await Promise.all([started, stopped])

        assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])
        assert.equal(runner.engine, null)
        assert.equal(runner.pendingStops, 0)
    })
}

test('retries engine initialization after failed downloads skip queued stops', async () => {
    const engine = deferred()
    const calls = []
    let attempts = 0
    const runner = newRunner(url => {
        calls.push(url)
        if (url === '/engine.wasm') {
            return ++attempts === 1 ? engine.promise
                : Promise.resolve({ arrayBuffer: async () => 'engine bytes' })
        }
        if (url === '/engine.zip') return Promise.resolve({ arrayBuffer: async () => 'pack bytes' })
        return Promise.resolve({ url })
    }, calls)
    const started = runner.InitEngine()
    await new Promise(setImmediate)
    const stopped = runner.StopGame()
    const reset = runner.ResetGame()
    const settled = Promise.allSettled([started, stopped, reset])
    const error = new Error('engine download failed')
    engine.reject(error)

    assert.deepEqual(await settled, Array(3).fill({ status: 'rejected', reason: error }))
    assert.equal(runner.pendingStops, 0)
    await runner.InitEngine()
    assert.equal(attempts, 2)
    assert.deepEqual(calls.slice(3), [
        '/engine.wasm', '/ispx.wasm', '/engine.zip', 'logic', 'init', 'unpack', 'start',
    ])
})
