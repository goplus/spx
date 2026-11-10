const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

const sourcePath = path.join(__dirname, '../js/libs/library_godot_recorder.js');
const source = fs.readFileSync(sourcePath, 'utf8');

function recorderBridge(failAt = '') {
    const recordings = [], downloads = [], connections = [], disconnections = [], tracks = [];
    const errors = [], freed = [], downloadedBlobs = [];
    let runtimeActive = false, runtimeStops = 0;
    const fail = name => { if (failAt === name) throw new Error(name); };
    const track = kind => {
        const value = { kind, readyState: 'live', stop() { this.readyState = 'ended'; } };
        tracks.push(value);
        return value;
    };
    class MediaStream {
        constructor(values = []) { this.tracks = [...values]; }
        getTracks() { return this.tracks; }
        getAudioTracks() { return this.tracks.filter(value => value.kind === 'audio'); }
        getVideoTracks() { return this.tracks.filter(value => value.kind === 'video'); }
    }
    class MediaRecorder {
        static isTypeSupported(type) { return type.endsWith('webm'); }
        constructor(stream, options) {
            fail('recorder');
            this.stream = stream;
            this.options = options;
            this.state = 'inactive';
            recordings.push(this);
        }
        start() { fail('start'); this.state = 'recording'; }
        stop() { this.state = 'inactive'; } // The final events arrive later.
        emit(text) { this.ondataavailable?.({ data: new Blob([text]) }); }
        finish(text = '') { this.emit(text); this.onstop?.(); }
    }
    const output = {
        connect(destination) { fail('connect'); connections.push(destination); },
        disconnect(destination) {
            fail('disconnect');
            if (!connections.includes(destination)) throw new Error('InvalidAccessError');
            disconnections.push(destination);
        },
    };
    const b = vm.createContext({
        Blob, MediaStream, MediaRecorder, Module: {
            _godot_web_recording_request_start() {
                fail('native');
                if (runtimeActive) return 0;
                runtimeActive = true;
                return 1;
            },
            _godot_web_recording_request_stop() { fail('native-stop'); runtimeStops++; runtimeActive = false; },
        },
        LibraryManager: { library: {} }, autoAddDeps() {}, mergeInto: Object.assign,
        GodotAudio: {
            ctx: { createMediaStreamDestination: () => {
                fail('destination');
                return { stream: new MediaStream([track('audio')]) };
            } },
            buses: [{ getOutputNode: () => output }],
        },
        GodotRuntime: {
            error: message => errors.push(message), parseString: value => value,
            allocString: value => value, free: value => freed.push(value),
        },
        URL: { createObjectURL: blob => { downloadedBlobs.push(blob); return 'blob:test'; }, revokeObjectURL() {} },
        document: {
            createElement: () => ({ click() { downloads.push(this.download); }, remove() {} }),
            body: { appendChild() {} },
        },
    });
    // Emscripten binds this name to the $ object when emitting runtime glue.
    // Rebind it here as well, while executing the actual source with its filename.
    vm.runInContext(source.replace('const GodotAudioRecorder =', 'var GodotAudioRecorder ='), b, { filename: sourcePath });
    const library = b.LibraryManager.library;
    b.GodotAudioRecorder = library.$GodotAudioRecorder;
    for (const [name, value] of Object.entries(library)) {
        if (typeof value === 'function') b[name] = value;
    }
    vm.runInContext(library.$GodotAudioRecorder__postset, b);
    b.Module.setRecorderCanvas({ captureStream: () => {
        fail('capture');
        return new MediaStream([track('video')]);
    } });
    return { b, recordings, downloads, connections, disconnections, tracks, errors, freed, downloadedBlobs,
        active: () => runtimeActive, stopCount: () => runtimeStops,
        failAt: value => { failAt = value; } };
}

