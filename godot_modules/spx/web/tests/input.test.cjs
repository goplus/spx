const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../js/engine/gdspx.input.js'), 'utf8');

function bridge() {
    const context = vm.createContext({ Module: {} });
    vm.runInContext('function HasActiveModule() { return Module != null; }', context);
    vm.runInContext(source, context);
    return context;
}

test('action IDs are cached per module and preserve the registration receiver', () => {
    const b = bridge();
    vm.runInContext(`
        let registrations = 0;
        let instances = 0;
        class GdspxFuncs {
            constructor() { this.id = ++instances; }
            gdspx_input_register_action() { registrations++; return {low: this.id}; }
        }
    `, b);
    assert.equal(b.GdspxGetInputActionID('left'), 1);
    assert.equal(b.GdspxGetInputActionID('left'), 1);
    assert.equal(vm.runInContext('registrations', b), 1);
    const epoch = b.GdspxGetInputActionEpoch();
    b.Module = {};
    assert.equal(b.GdspxGetInputActionEpoch(), epoch + 1);
    assert.equal(b.GdspxGetInputActionID('left'), 2);
    assert.equal(vm.runInContext('registrations', b), 2);
});

test('direct registrations support integer representations and retry failed IDs', () => {
    const b = bridge();
    let calls = 0;
    let value = -1;
    b.gdspx_input_register_action = () => { calls++; return value; };
    assert.equal(b.GdspxGetInputActionID('left'), -1);
    value = 7n;
    assert.equal(b.GdspxGetInputActionID('left'), 7);
    assert.equal(b.GdspxGetInputActionID('left'), 7);
    assert.equal(calls, 2);
    value = {low: 9, high: 0};
    assert.equal(b.GdspxGetInputActionID('right'), 9);
    value = 11;
    assert.equal(b.GdspxGetInputActionID('up'), 11);
});

test('missing module or registration leaves the action unavailable', () => {
    const b = bridge();
    assert.equal(b.GdspxGetInputActionID('left'), -1);
    b.gdspx_input_register_action = () => 3;
    assert.equal(b.GdspxGetInputActionID('left'), 3);
    b.Module = null;
    assert.equal(b.GdspxGetInputActionID('left'), -1);
});
