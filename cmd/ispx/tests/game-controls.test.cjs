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

const controls = {
    restart: '_gdspx_ext_request_restart',
    pause: '_gdspx_ext_pause',
    resume: '_gdspx_ext_resume',
    stepNextFrame: '_gdspx_ext_next_frame',
    isPaused: '_gdspx_ext_is_paused',
}

test('calls runtime controls without a receiver and returns only pause status', () => {
    const runner = new context.GameRunner()
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
    const runner = new context.GameRunner()
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
    const runner = new context.GameRunner()
    for (const engine of [null, {}]) {
        runner.engine = engine
        assert.equal(runner.pause(), undefined)
        assert.equal(runner.isPaused(), false)
        for (const method of ['restart', 'resume', 'stepNextFrame']) {
            assert.throws(() => runner[method](), { name: 'TypeError' })
        }
    }
})
