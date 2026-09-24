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

function newRunner() {
    const calls = []
    const replay = { data: 'recording' }
    const context = {
        Error,
        EnginePackMode: 'normal',
        LOG_LEVEL_VERBOSE: 1,
        profiler: {},
        WorkerMessageManager: class {},
        ispx_stop: () => { calls.push('stop') },
    }
    context.window = context
    vm.runInNewContext(source, context)
    const runner = new context.GameRunner()
    runner.engine = {}
    runner.finishInputRecording = () => {
        calls.push('recording')
        return replay
    }
    const lifecycle = runner.beginGameLifecycle({ mode: 'record' })
    return { runner, context, lifecycle, calls, replay }
}

test('finishes recording before the stop callback and waits for runtime exit', async () => {
    const { runner, lifecycle, calls, replay } = newRunner()
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
    const { runner, context, calls } = newRunner()
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
    const { runner, lifecycle, calls } = newRunner()
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
    const { runner, context, lifecycle } = newRunner()
    const error = new Error('stop failed')
    context.ispx_stop = () => error

    await assert.rejects(runner.StopGame(), actual => actual === error)
    assert.equal(lifecycle.completed, false)
    assert.equal(runner.pendingStops, 0)
})

test('propagates a thrown stop error and releases the pending count', async () => {
    const { runner, context, calls } = newRunner()
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
    const { runner, lifecycle, calls } = newRunner()
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
    const { runner, calls } = newRunner()
    const stopped = runner.StopGame('invalid')
    assert.equal(typeof stopped.then, 'function')
    assert.equal(runner.pendingStops, 0)
    await assert.rejects(stopped, /beforeStop must be a function/)
    assert.deepEqual(calls, [])
})

test('resets without finishing input recording', async () => {
    const { runner, calls } = newRunner()
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
    const { runner, lifecycle, calls, replay } = newRunner()
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
    const { runner, context, calls } = newRunner()
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
