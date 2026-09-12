const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.util.js'), 'utf8');

function bridge() {
    let offset = 64;
    const freed = [];
    const module = {};
    function heap(size) {
        const buffer = new ArrayBuffer(size);
        if (module.HEAPU8) new Uint8Array(buffer).set(module.HEAPU8);
        module.HEAPU8 = new Uint8Array(buffer);
        module.HEAPU32 = new Uint32Array(buffer);
        module.HEAPF32 = new Float32Array(buffer);
    }
    heap(4 * 1024 * 1024);
    module._cmalloc = (size) => {
        const ptr = offset;
        offset += Math.ceil(size / 8) * 8;
        if (offset > module.HEAPU8.length) heap(Math.max(offset * 2, module.HEAPU8.length * 2));
        return ptr;
    };
    module._cfree = ptr => freed.push(ptr);
    module._gdspx_borrow_array = (ptr, bytes, count, type) => {
        const info = module._cmalloc(12);
        module.HEAPU32.set([count, type, ptr], info / 4);
        module.lastBorrow = { ptr, bytes, count, type };
        return info;
    };
    module._gdspx_get_array_info = info => info;
    const context = vm.createContext({ Module: module, console, TextEncoder, TextDecoder, Uint8Array, DataView });
    vm.runInContext(source, context);
    return { context, module, freed, heap, run: text => vm.runInContext(text, context) };
}

test('all fixed-width types use the same native descriptor and borrow their input', () => {
    const b = bridge();
    for (const [type, size] of [[1, 8], [2, 4], [3, 1], [5, 1], [6, 8]]) {
        b.context.type = type;
        b.context.size = size;
        b.run('array = GdspxBorrowNativeArray(type, 2, size * 2); array.data[0] = 1; wrapper = ToGdArray(array);');
        assert.equal(b.module.lastBorrow.ptr, b.context.array.ptr);
        assert.equal(b.module.lastBorrow.count, 2);
        assert.equal(b.module.lastBorrow.type, type);
        assert.equal(b.module.lastBorrow.bytes, size * 2);
        const result = b.run('ToJsArray(wrapper)');
        assert.equal(result.type, type);
        assert.equal(result.count, 2);
        b.module.HEAPU8[b.context.array.ptr] = 0;
        assert.equal(result.data[0], 1, 'returned payload must survive native release/reuse');
    }
});

test('string inputs use offset/length data and string results use the same layout', () => {
    const b = bridge();
    const input = b.run('GdspxBorrowNativeArray(4, 2, 20)');
    input.data.set([16, 0, 0, 0, 2, 0, 0, 0, 19, 0, 0, 0, 0, 0, 0, 0, 104, 105, 0, 0]);
    b.context.array = input;
    b.run('ToGdArray(array)');
    assert.equal(b.module.lastBorrow.ptr, input.ptr);
    assert.equal(b.module.lastBorrow.type, 4);
    const strings = b.module._cmalloc(8);
    b.module.HEAPU32.set([input.ptr + 16, input.ptr + 19], strings / 4);
    const info = b.module._cmalloc(12);
    b.module.HEAPU32.set([2, 4, strings], info / 4);
    b.context.info = info;
    const output = b.run('ToJsArray(info)');
    assert.deepEqual(Array.from(output.data), Array.from(input.data));
    input.data.fill(0);
    assert.equal(output.data[16], 104);
});

test('arenas rotate without overwriting earlier arguments and refresh after memory growth', () => {
    const b = bridge();
    const first = b.run('GdspxBorrowNativeArray(5, 700000, 700000)');
    first.data[0] = 23;
    const second = b.run('GdspxBorrowNativeArray(5, 700000, 700000)');
    second.data[0] = 42;
    assert.notEqual(first.ptr, second.ptr);
    assert.equal(first.data[0], 23);
    assert.equal(b.freed.length, 0);
    const oldBuffer = first.data.buffer;
    b.heap(8 * 1024 * 1024);
    assert.notEqual(first.data.buffer, oldBuffer);
    assert.equal(first.data[0], 23);
    b.run('GdspxFlushDeferredFrees()');
    assert.equal(b.freed.length, 1);
});

test('empty arrays preserve their type and malformed descriptors are rejected', () => {
    const b = bridge();
    for (const type of [1, 2, 3, 4, 5, 6]) {
        b.context.type = type;
        const out = b.run('ToJsArray(ToGdArray(GdspxBorrowNativeArray(type, 0, 0)))');
        assert.equal(out.type, type);
        assert.equal(out.count, 0);
        assert.equal(out.data.length, 0);
    }
    assert.equal(b.run('GdspxBorrowNativeArray(4, 2, 16)'), null);
    assert.equal(b.run('GdspxBorrowNativeArray(99, 0, 0)'), null);
    assert.throws(() => b.run('ToGdArray(new Uint8Array(8))'), /requires a native array/);
    assert.equal(b.run('ToJsArray(0)'), null);
    const slots = b.module._cmalloc(4);
    const info = b.module._cmalloc(12);
    b.module.HEAPU32.set([1, 4, slots], info / 4);
    b.context.info = info;
    assert.equal(b.run('ToJsArray(info)'), null, 'invalid native results preserve the null result');
});

