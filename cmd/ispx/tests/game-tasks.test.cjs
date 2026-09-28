const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const test = require('node:test')
const vm = require('node:vm')

const context = {
    EnginePackMode: 'normal',
    LOG_LEVEL_VERBOSE: 1,
    profiler: {},
    WorkerMessageManager: class {},
}
context.window = context
vm.runInNewContext(readFileSync(path.join(__dirname, '../web/game.js'), 'utf8'), context)

test('skips queued tasks after a rejection and allows retry', async () => {
    const runner = new context.GameRunner()
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
    const runner = new context.GameRunner()
    const syncError = new Error('sync failure')
    const asyncError = new Error('async failure')
    const first = runner.enqueue(() => { throw syncError })
    await assert.rejects(first, error => error === syncError)
    const second = runner.enqueue(() => Promise.reject(asyncError))
    await assert.rejects(second, error => error === asyncError)
    assert.equal(await runner.enqueue(() => 'done'), 'done')
})

test('allows immediate retry from a public failure handler after queued tasks are skipped', async () => {
    const runner = new context.GameRunner()
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
    const runner = new context.GameRunner()
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
