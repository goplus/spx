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

function newRunner(overrides = {}) {
    const context = {
        EnginePackMode: 'normal',
        LOG_LEVEL_VERBOSE: 1,
        profiler: {},
        WorkerMessageManager: class {},
        ...overrides,
    }
    context.window = context
    vm.runInNewContext(source, context)
    return { runner: new context.GameRunner(), context }
}

function newRecordingRunner() {
    const calls = []
    const replay = { data: 'recording' }
    const { runner, context } = newRunner({ Error, ispx_stop: () => { calls.push('stop') } })
    runner.engine = {}
    runner.finishInputRecording = () => {
        calls.push('recording')
        return replay
    }
    const lifecycle = runner.beginGameLifecycle({ mode: 'record' })
    return { runner, context, lifecycle, calls, replay }
}

test('finishes recording before the stop callback and waits for runtime exit', async () => {
    const { runner, lifecycle, calls, replay } = newRecordingRunner()
    const ready = deferred()
    let settled = false
    const stopped = runner.StopGame(async result => {
        assert.equal(result.inputReplay, replay)
        calls.push('callback')
        await ready.promise
    }).then(result => {
        settled = true
        return result
    })

    await new Promise(setImmediate)
    assert.deepEqual(calls, ['recording', 'callback'])
    assert.equal(lifecycle.inputReplay, replay)
    ready.resolve()
    await new Promise(setImmediate)
    assert.deepEqual(calls, ['recording', 'callback', 'stop'])
    assert.equal(settled, false)

    runner.completeGameLifecycle(0)
    const result = await stopped
    assert.equal(result.inputReplay, replay)
    assert.equal(result.stopped, true)
    assert.equal(runner.pendingStops, 0)
})

test('stops after a recording failure and keeps it ahead of a returned stop error', async () => {
    const { runner, context, calls } = newRecordingRunner()
    const error = new Error('recording failed')
    runner.finishInputRecording = () => { throw error }
    context.ispx_stop = () => {
        calls.push('stop')
        return new Error('stop failed')
    }

    await assert.rejects(runner.StopGame(() => calls.push('callback')), actual => actual === error)
    assert.deepEqual(calls, ['stop'])
    assert.equal(runner.pendingStops, 0)
})

test('stops after a callback failure and keeps it ahead of an exit rejection', async () => {
    const { runner, lifecycle, calls } = newRecordingRunner()
    const exit = deferred()
    lifecycle.exit = exit.promise
    const error = new Error('callback failed')
    const rejected = assert.rejects(runner.StopGame(() => { throw error }), actual => actual === error)

    await new Promise(setImmediate)
    assert.deepEqual(calls, ['recording', 'stop'])
    exit.reject(new Error('exit failed'))
    await rejected
    assert.equal(runner.pendingStops, 0)
})

test('reports a returned stop error without waiting for runtime exit', async () => {
    const { runner, context, lifecycle } = newRecordingRunner()
    const error = new Error('stop failed')
    context.ispx_stop = () => error

    await assert.rejects(runner.StopGame(), actual => actual === error)
    assert.equal(lifecycle.completed, false)
    assert.equal(runner.pendingStops, 0)
})

test('propagates a thrown stop error and releases the pending count', async () => {
    const { runner, context, calls } = newRecordingRunner()
    const error = new Error('stop threw')
    runner.finishInputRecording = () => { throw new Error('recording failed') }
    runner.recordingOnGameStart = true
    runner.autoDownloadRecordedVideo = true
    runner.downloadRecordedVideo = () => calls.push('download')
    context.ispx_stop = () => { throw error }

    await assert.rejects(runner.StopGame(() => calls.push('callback')), actual => actual === error)
    assert.deepEqual(calls, [])
    assert.equal(runner.pendingStops, 0)

    context.ispx_stop = () => runner.completeGameLifecycle(0)
    assert.equal((await runner.ResetGame()).stopped, true)
    assert.deepEqual(calls, ['download'])
    assert.equal(runner.pendingStops, 0)
})

test('reports an exit rejection when recording and the callback succeed', async () => {
    const { runner, lifecycle, calls } = newRecordingRunner()
    const exit = deferred()
    lifecycle.exit = exit.promise
    const error = new Error('exit failed')
    const rejected = assert.rejects(runner.StopGame(() => calls.push('callback')), actual => actual === error)

    await new Promise(setImmediate)
    assert.deepEqual(calls, ['recording', 'callback', 'stop'])
    exit.reject(error)
    await rejected
    assert.equal(runner.pendingStops, 0)
})

test('rejects invalid stop callbacks without queuing a stop', async () => {
    const { runner, calls } = newRecordingRunner()
    const stopped = runner.StopGame('invalid')
    assert.equal(typeof stopped.then, 'function')
    assert.equal(runner.pendingStops, 0)
    await assert.rejects(stopped, /beforeStop must be a function/)
    assert.deepEqual(calls, [])
})

test('resets without finishing input recording', async () => {
    const { runner, calls } = newRecordingRunner()
    const stopped = runner.ResetGame()
    await new Promise(setImmediate)
    assert.deepEqual(calls, ['stop'])
    runner.completeGameLifecycle(0)
    const result = await stopped
    assert.equal(result.inputReplay, null)
    assert.equal(result.stopped, true)
    assert.equal(runner.pendingStops, 0)
})