test('generated wrappers release every acquired argument and result on failure', () => {
    const generated = fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.js'), 'utf8');
    for (const failure of ['argument', 'call', 'result', null]) {
        const b = bridge();
        const allocated = [];
        const released = [];
        const allocate = () => {
            const ptr = b.module._cmalloc(8);
            allocated.push(ptr);
            return ptr;
        };
        let arrays = 0;
        Object.assign(b.context, {
            AllocGdString: allocate,
            ToGdString: allocate,
            ToGdArray: () => {
                if (++arrays === 2 && failure === 'argument') throw new Error('argument');
                return allocate();
            },
            ToJsString: () => {
                if (failure === 'result') throw new Error('result');
                return 'ok';
            },
            FreeGdArray: ptr => released.push(ptr),
            FreeGdString: ptr => released.push(ptr),
        });
        b.module._gdspx_res_apply_project_fonts = () => {
            if (failure === 'call') throw new Error('call');
        };
        b.run(generated);
        const call = () => b.run('new GdspxFuncs().gdspx_res_apply_project_fonts("", [], [], [])');
        if (failure) assert.throws(call, new RegExp(failure));
        else assert.equal(call(), 'ok');
        assert.deepEqual(released.sort((a, b) => a - b), allocated.sort((a, b) => a - b));
    }
});