test('native video imports retain audio, cache chunks and support downloads', async () => {
    const { b, recordings, downloads, tracks, connections, disconnections } = recorderBridge();
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(connections.length, 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    assert.equal(recordings.length, 1);
    assert.deepEqual(recordings[0].stream.getTracks().map(track => track.kind), ['audio', 'video']);
    recordings[0].emit('first');
    const first = b.Module.getRecordedVideoBlob();
    assert.equal(b.Module.getRecordedVideoBlob(), first);
    recordings[0].emit('second');
    const complete = b.Module.getRecordedVideoBlob();
    assert.notEqual(complete, first);
    assert.equal(await complete.text(), 'firstsecond');
    b.godot_video_recorder_download_data('capture.webm');
    b.Module.downloadRecordedVideo();
    assert.deepEqual(downloads, ['capture.webm', 'recorded_video.webm']);
    b.godot_video_recorder_stop();
    assert.equal(recordings[0].state, 'inactive');
    recordings[0].finish();
    b.godot_video_recorder_cleanup();
    b.godot_video_recorder_cleanup();
    assert.equal(b.Module.getRecordedVideoBlob(), null);
    assert.ok(tracks.every(track => track.readyState === 'ended'));
    assert.deepEqual(disconnections, connections);
});

test('public stop waits for final data and shares completion across callers', async () => {
    const env = recorderBridge();
    const { b, recordings, freed } = env;
    assert.equal(b.Module.tryStartRecording('movie'), true);
    assert.deepEqual(freed, ['movie']);
    assert.equal(b.Module.tryStartRecording('duplicate'), false);
    assert.equal(recordings.length, 1);
    recordings[0].emit('prefix');
    const stopped = b.Module.tryStopRecording();
    assert.equal(b.Module.tryStopRecording(), stopped);
    let completed = false;
    stopped.then(() => { completed = true; });
    await Promise.resolve();
    assert.equal(completed, false);
    assert.equal(env.active(), true);
    recordings[0].finish('final');
    assert.equal(await (await stopped).text(), 'prefixfinal');
    assert.equal(env.active(), false);
    assert.equal(env.stopCount(), 1);
    assert.equal(await b.Module.tryStopRecording(), await stopped);
    assert.equal(env.stopCount(), 1);
    assert.equal(b.Module.tryStartRecording('next'), true);
    assert.ok(recordings[1].stream.getTracks().every(track => track.readyState === 'live'));
    const next = b.Module.tryStopRecording();
    recordings[1].finish('new');
    assert.equal(await (await next).text(), 'new');
});

test('cleanup isolates late callbacks from a new session and releases both stream branches', async () => {
    const { b, recordings, tracks } = recorderBridge();
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    b.godot_video_recorder_cleanup();
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    recordings[0].finish('old');
    assert.equal(b.Module.getRecordedVideoBlob(), null);
    recordings[1].emit('new');
    assert.equal(await b.Module.getRecordedVideoBlob().text(), 'new');
    assert.ok(tracks.slice(0, 2).every(track => track.readyState === 'ended'));
    assert.ok(tracks.slice(2).every(track => track.readyState === 'live'));
    b.godot_video_recorder_cleanup();
});

for (const failure of ['destination', 'connect', 'capture', 'recorder', 'start', 'native']) {
    test(`start failure at ${failure} rolls back and permits retry`, async () => {
        const env = recorderBridge(failure);
        assert.equal(env.b.Module.tryStartRecording('broken'), false);
        assert.equal(env.active(), false);
        assert.deepEqual(env.freed, ['broken']);
        assert.ok(env.tracks.every(track => track.readyState === 'ended'));
        env.failAt('');
        assert.equal(env.b.Module.tryStartRecording('retry'), true);
        const stop = env.b.Module.tryStopRecording();
        env.recordings.at(-1).finish();
        assert.equal(await stop, null);
        assert.equal(env.active(), false);
    });
}

test('missing browser prerequisites fail without entering a recording session', async () => {
    for (const missing of ['canvas', 'context', 'bus']) {
        const { b } = recorderBridge();
        if (missing === 'canvas') b.Module.setRecorderCanvas(null);
        if (missing === 'context') b.GodotAudio.ctx = null;
        if (missing === 'bus') b.GodotAudio.buses.length = 0;
        assert.equal(b.Module.tryStartRecording(), false);
        assert.equal(b.godot_video_recorder_start(), 0);
        assert.equal(await b.Module.tryStopRecording(), null);
    }
});

test('native stop downloads final chunks after cleanup without crossing sessions', async () => {
    const { b, recordings, downloads, downloadedBlobs } = recorderBridge();
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    b.godot_video_recorder_stop();
    b.godot_video_recorder_download_data('short.webm');
    b.godot_video_recorder_cleanup();
    assert.deepEqual(downloads, []);
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(b.godot_video_recorder_start(), 1);
    recordings[0].finish('final-only');
    await Promise.resolve();
    assert.deepEqual(downloads, ['short.webm']);
    assert.equal(await downloadedBlobs[0].text(), 'final-only');
    assert.equal(b.Module.getRecordedVideoBlob(), null);
    b.godot_video_recorder_stop();
    b.godot_video_recorder_download_data('empty.webm');
    b.godot_video_recorder_cleanup();
    recordings[1].finish();
    await Promise.resolve();
    assert.deepEqual(downloads, ['short.webm']);
});

test('an old pending public stop cannot stop the replacement runtime session', async () => {
    const env = recorderBridge();
    const { b, recordings } = env;
    assert.equal(b.Module.tryStartRecording(), true);
    const oldStop = b.Module.tryStopRecording();
    b.Module._godot_web_recording_request_stop();
    b.godot_video_recorder_cleanup();
    assert.equal(b.Module.tryStartRecording(), true);
    recordings[0].finish('old');
    assert.equal(await (await oldStop).text(), 'old');
    assert.equal(env.active(), true);
    assert.equal(env.stopCount(), 1);
    const newStop = b.Module.tryStopRecording();
    recordings[1].finish('new');
    assert.equal(await (await newStop).text(), 'new');
    assert.equal(env.active(), false);
});

test('native stop errors still release browser resources', async () => {
    const env = recorderBridge();
    assert.equal(env.b.Module.tryStartRecording(), true);
    env.failAt('native-stop');
    const stop = env.b.Module.tryStopRecording();
    env.recordings[0].finish('complete');
    await assert.rejects(stop, /native-stop/);
    assert.ok(env.tracks.every(track => track.readyState === 'ended'));
    assert.deepEqual(env.disconnections, env.connections);
});

test('disconnected audio branches do not prevent track cleanup', async () => {
    const env = recorderBridge();
    assert.equal(env.b.Module.tryStartRecording(), true);
    env.failAt('disconnect');
    const stop = env.b.Module.tryStopRecording();
    env.recordings[0].finish();
    assert.equal(await stop, null);
    assert.ok(env.tracks.every(track => track.readyState === 'ended'));
    assert.equal(env.active(), false);
    assert.equal(env.errors.length, 1);
});

test('failed native MediaRecorder start leaves no pending completion', async () => {
    const { b, tracks } = recorderBridge('start');
    assert.equal(b.godot_video_recorder_init(24), 1);
    assert.equal(b.godot_video_recorder_start(), 0);
    assert.equal(await b.Module.tryStopRecording(), null);
    assert.ok(tracks.every(track => track.readyState === 'ended'));
});