test('completes a lifecycle once and reuses its recording when already stopped', async () => {
    const { runner, lifecycle, calls, replay } = newRecordingRunner()
    lifecycle.inputReplay = replay
    assert.equal(runner.completeGameLifecycle(3), true)
    assert.equal(runner.completeGameLifecycle(9), false)
    assert.equal(await lifecycle.exit, 3)

    const result = await runner.StopGame(() => calls.push('callback'))
    assert.equal(result.inputReplay, replay)
    assert.equal(result.stopped, false)
    assert.deepEqual(calls, [])
    assert.equal(runner.pendingStops, 0)
})

test('starts after a queued stop exits and releases its pending count', async () => {
    const { runner, context, calls } = newRecordingRunner()
    context.profiler = { profile: (_, fn) => fn(), mark() {}, measure() {} }
    runner.canvas = { focus() {} }
    runner.restart = () => calls.push('restart')
    runner.runProject = async () => {
        assert.equal(runner.pendingStops, 0)
        calls.push('start')
    }
    const stopped = runner.StopGame()
    const started = runner.StartGame()

    await new Promise(setImmediate)
    assert.deepEqual(calls, ['recording', 'stop'])
    runner.completeGameLifecycle(0)
    await Promise.all([stopped, started])

    assert.deepEqual(calls, ['recording', 'stop', 'restart', 'start'])
    assert.equal(runner.pendingStops, 0)
})

const controls = {
    restart: '_gdspx_ext_request_restart',
    pause: '_gdspx_ext_pause',
    resume: '_gdspx_ext_resume',
    stepNextFrame: '_gdspx_ext_next_frame',
    isPaused: '_gdspx_ext_is_paused',
}

test('calls runtime controls without a receiver and returns only pause status', () => {
    const { runner } = newRunner()
    const calls = []
    runner.engine = { rtenv: {} }
    for (const [method, name] of Object.entries(controls)) {
        runner.engine.rtenv[name] = function () {
            'use strict'
            assert.equal(this, undefined)
            calls.push(method)
            return 1
        }
        assert.equal(runner[method](), method === 'isPaused' ? true : undefined)
    }
    assert.deepEqual(calls, Object.keys(controls))
    runner.engine.rtenv._gdspx_ext_is_paused = () => 0
    assert.equal(runner.isPaused(), false)
})

test('ignores absent exports and propagates runtime failures', () => {
    const { runner } = newRunner()
    runner.engine = { rtenv: {} }
    const error = new Error('runtime failed')
    for (const [method, name] of Object.entries(controls)) {
        assert.equal(runner[method](), method === 'isPaused' ? false : undefined)
        runner.engine.rtenv[name] = () => { throw error }
        assert.throws(() => runner[method](), actual => actual === error)
        runner.engine.rtenv[name] = 1
        assert.throws(() => runner[method](), { name: 'TypeError' })
    }
})

test('guards pause queries but requires a runtime for other controls', () => {
    const { runner } = newRunner()
    for (const engine of [null, {}]) {
        runner.engine = engine
        assert.equal(runner.pause(), undefined)
        assert.equal(runner.isPaused(), false)
        for (const method of ['restart', 'resume', 'stepNextFrame']) {
            assert.throws(() => runner[method](), { name: 'TypeError' })
        }
    }
})

test('skips queued tasks after a rejection and allows retry', async () => {
    const { runner } = newRunner()
    const calls = []
    const error = new Error('first task failed')
    let reject
    const pending = new Promise((_, fail) => { reject = fail })
    const first = runner.enqueue(() => {
        calls.push('first')
        return pending
    })
    const second = runner.enqueue(() => {
        calls.push('second')
        return 2
    })
    const third = runner.enqueue(() => {
        calls.push('third')
        return 3
    })
    const settled = Promise.allSettled([first, second, third])

    await new Promise(setImmediate)
    assert.deepEqual(calls, ['first'])
    reject(error)
    assert.deepEqual(await settled, [
        { status: 'rejected', reason: error },
        { status: 'rejected', reason: error },
        { status: 'rejected', reason: error },
    ])
    assert.equal(await runner.enqueue(() => {
        calls.push('fourth')
        return 4
    }), 4)
    assert.deepEqual(calls, ['first', 'fourth'])
})

test('recovers after repeated synchronous and asynchronous failures', async () => {
    const { runner } = newRunner()
    const syncError = new Error('sync failure')
    const asyncError = new Error('async failure')
    const first = runner.enqueue(() => { throw syncError })
    await assert.rejects(first, error => error === syncError)
    const second = runner.enqueue(() => Promise.reject(asyncError))
    await assert.rejects(second, error => error === asyncError)
    assert.equal(await runner.enqueue(() => 'done'), 'done')
})

test('allows immediate retry from a public failure handler after queued tasks are skipped', async () => {
    const { runner } = newRunner()
    const error = new Error('initial failure')
    let calls = 0
    runner.initGame = async () => {
        if (++calls === 1) throw error
        return 'ready'
    }
    const first = runner.InitGame({})
    const queued = runner.InitGame({})
    const retry = first.catch(() => runner.InitGame({}))

    assert.deepEqual(await Promise.allSettled([first, queued, retry]), [
        { status: 'rejected', reason: error },
        { status: 'rejected', reason: error },
        { status: 'fulfilled', value: 'ready' },
    ])
    assert.equal(calls, 2)
})

test('waits for successful asynchronous tasks and preserves their results', async () => {
    const { runner } = newRunner()
    const value = { ready: true }
    let resolve
    const pending = new Promise(done => { resolve = done })
    const first = runner.enqueue(() => pending)
    let secondStarted = false
    const second = runner.enqueue(() => {
        secondStarted = true
        return 2
    })

    await new Promise(setImmediate)
    assert.equal(secondStarted, false)
    resolve(value)
    assert.deepEqual(await Promise.all([first, second]), [value, 2])
    assert.equal(secondStarted, true)
})
