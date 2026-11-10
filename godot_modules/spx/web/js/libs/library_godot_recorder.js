/** Browser streams, callbacks and chunks belong to one recording session. */
const GodotRecorder = {
	$GodotRecorder__deps: ['$GodotAudio'],
	$GodotRecorder__postset: [
		'Module["downloadRecordedVideo"] = GodotRecorder.downloadRecordedVideo;',
		'Module["getRecordedVideoBlob"] = GodotRecorder.getRecordedVideoBlob;',
		'Module["setRecorderCanvas"] = GodotRecorder.setRecorderCanvas;',
		'Module["tryStartRecording"] = GodotRecorder.tryStartRecording;',
		'Module["tryStopRecording"] = GodotRecorder.tryStopRecording;',
	].join(''),
	$GodotRecorder: {
		videoSession: null,
		targetCanvas: null,

		createSession: function(fps) {
			const source = GodotAudio.ctx && GodotAudio.buses[0]?.getOutputNode();
			if (!source || !this.targetCanvas) {
				GodotRuntime.error('Recording requires an audio context, Master bus and a video canvas');
				return null;
			}
			const session = { source: null, destination: null, videoStream: null, stream: null,
				recorder: null, chunks: [], blob: null, finished: true, completion: null, stopPromise: null };
			try {
				session.destination = GodotAudio.ctx.createMediaStreamDestination();
				source.connect(session.destination);
				session.source = source;
				const tracks = session.destination.stream.getAudioTracks();
				session.videoStream = this.targetCanvas.captureStream(fps);
				tracks.push(...session.videoStream.getVideoTracks());
				session.stream = new MediaStream(tracks);
				const types = ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus',
					'video/webm;codecs=h264,opus', 'video/webm', 'video/mp4;codecs=h264,aac', 'video/mp4'];
				session.mimeType = types.find(type => MediaRecorder.isTypeSupported(type)) || 'video/webm';
				return session;
			} catch (error) {
				this.releaseSession(session);
				GodotRuntime.error('Failed to initialize recording: ' + error.message);
				return null;
			}
		},

		startSession: function(session) {
			if (!session) {
				return false;
			}
			if (session.recorder && !session.finished) {
				return session.recorder.state === 'recording';
			}
			try {
				const options = { mimeType: session.mimeType, audioBitsPerSecond: 128000, videoBitsPerSecond: 2500000 };
				const recorder = new MediaRecorder(session.stream, options);
				session.recorder = recorder;
				session.chunks = [];
				session.blob = null;
				session.stopPromise = null;
				session.finished = false;
				session.completion = new Promise(resolve => {
					recorder.onstop = () => {
						session.finished = true;
						resolve();
					};
				});
				// Capture the session, so late events cannot reach a newer recording.
				recorder.ondataavailable = event => {
					if (event.data.size > 0) {
						session.chunks.push(event.data);
						session.blob = null;
					}
				};
				recorder.onerror = event => GodotRuntime.error('MediaRecorder error: ' + event.error);
				recorder.start(100);
				return true;
			} catch (error) {
				session.finished = true;
				session.completion = Promise.resolve();
				GodotRuntime.error('Failed to start recording: ' + error.message);
				return false;
			}
		},

		stopSession: function(session) {
			if (!session?.recorder || session.finished) {
				return false;
			}
			if (session.recorder.state !== 'inactive') session.recorder.stop();
			return true;
		},

		releaseSession: function(session) {
			if (!session) return;
			this.stopSession(session);
			if (session.destination) {
				// Disconnect only this recording branch, keeping speaker output intact.
				try {
					if (session.source) session.source.disconnect(session.destination);
				} catch (error) {
					GodotRuntime.error('Failed to disconnect recording: ' + error.message);
				}
				session.destination.stream.getTracks().forEach(track => track.stop());
				session.destination = null;
			}
			if (session.videoStream) {
				session.videoStream.getTracks().forEach(track => track.stop());
				session.videoStream = null;
			}
		},

		initVideoRecorder: function(fps = 30) {
			this.cleanupVideoRecorder();
			this.videoSession = this.createSession(fps);
			return this.videoSession !== null;
		},
		cleanupVideoRecorder: function() {
			this.releaseSession(this.videoSession);
			this.videoSession = null;
		},
		setRecorderCanvas: function(canvas) {
			GodotRecorder.targetCanvas = canvas;
		},

		tryStartRecording: function(filename = 'recording') {
			let started = false;
			let success = false;
			try {
				const filenamePtr = GodotRuntime.allocString(filename);
				try {
					started = Module['_godot_web_recording_request_start'](filenamePtr) === 1;
				} finally {
					GodotRuntime.free(filenamePtr);
				}
				if (started) {
					success = GodotRecorder.initVideoRecorder() && GodotRecorder.startSession(GodotRecorder.videoSession);
				}
				return success;
			} catch (error) {
				GodotRuntime.error('Failed to start recording: ' + error.message);
				return false;
			} finally {
				if (started && !success) {
					GodotRecorder.cleanupVideoRecorder();
					Module['_godot_web_recording_request_stop']();
				}
			}
		},

		tryStopRecording: function() {
			const session = GodotRecorder.videoSession;
			if (!session?.recorder) return Promise.resolve(null);
			if (!session.stopPromise) {
				session.stopPromise = (async () => {
					try {
						GodotRecorder.stopSession(session);
						// MediaRecorder emits its final data before stop, without a fixed delay.
						await session.completion;
						return GodotRecorder.sessionBlob(session);
					} finally {
						try {
							if (GodotRecorder.videoSession === session) Module['_godot_web_recording_request_stop']();
						} finally {
							GodotRecorder.releaseSession(session);
						}
					}
				})();
			}
			return session.stopPromise;
		},

		sessionBlob: function(session) {
			if (!session || session.chunks.length === 0) return null;
			if (!session.blob) session.blob = new Blob(session.chunks, { type: session.mimeType });
			return session.blob;
		},
		getRecordedVideoBlob: function() {
			return GodotRecorder.sessionBlob(GodotRecorder.videoSession);
		},
		downloadRecordedVideo: async function(filename) {
			const session = GodotRecorder.videoSession;
			// Native stop/cleanup is synchronous; retain this session until its final data arrives.
			if (session?.recorder?.state === 'inactive' && !session.finished) await session.completion;
			const blob = GodotRecorder.sessionBlob(session);
			if (!blob) return;
			const url = URL.createObjectURL(blob);
			try {
				const link = document.createElement('a');
				link.href = url;
				link.download = filename || 'recorded_video.' + blob.type.split('/')[1].split(';')[0];
				document.body.appendChild(link);
				link.click();
				link.remove();
			} finally {
				URL.revokeObjectURL(url);
			}
		},
	},

	// C++ interface functions

	godot_video_recorder_init__proxy: 'sync',
	godot_video_recorder_init__sig: 'ii',
	/**
	 * Initialize Canvas video recorder (audio and video merged recording)
	 * @param {number} fps Video frame rate
	 * @returns {number} 1 on success, 0 on failure
	 */
	godot_video_recorder_init: function(fps) {
		return GodotRecorder.initVideoRecorder(fps) ? 1 : 0;
	},

	godot_video_recorder_start__proxy: 'sync',
	godot_video_recorder_start__sig: 'i',
	/**
	 * Start video recording (audio and video merged)
	 * @returns {number} 1 on success, 0 on failure
	 */
	godot_video_recorder_start: function() {
		return GodotRecorder.startSession(GodotRecorder.videoSession) ? 1 : 0;
	},

	godot_video_recorder_stop__proxy: 'sync',
	godot_video_recorder_stop__sig: 'v',
	/**
	 * Stop video recording
	 */
	godot_video_recorder_stop: function() {
		GodotRecorder.stopSession(GodotRecorder.videoSession);
	},

	godot_video_recorder_download_data__proxy: 'sync',
	godot_video_recorder_download_data__sig: 'vi',
	/**
	 * Download recorded video data
	 * @param {number} filenamePtr Filename string pointer
	 */
	godot_video_recorder_download_data: function(filenamePtr) {
		const filename = GodotRuntime.parseString(filenamePtr);
		GodotRecorder.downloadRecordedVideo(filename).catch(error => GodotRuntime.error('Failed to download recording: ' + error.message));
	},

	godot_video_recorder_cleanup__proxy: 'sync',
	godot_video_recorder_cleanup__sig: 'v',
	/**
	 * Clean up video recorder resources
	 */
	godot_video_recorder_cleanup: function() {
		GodotRecorder.cleanupVideoRecorder();
	},
};

autoAddDeps(GodotRecorder, '$GodotRecorder');
mergeInto(LibraryManager.library, GodotRecorder);
