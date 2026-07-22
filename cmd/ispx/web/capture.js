"use strict";

// Coordinates canvas screenshots with the host that persists or downloads them.
// Game lifecycle concerns intentionally live outside this file.
function createCaptureBridge(captureScreenshot) {
	if (typeof captureScreenshot !== 'function') {
		throw new TypeError('captureScreenshot must be a function')
	}

	let host = null
	let hostVersion = 0
	let nextOrder = 0
	let isDelivering = false
	let deliveryScheduled = false
	let needsSort = false
	const pendingJobs = []
	const flushWaiters = []
	const inflightScreenshots = new Map()

	function setHost(nextHost) {
		hostVersion++
		host = nextHost || null
	}

	function getHost() {
		return host
	}

	function createFilename(request) {
		let captureName = typeof request.name === 'string' ? request.name.trim() : ''
		captureName = captureName.replace(/[\\/:*?"<>|\u0000-\u001F]+/g, '_').replace(/\.png$/i, '')

		const frame = Number.isFinite(request.frame)
			? String(Math.trunc(request.frame)).padStart(4, '0')
			: 'unknown'
		const sequence = Number.isFinite(request.sequence)
			? String(Math.trunc(request.sequence)).padStart(4, '0')
			: 'unknown'
		const nameSuffix = captureName ? `_${captureName}` : ''
		return `frame_${frame}_${sequence}${nameSuffix}.png`
	}

	function normalizeRequest(request) {
		if (!request || typeof request !== 'object' || Array.isArray(request)) {
			throw new Error('spx: capture request must be an object')
		}
		const normalized = {
			name: typeof request.name === 'string' ? request.name : '',
			frame: Number.isFinite(request.frame) ? request.frame : null,
			sequence: Number.isFinite(request.sequence) ? request.sequence : null,
		}
		normalized.filename = createFilename(normalized)
		return normalized
	}

	function emit(detail) {
		window.dispatchEvent(new CustomEvent('spxCaptureScreenshot', { detail }))
		if (window.parent && window.parent !== window) {
			try {
				window.parent.dispatchEvent(new CustomEvent('spxCaptureScreenshot', { detail }))
			} catch (error) {
				console.warn('Failed to notify parent capture event', error)
			}
		}
	}

	function downloadBlob(blob, filename) {
		const url = URL.createObjectURL(blob)
		const anchor = document.createElement('a')
		anchor.href = url
		anchor.download = filename
		document.body.appendChild(anchor)
		anchor.click()
		document.body.removeChild(anchor)
		setTimeout(() => URL.revokeObjectURL(url), 0)
	}

	function deliverToHost(captureHost, request, blob) {
		if (!captureHost) return null
		if (typeof captureHost === 'function') return Promise.resolve(captureHost(request, blob))
		if (typeof captureHost.handleCapture === 'function') {
			return Promise.resolve(captureHost.handleCapture(request, blob))
		}
		throw new Error('spx: capture host must be a function or expose handleCapture(request, blob)')
	}

	function emitSuccess(request, blob, destination) {
			emit({
			ok: true,
			name: request.name,
			filename: request.filename,
			frame: request.frame,
			sequence: request.sequence,
			destination,
			size: blob.size,
			type: blob.type || 'image/png',
		})
	}

	function emitFailure(request, error) {
		const message = error instanceof Error ? error.message : `${error}`
		console.error('spx: capture bridge failed', error)
		emit({
			ok: false,
			error: message,
			name: request ? request.name : '',
			filename: request ? request.filename : '',
			frame: request ? request.frame : null,
			sequence: request ? request.sequence : null,
		})
	}

	function notifyHostFailure(captureHost, request, error) {
		if (!captureHost || typeof captureHost.handleCaptureFailure !== 'function') return
		try {
			captureHost.handleCaptureFailure(request, error)
		} catch (notifyError) {
			console.error('spx: capture host failure notification failed', notifyError)
		}
	}

	function waitForRenderFence() {
		return new Promise((resolve) => {
			let settled = false
			let frameID = null
			let timeoutID = null
			let listeningForVisibility = false

			const cleanup = () => {
				if (timeoutID !== null) {
					window.clearTimeout(timeoutID)
					timeoutID = null
				}
				if (frameID !== null) {
					if (typeof window.cancelAnimationFrame === 'function') window.cancelAnimationFrame(frameID)
					frameID = null
				}
				if (listeningForVisibility) {
					document.removeEventListener('visibilitychange', onVisibilityChange)
					listeningForVisibility = false
				}
			}
			const finish = () => {
				if (settled) return
				settled = true
				cleanup()
				resolve()
			}
			const scheduleTimeout = (delay) => {
				if (timeoutID !== null) window.clearTimeout(timeoutID)
				timeoutID = window.setTimeout(finish, delay)
			}
			const onVisibilityChange = () => {
				if (document.visibilityState === 'hidden') finish()
			}

			if (typeof window.requestAnimationFrame !== 'function' || document.visibilityState === 'hidden') {
				scheduleTimeout(0)
				return
			}
			document.addEventListener('visibilitychange', onVisibilityChange)
			listeningForVisibility = true
			scheduleTimeout(250)
			frameID = window.requestAnimationFrame(finish)
		})
	}

	function captureFrame(request, order, version) {
		const key = request.frame === null ? `${version}:request:${order}` : `${version}:frame:${request.frame}`
		const inflight = inflightScreenshots.get(key)
		if (inflight) return inflight

		const screenshot = waitForRenderFence()
			.then(captureScreenshot)
			.then((blob) => {
				if (!blob) throw new Error(`spx: failed to capture screenshot ${request.filename}`)
				return { blob, error: null }
			})
			.catch((error) => ({ blob: null, error }))
			.then((result) => {
				if (inflightScreenshots.get(key) === screenshot) inflightScreenshots.delete(key)
				return result
			})
		inflightScreenshots.set(key, screenshot)
		return screenshot
	}

	async function deliver(job) {
		const screenshot = await job.screenshot
		if (screenshot.error) throw screenshot.error

		const completion = deliverToHost(job.host, job.request, screenshot.blob)
		if (completion === null) {
			downloadBlob(screenshot.blob, job.request.filename)
			emitSuccess(job.request, screenshot.blob, 'download')
			return
		}
		await completion
		emitSuccess(job.request, screenshot.blob, 'host')
	}

	function compareJobs(left, right) {
		if (left.hostVersion !== right.hostVersion) return left.hostVersion - right.hostVersion
		const leftSequence = left.request.sequence
		const rightSequence = right.request.sequence
		if (leftSequence !== null && rightSequence !== null && leftSequence !== rightSequence) {
			return leftSequence - rightSequence
		}
		if (leftSequence === null && rightSequence !== null) return 1
		if (leftSequence !== null && rightSequence === null) return -1
		return left.order - right.order
	}

	function enqueue(request, captureHost, version) {
		const order = ++nextOrder
		pendingJobs.push({
			request,
			host: captureHost,
			hostVersion: version,
			order,
			screenshot: captureFrame(request, order, version),
		})
		needsSort = true
		scheduleDelivery()
	}

	function scheduleDelivery() {
		if (isDelivering || deliveryScheduled) return
		deliveryScheduled = true
		Promise.resolve().then(deliverPending)
	}

	async function deliverPending() {
		if (isDelivering) return
		deliveryScheduled = false
		isDelivering = true
		try {
			while (pendingJobs.length > 0) {
				if (needsSort) {
					pendingJobs.sort(compareJobs)
					needsSort = false
				}
				const job = pendingJobs.shift()
				try {
					await deliver(job)
				} catch (error) {
					notifyHostFailure(job.host, job.request, error)
					emitFailure(job.request, error)
				}
			}
		} finally {
			isDelivering = false
			if (pendingJobs.length > 0) {
				scheduleDelivery()
				return
			}
			flushWaiters.splice(0).forEach((resolve) => resolve())
		}
	}

	function flush() {
		if (!isDelivering && !deliveryScheduled && pendingJobs.length === 0) return Promise.resolve()
		return new Promise((resolve) => flushWaiters.push(resolve))
	}

	function handleRequest(request) {
		const normalized = normalizeRequest(request)
		const captureHost = host
		const version = hostVersion
		enqueue(normalized, captureHost, version)
		return { ok: true, pending: true }
	}

	return Object.freeze({
		setHost,
		getHost,
		flush,
		handleRequest,
	})
}
