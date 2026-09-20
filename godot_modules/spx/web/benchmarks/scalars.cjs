const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const {performance} = require('node:perf_hooks');

const directory = process.argv[2];
const iterations = 200000;
const samples = 7;
const cases = [
    ['float input', 1, api => api.gdspx_physics_set_global_gravity(9.81)],
    ['bool input', 1, api => api.gdspx_camera_set_camera_smoothing(true)],
    ['64-bit ID + float', 1, api => api.gdspx_sprite_set_rotation(1, 0x200000, 45.5)],
    ['64-bit ID + 4 floats', 1, api => api.gdspx_ui_set_range(1, 0x200000, 0, 100, 0.5, 25)],
    ['100 sprites: rotation + visibility', 200, api => {
        for (let sprite = 0; sprite < 100; sprite++) {
            api.gdspx_sprite_set_rotation(sprite, 0x200000, sprite * 0.5);
            api.gdspx_sprite_set_visible(sprite, 0x200000, (sprite & 1) === 0);
        }
    }],
];

function bridge(module, label) {
    const context = vm.createContext({Module: module, TextEncoder, TextDecoder, console});
    vm.runInContext(fs.readFileSync(path.join(directory, `${label}.util.js`), 'utf8'), context);
    vm.runInContext(fs.readFileSync(path.join(directory, `${label}.calls.js`), 'utf8'), context);
    return vm.runInContext('new GdspxFuncs()', context);
}

function verify(module, api) {
    for (const value of [0, -0, 1 / 3, -123.5, Infinity, -Infinity, NaN]) {
        api.gdspx_physics_set_global_gravity(value);
        assert.ok(Object.is(module._benchmark_float(), Math.fround(value)));
    }
    for (const value of [true, false]) {
        api.gdspx_camera_set_camera_smoothing(value);
        assert.equal(module._benchmark_bool(), Number(value));
    }
    for (const [low, high] of [[1, 0x200000], [0xffffffff, 0x7fffffff], [0, 0x80000000], [0xffffffff, 0xffffffff]]) {
        api.gdspx_sprite_set_rotation(low, high, 0.5);
        assert.equal(module._benchmark_id_low() >>> 0, low);
        assert.equal(module._benchmark_id_high() >>> 0, high);
    }
    assert.equal(module._benchmark_grow_heap(), 1);
    api.gdspx_ui_set_range(1, 0x200000, 0, 100, 0.5, 25);
    assert.equal(module._benchmark_float(), 125.5);
}

async function main() {
    const modules = {};
    const apis = {};
    for (const label of ['before', 'after']) {
        modules[label] = await require(path.join(directory, `${label}.cjs`))();
        apis[label] = bridge(modules[label], label);
        verify(modules[label], apis[label]);
        assert.ok(modules[label]._benchmark_allocations() > 0, "the allocation probe must observe pool initialization");
    }
    const output = {node: process.version, cpu: os.cpus()[0].model, platform: `${process.platform}/${process.arch}`,
        baseline: process.argv[3], candidate: process.argv[4], samples, callsPerSample: iterations, cases: []};
    for (const [name, callsPerIteration, run] of cases) {
        const loops = iterations / callsPerIteration;
        const result = {name, callsPerIteration};
        const timings = {before: [], after: []};
        for (const label of ['before', 'after']) {
            for (let i = 0; i < Math.min(loops, 20000); i++) run(apis[label]);
        }
        // Alternate order to reduce warm-up and thermal ordering bias.
        for (let sample = 0; sample < samples; sample++) {
            for (const label of sample % 2 ? ['after', 'before'] : ['before', 'after']) {
                const allocations = modules[label]._benchmark_allocations();
                const start = performance.now();
                for (let i = 0; i < loops; i++) run(apis[label]);
                timings[label].push(performance.now() - start);
                assert.equal(modules[label]._benchmark_allocations(), allocations, 'warm scalar calls must not allocate heap storage');
            }
        }
        for (const label of ['before', 'after']) {
            const counts = {wasmCalls: 0, wrapperAcquires: 0, wrapperReleases: 0};
            const instrumented = {...modules[label]};
            for (const [key, fn] of Object.entries(instrumented)) {
                if (!key.startsWith('_gdspx_') || typeof fn !== 'function') continue;
                instrumented[key] = (...args) => {
                    counts.wasmCalls++;
                    if (/^_gdspx_(new|alloc)_/.test(key)) counts.wrapperAcquires++;
                    if (/^_gdspx_free_/.test(key)) counts.wrapperReleases++;
                    return fn(...args);
                };
            }
            run(bridge(instrumented, label));
            const times = timings[label].sort((a, b) => a - b);
            result[label] = {...counts, medianMs: times[Math.floor(samples / 2)],
                minMs: times[0], maxMs: times[samples - 1], heapAllocations: 0};
            result[label].millionCallsPerSecond = iterations / result[label].medianMs / 1000;
        }
        result.speedup = result.before.medianMs / result.after.medianMs;
        output.cases.push(result);
    }
    console.log(JSON.stringify(output, null, 2));
}
main().catch(error => { console.error(error); process.exitCode = 1; });
