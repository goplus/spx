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

function newApp(fetch, calls) {
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
    const game = new context.GameApp({
        assetURLs: {
            'engine.wasm': '/engine.wasm',
            'ispx.wasm': '/ispx.wasm',
            'engine.zip': '/engine.zip',
        },
    })
    game.loadLogicWasm = async response => {
        calls.push('logic')
        assert.equal((await response).url, '/ispx.wasm')
    }
    game.runLogicWasm = async () => {}
    game.onRunAfterInit = async () => {}
    return game
}

function startApp(queued = false) {
    const engine = deferred()
    const pack = deferred()
    const calls = []
    const game = newApp(url => {
        calls.push(url)
        if (url === '/engine.wasm') return engine.promise
        if (url === '/engine.zip') return Promise.resolve({ arrayBuffer: () => pack.promise })
        return Promise.resolve({ url })
    }, calls)
    return { engine, pack, calls, game, started: queued ? game.InitEngine() : game.initEngine() }
}

test('starts all downloads before engine initialization and waits for the pack', async () => {
    const { engine, pack, calls, started } = startApp()
    assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])
    engine.resolve({ arrayBuffer: async () => 'engine bytes' })
    await new Promise(setImmediate)
    assert.deepEqual(calls.slice(3), ['logic', 'init'])
    pack.resolve('pack bytes')
    await started
    assert.deepEqual(calls.slice(3), ['logic', 'init', 'unpack', 'start'])
})

test('reports an early pack download failure when unpacking', async () => {
    const { engine, pack, started } = startApp()
    pack.reject(new Error('pack failed'))
    await new Promise(setImmediate)
    engine.resolve({ arrayBuffer: async () => 'engine bytes' })
    await assert.rejects(started, /pack failed/)
})

test('skips downloads for a stopped or running app', async () => {
    const calls = []
    const game = newApp(url => calls.push(url), [])
    game.stopGameTask = 1
    await game.initEngine()
    game.stopGameTask = 0
    game.game = {}
    await game.initEngine()
    assert.deepEqual(calls, [])
})

for (const stop of ['StopGame', 'ResetGame']) {
    test(`does not initialize after ${stop} during engine download`, async () => {
        const { engine, pack, calls, game, started } = startApp(true)
        await new Promise(setImmediate)
        assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])

        const stopped = game[stop]()
        pack.resolve('pack bytes')
        engine.resolve({ arrayBuffer: async () => 'engine bytes' })
        await Promise.all([started, stopped])

        assert.deepEqual(calls, ['/engine.wasm', '/ispx.wasm', '/engine.zip'])
        assert.equal(game.game, null)
        assert.equal(game.stopGameTask, 0)
    })
}
