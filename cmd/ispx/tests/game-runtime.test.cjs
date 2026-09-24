const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const test = require('node:test')
const vm = require('node:vm')

const source = readFileSync(path.join(__dirname, '../web/game.js'), 'utf8')
const bindings = readFileSync(path.join(__dirname, '../../../godot_modules/spx/web/js/engine/gdspx.js'), 'utf8')

function newRunner(mode) {
    const calls = []
    const downloads = []
    const progress = []
    const threads = {}
    const exports = {}
    const extension = () => {}
    const runtime = {
        _gdspx_ext_request_restart() { calls.push('restart') },
        _gdspx_audio_create_audio() {},
    }
    function checkBindings() {
        assert.equal(typeof context.go_wasm_init, 'function')
        assert.equal(typeof context.gdspx_dispatch, 'function')
        assert.equal(typeof context.gdspx_audio_create_audio, 'function')
    }
    const context = {
        EnginePackMode: mode,
        LOG_LEVEL_VERBOSE: 1,
        isWasmCompressed: true,
        profiler: { profile: (_, fn) => fn(), mark() {}, measure() {} },
        GameGlobal: {},
        initExtensionWasm: extension,
        godotSdk: {
            set_engine(engine) {
                checkBindings()
                assert.equal(context.GameGlobal.engine, engine)
                calls.push('sdk')
            },
        },
        fetch: async url => {
            downloads.push(url)
            return { url, arrayBuffer: async () => url }
        },
        Engine: class {
            constructor(config) {
                calls.push('engine.create')
                assert.equal(config.wasmEngine, '/engine.wasm.br')
                this.rtenv = runtime
            }
            async init() {
                checkBindings()
                assert.equal(context.initExtensionWasm === extension, mode === 'worker')
                calls.push('engine.init')
            }
            async unpackEngineData(directory, name, data) {
                assert.equal(directory, 'engine')
                assert.equal(name, 'engine.zip')
                assert.equal(data, '/engine.zip')
                calls.push('engine.unpack')
            }
            async start({ args, canvas }) {
                assert.deepEqual(Array.from(args), ['--main-pack', 'engine/engine.zip', '--write-movie', 'engine/movie.avi'])
                assert.equal(canvas, runner.canvas)
                calls.push('engine.start')
            }
            getPThread() {
                calls.push('threads')
                return threads
            }
        },
        WorkerMessageManager: class {
            bindMainThreadCallbacks(engine) {
                assert.equal(engine, runner.engine)
                calls.push('callbacks')
            }
            setPThreads(value) {
                assert.equal(value, threads)
                calls.push('bindThreads')
            }
            callWorkerProjectDataUpdate(files, urls) {
                assert.equal(files, runner.nonAssetFiles)
                assert.equal(urls, runner.assetURLs)
                calls.push('project.update')
            }
        },
        Go: class {
            constructor() {
                checkBindings()
                this.importObject = {}
            }
            run(instance) {
                assert.equal(instance.exports, exports)
                calls.push('logic.run')
                return new Promise(() => {})
            }
        },
        WebAssembly: {
            Instance: class {},
            async instantiate(url, imports) {
                assert.equal(url, '/ispx.wasm.br')
                assert.equal(imports, runner.go.importObject)
                calls.push('logic.load')
                return { instance: { exports } }
            },
            async instantiateStreaming(response, imports) {
                assert.equal((await response).url, '/ispx.wasm.br')
                assert.equal(imports, runner.go.importObject)
                calls.push('logic.load')
                return { instance: { exports } }
            },
        },
        ispx_start(input) {
            assert.equal(input, null)
            assert.equal(context.Module, runtime)
            assert.equal(vm.runInContext('FFI === self', context), true)
            assert.equal(runner.gameLifecycle.completed, false)
            calls.push('project.start')
        },
        AllocGdObj: () => 1,
        ToJsObj: (_, result) => result,
        FreeGdObj() {},
    }
    context.window = context
    context.self = context
    vm.createContext(context)
    vm.runInContext(bindings, context)
    vm.runInContext(source, context)
    const runner = new context.GameRunner({
        gameCanvas: { focus: () => calls.push('focus') },
        recordingOnGameStart: true,
        onProgress: value => progress.push(value),
        assetURLs: {
            'engine.wasm': '/engine.wasm',
            'ispx.wasm': '/ispx.wasm',
            'engine.zip': '/engine.zip',
        },
    })
    runner.nonAssetFiles = {}
    return { runner, context, calls, downloads, progress }
}

for (const mode of ['normal', 'miniprogram', 'worker', 'minigame']) {
    test(`preserves runtime bootstrap and project startup in ${mode} mode`, async () => {
        const { runner, context, calls, downloads, progress } = newRunner(mode)
        await runner.InitEngine()
        const prepare = mode === 'minigame' ? ['sdk']
            : mode === 'worker' ? [] : ['logic.load', 'logic.run']
        const initialize = mode === 'minigame' ? ['logic.load']
            : mode === 'worker' ? ['callbacks'] : []
        assert.deepEqual(calls, [
            'engine.create', ...prepare, 'engine.init', 'engine.unpack', ...initialize, 'engine.start',
        ])
        const wasm = mode === 'minigame' ? [] : ['/engine.wasm.br']
        if (mode === 'normal' || mode === 'miniprogram') wasm.push('/ispx.wasm.br')
        assert.deepEqual(downloads, [...wasm, '/engine.zip'])
        assert.deepEqual(progress, [0.5, 0.5, 0.6, 0.7, 0.8, 1])

        calls.length = 0
        await runner.StartGame()
        const startup = mode === 'worker' ? ['threads', 'bindThreads', 'project.update']
            : mode === 'minigame' ? ['logic.run', 'project.start'] : ['project.start']
        assert.deepEqual(calls, ['restart', ...startup, 'focus'])
        if (mode !== 'worker') {
            const createAudio = context.gdspx_audio_create_audio
            assert.deepEqual({ ...createAudio() }, { low: 0, high: 0 })
        }
    })
}

for (const mode of ['normal', 'minigame']) {
    test(`reports logic startup failure as a rejection in ${mode} mode`, async () => {
        const { runner, context } = newRunner(mode)
        const error = new Error('logic startup failed')
        context.Go.prototype.run = () => { throw error }
        if (mode === 'normal') {
            await assert.rejects(runner.InitEngine(), actual => actual === error)
        } else {
            await runner.InitEngine()
            await assert.rejects(runner.StartGame(), actual => actual === error)
            assert.equal(runner.gameLifecycle.completed, true)
        }
    })
}