test('fixed output allocation uses generated metadata and preserves in-place calls', () => {
    const b = bridge();
    b.run(fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.js'), 'utf8'));
    const calls = [];
    b.module._gdspx_input_write_snapshot = (ptr, count) => {
        calls.push({ ptr, count });
        if (count >= 3) b.module.HEAPF32.set([1.5, -2.5, 7], ptr / 4);
    };
    const snapshot = b.run("GdspxFuncs['arrayOutputs']['gdspx_input_write_snapshot']()");
    assert.equal(snapshot.type, 2);
    assert.equal(snapshot.count, 3);
    assert.equal(snapshot.data.length, 12);
    assert.deepEqual(Array.from(b.module.HEAPF32.subarray(snapshot.ptr / 4, snapshot.ptr / 4 + 3)), [1.5, -2.5, 7]);
    for (const count of [0, 2, 4]) {
        b.context.count = count;
        const out = b.run('out = GdspxBorrowNativeArray(2, count, count * 4)');
        b.module.HEAPF32.fill(42, out.ptr / 4, out.ptr / 4 + count);
        b.run('new GdspxFuncs().gdspx_input_write_snapshot(out)');
        assert.deepEqual(calls.at(-1), { ptr: out.ptr, count });
        const values = Array.from(b.module.HEAPF32.subarray(out.ptr / 4, out.ptr / 4 + count));
        assert.deepEqual(values, count < 3 ? Array(count).fill(42) : [1.5, -2.5, 7, 42]);
    }
    // A different type and count exercise the same allocator without a method-name branch.
    b.module._example = (ptr, count) => b.module.HEAPU8.fill(23, ptr, ptr + count);
    const bytes = b.run("ReadArrayOutput('_example', 5, 7)");
    assert.equal(bytes.type, 5);
    assert.equal(bytes.count, 7);
    assert.deepEqual(Array.from(bytes.data), Array(7).fill(23));
    assert.equal(b.run("ReadArrayOutput('_missing', 5, 7)"), null);
    delete b.module._gdspx_input_write_snapshot;
    assert.equal(b.run("GdspxFuncs['arrayOutputs']['gdspx_input_write_snapshot']()"), null);
});

test('structured results are fresh by default and reuse only an explicit destination', () => {
    const b = bridge();
    const ptr = b.module._cmalloc(16);
    b.context.ptr = ptr;
    b.module.HEAPF32.set([1, 2, 3, 4], ptr / 4);
    for (const [type, expected] of [
        ['Vec2', {x: 1, y: 2}],
        ['Vec3', {x: 1, y: 2, z: 3}],
        ['Vec4', {x: 1, y: 2, z: 3, w: 4}],
        ['Color', {r: 1, g: 2, b: 3, a: 4}],
        ['Rect2', {position: {x: 1, y: 2}, size: {x: 3, y: 4}}],
    ]) {
        const first = b.run(`ToJs${type}(ptr)`);
        assert.deepEqual(JSON.parse(JSON.stringify(first)), expected);
        assert.notEqual(b.run(`ToJs${type}(ptr)`), first);
        b.context.first = first;
        assert.equal(b.run(`ToJs${type}(ptr, first)`), first);
        if (type === 'Rect2') {
            const position = first.position;
            b.run('ToJsRect2(ptr, first)');
            assert.equal(first.position, position);
        }
    }
});

test('integer and object results preserve bits, reuse scope and heap growth', () => {
    const b = bridge();
    const ptr = b.module._cmalloc(8);
    b.context.ptr = ptr;
    b.module.HEAPU32.set([0x89abcdef, 0xfedcba98], ptr / 4);
    const first = b.run('ToJsInt(ptr)');
    assert.deepEqual({...first}, {low: 0x89abcdef, high: 0xfedcba98});
    assert.notEqual(b.run('ToJsInt(ptr)'), first);
    b.context.result = first;
    assert.equal(b.run('ToJsObj(ptr, result)'), first);
    b.heap(8 * 1024 * 1024);
    b.module.HEAPU32.set([0xffffffff, 0x80000000], ptr / 4);
    assert.equal(b.run('ToJsInt(ptr, result)'), first);
    assert.deepEqual({...first}, {low: 0xffffffff, high: 0x80000000});

    b.run(fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.js'), 'utf8'));
    const released = [];
    Object.assign(b.context, {
        AllocGdInt: () => ptr,
        FreeGdInt: value => released.push(value),
        AllocGdObj: () => ptr,
        FreeGdObj: value => released.push(value),
    });
    b.run('api = new GdspxFuncs(); other = new GdspxFuncs()');
    const names = ['gdspx_platform_get_max_fps', 'gdspx_audio_create_audio'];
    const results = [];
    for (const name of names) {
        b.module['_' + name] = () => {};
        b.context.name = name;
        const value = b.run('api[name]()');
        results.push(value);
        assert.deepEqual({...value}, {low: 0xffffffff, high: 0x80000000});
        assert.equal(b.run('api[name]()'), value);
        assert.notEqual(b.run('other[name]()'), value);
    }
    assert.notEqual(results[0], results[1], 'int and object results use separate slots');
    assert.equal(released.length, names.length * 3);
});

test('declared Web bindings preserve instance result identity and release on failure', () => {
    const b = bridge();
    b.run(fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.js'), 'utf8'));
    const released = [];
    Object.assign(b.context, {
        AllocGdVec2: () => b.module._cmalloc(8),
        FreeGdVec2: ptr => released.push(ptr),
    });
    let value = 0;
    b.module._gdspx_input_get_global_mouse_pos = ptr => b.module.HEAPF32.set([++value, 2], ptr / 4);
    b.module._gdspx_camera_get_camera_position = b.module._gdspx_input_get_global_mouse_pos;
    b.run('api = new GdspxFuncs(); other = new GdspxFuncs()');
    const first = b.run('api.gdspx_input_get_global_mouse_pos()');
    const second = b.run('api.gdspx_input_get_global_mouse_pos()');
    assert.equal(first, second);
    assert.equal(first.x, 2);
    assert.notEqual(b.run('other.gdspx_input_get_global_mouse_pos()'), first);
    assert.notEqual(b.run('api.gdspx_camera_get_camera_position()'), b.run('api.gdspx_camera_get_camera_position()'));
    assert.equal(released.length, 5);
    b.module._gdspx_input_get_global_mouse_pos = () => { throw new Error('engine failure'); };
    assert.throws(() => b.run('api.gdspx_input_get_global_mouse_pos()'), /engine failure/);
    assert.equal(released.length, 6);
    b.module._gdspx_res_free_str = () => { throw new Error('native release must not run on Web'); };
    assert.equal(b.run('api.gdspx_res_free_str("value")'), undefined);
});

test('writable arrays retain caller storage and reject copied or stale buffers', () => {
    const b = bridge();
    b.run('input = {[GDSPX_ARRAY_TAG]: true, type: 2, count: 1, data: new Uint8Array([0, 0, 192, 63])}');
    const copied = b.run('RequireNativeArray(input, "read", 2)');
    assert.equal(b.module.HEAPF32[copied / 4], 1.5);
    assert.throws(() => b.run('RequireNativeArray(input, "write", 2, true)'), /pre-allocated Wasm array/);
    const borrowed = b.run('borrowed = GdspxBorrowNativeArray(2, 1, 4)');
    const ptr = b.run('RequireNativeArray(borrowed, "write", 2, true)');
    assert.equal(ptr, borrowed.ptr);
    b.module.HEAPF32[ptr / 4] = 2.5;
    assert.equal(new DataView(borrowed.data.buffer, borrowed.data.byteOffset, 4).getFloat32(0, true), 2.5);
    b.run('clone = {...borrowed}');
    assert.throws(() => b.run('RequireNativeArray(clone, "write", 2, true)'), /pre-allocated Wasm array/);
    const copiedPtr = b.run('RequireNativeArray(clone, "read", 2)');
    assert.notEqual(copiedPtr, ptr, 'copied descriptors cannot claim the original Wasm pointer');
    assert.equal(b.module.HEAPF32[copiedPtr / 4], 2.5);
    assert.throws(() => b.run('RequireNativeArray(borrowed, "write", 5, true)'), /incompatible/);
    b.context.Module = {...b.module};
    assert.throws(() => b.run('RequireNativeArray(borrowed, "read", 2)'), /valid native array/);
    assert.throws(() => b.run('RequireNativeArray(borrowed, "write", 2, true)'), /valid native array/);
});
