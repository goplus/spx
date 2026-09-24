const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const test = require('node:test')
const vm = require('node:vm')

const root = path.join(__dirname, '../../..')
const source = name => readFileSync(path.join(root, name), 'utf8')
const engineSource = source('godot_modules/spx/web/js/engine/engine.js')
const bridgeSource = source('godot_modules/spx/web/js/libs/library_godot_gdspx.js')
const gameSource = source('cmd/ispx/web/game.js')

function file(lastModified, text) {
    return { lastModified, content: Uint8Array.from(Buffer.from(text)).buffer }
}

function newApp() {
    const files = new Map()
    const writes = []
    const deleted = []
    const fs = {
        stat(name) {
            if (!files.has(name)) throw Object.assign(new Error('missing file'), { errno: 44 })
            return { mode: 'file' }
        },
        isDir: mode => mode === 'directory',
        unlink(name) {
            files.delete(name)
            deleted.push(name)
        },
    }
    const context = vm.createContext({
        console: { error() {} },
        Preloader: class {},
        InternalConfig: class {},
        Features: {},
        EnginePackMode: 'normal',
        LOG_LEVEL_VERBOSE: 1,
        WorkerMessageManager: class {},
        FS: fs,
        GodotFS: { ENOENT: 44 },
        GodotRuntime: { error() {} },
        autoAddDeps() {},
        mergeInto: Object.assign,
        LibraryManager: { library: {} },
    })
    context.window = context
    vm.runInContext(bridgeSource, context)
    vm.runInContext('Object.assign(GodotGdspx, GodotGdspx.$GodotGdspx)', context)
    vm.runInContext(engineSource, context)
    vm.runInContext(gameSource, context)

    const runtime = {
        copyToFS(name, data) {
            files.set(name, Buffer.from(data).toString())
            writes.push(name)
        },
        deleteDirRecursive: vm.runInContext('GodotGdspx.removeDirRecursive', context),
        updateGameDatas: vm.runInContext('GodotGdspx.updateGameDatas', context),
    }
    const app = new context.GameApp()
    app.game = new context.Engine()
    app.game.rtenv = runtime
    return { app, files, writes, deleted, fs, runtime }
}

test('syncs changed files, removes old files, and skips unchanged files and directories', () => {
    const { app, files, writes, deleted } = newApp()
    const keep = file(1, 'keep')
    app.updateEngineFiles({ keep, old: file(1, 'old') })
    writes.length = 0

    const next = { keep, added: file(2, 'new'), 'directory/': file(2, '') }
    app.updateEngineFiles(next)
    app.updateEngineFiles(next)

    assert.deepEqual([...files], [['engine/keep', 'keep'], ['engine/added', 'new']])
    assert.deepEqual(writes, ['engine/added'])
    assert.deepEqual(deleted, ['engine/old'])
    assert.deepEqual(Object.keys(app.projectFilesMeta).sort(), ['added', 'keep'])
})

for (const method of ['copyToFS', 'updateGameDatas']) {
    test(`${method} failure rejects InitGame and retries the same timestamp`, async () => {
        const { app, files, writes, runtime } = newApp()
        const original = runtime[method]
        const error = new Error('asset sync failed')
        runtime[method] = (...args) => {
            original(...args)
            throw error
        }
        let builds = 0
        app.buildGame = () => { builds++ }
        const next = { sprite: file(1, 'new') }

        await assert.rejects(app.InitGame(next), error)
        assert.equal(builds, 0)
        const attempts = writes.length
        runtime[method] = original
        await app.InitGame(next)

        assert.equal(files.get('engine/sprite'), 'new')
        assert.equal(writes.length, attempts + 1)
        assert.equal(builds, 1)
        assert.equal(app.projectFilesMeta.sprite.lastModified, 1)
    })
}

test('retrying an older version after partial writes restores files and removes new paths', () => {
    const { app, files, runtime } = newApp()
    const previous = { sprite: file(1, 'old') }
    app.updateEngineFiles(previous)
    const copy = runtime.copyToFS
    const error = new Error('write failed')
    runtime.copyToFS = (name, data) => {
        copy(name, data)
        if (name === 'engine/broken') throw error
    }

    assert.throws(() => app.updateEngineFiles({
        sprite: file(2, 'changed'),
        added: file(2, 'new'),
        broken: file(2, 'partial'),
    }), error)
    runtime.copyToFS = copy
    app.updateEngineFiles(previous)

    assert.deepEqual([...files], [['engine/sprite', 'old']])
    assert.equal(app.projectFilesMeta.sprite.lastModified, 1)
})

test('retrying after partial deletion restores files with unchanged timestamps', () => {
    const { app, files, fs } = newApp()
    const previous = { first: file(1, 'first'), second: file(1, 'second') }
    app.updateEngineFiles(previous)
    const unlink = fs.unlink
    const error = Object.assign(new Error('delete failed'), { errno: 29 })
    fs.unlink = name => {
        unlink(name)
        if (name === 'engine/second') throw error
    }

    assert.throws(() => app.updateEngineFiles({}), error)
    fs.unlink = unlink
    app.updateEngineFiles(previous)

    assert.deepEqual([...files], [['engine/first', 'first'], ['engine/second', 'second']])
})

test('deleting an already missing file succeeds', () => {
    const { app, files } = newApp()
    app.updateEngineFiles({ sprite: file(1, 'old') })
    files.clear()
    app.updateEngineFiles({})
    assert.deepEqual(Object.keys(app.projectFilesMeta), [])
})

test('recursive deletion propagates child errors', () => {
    const { app, fs } = newApp()
    const error = Object.assign(new Error('permission denied'), { errno: 2 })
    fs.stat = name => ({ mode: name === 'engine/tree' ? 'directory' : 'file' })
    fs.readdir = () => ['.', '..', 'child']
    fs.unlink = () => { throw error }
    fs.rmdir = () => assert.fail('must stop after the child fails')

    assert.throws(() => app.game.deleteAssetsData('engine', ['tree']), error)
})
