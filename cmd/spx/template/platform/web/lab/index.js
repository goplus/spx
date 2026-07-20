const LAB_CONFIG = Object.freeze({
	projects: Object.freeze({
		primary: '/game.zip',
	}),
	replay: Object.freeze({
		baselineFilename: 'input-replay.json',
		statusPollIntervalMs: 100,
	}),
	capture: Object.freeze({
		refreshDelayMs: 120,
	}),
})

const INPUT_MODES = Object.freeze({
	normal: Object.freeze({ label: 'Normal', captureMode: 'auto' }),
	record: Object.freeze({ label: 'Record', captureMode: 'baseline' }),
	replay: Object.freeze({ label: 'Replay', captureMode: 'run' }),
})
const INPUT_MODE_ORDER = Object.freeze(['normal', 'record', 'replay'])

const LOG_LEVEL_VERBOSE = 0
const captureKeyParam = new URLSearchParams(window.location.search).get('captureKey')

const runtimeState = {
	phase: 'initializing',
	inputMode: 'normal',
	activeRun: null,
	stopPromise: null,
	message: '',
	isError: false,
}

const ui = {
	startGame: document.getElementById('startGameBtn'),
	stopGame: document.getElementById('stopGameBtn'),
	inputMode: document.getElementById('inputModeBtn'),
	inputModeStatus: document.getElementById('inputModeStatus'),
	captureDirectory: document.getElementById('selectCaptureDirBtn'),
	runner: document.getElementById('runnerFrame'),
}

const runnerWindow = ui.runner.contentWindow
let enlarged = false
let replayStatusPollTimer = 0
let replayStatusPollVersion = 0

window.addEventListener('load', () => {
	animateBox()
	runUIAction('initialize engine', initializeRuntime)
})

ui.startGame.addEventListener('click', () => runUIAction('start game', () => startGame(createSelectedStartOptions())))
ui.stopGame.addEventListener('click', () => runUIAction('stop game', stopGame))
ui.inputMode.addEventListener('click', selectNextInputMode)
document.getElementById('pauseGameBtn').addEventListener('click', () => runUIAction('pause game', pauseGame))
document.getElementById('resumeGameBtn').addEventListener('click', () => runUIAction('resume game', resumeGame))
document.getElementById('stepNextFrameBtn').addEventListener('click', () => runUIAction('step frame', stepNextFrame))
document.getElementById('toggleSizeBtn').addEventListener('click', () => runUIAction('resize game', toggleSizeFrame))
document.getElementById('selectCaptureDirBtn').addEventListener('click', () => runUIAction('select capture directory', selectCaptureDirectory))
document.getElementById('refreshCompareBtn').addEventListener('click', () => runUIAction('refresh capture comparison', refreshCaptureCompare))

function runUIAction(name, action) {
	void Promise.resolve().then(action).catch((error) => {
		console.error(`Failed to ${name}`, error)
	})
}

function createSelectedStartOptions() {
	return {
		input: runnerWindow.spxCreateInputSession(runtimeState.inputMode),
	}
}

function selectNextInputMode() {
	if (runtimeState.phase !== 'stopped') return
	const modes = INPUT_MODE_ORDER
	const nextIndex = (modes.indexOf(runtimeState.inputMode) + 1) % modes.length
	runtimeState.inputMode = modes[nextIndex]
	runtimeState.message = ''
	runtimeState.isError = false
	renderRuntimeControls()
}

function setRuntimeStatus(message, isError = false) {
	runtimeState.message = message
	runtimeState.isError = isError
	renderRuntimeControls()
}

function stopReplayStatusPolling() {
	replayStatusPollVersion++
	if (replayStatusPollTimer) {
		window.clearTimeout(replayStatusPollTimer)
		replayStatusPollTimer = 0
	}
}

function formatReplayStatus(status) {
	const currentTick = Number.isInteger(status.currentTick) && status.currentTick >= 0
		? status.currentTick
		: null
	const frameCount = Number.isInteger(status.frameCount) && status.frameCount >= 0
		? status.frameCount
		: 0
	const lastTick = frameCount > 0 ? frameCount - 1 : null
	const progress = lastTick === null
		? (currentTick === null ? '—' : `${currentTick}`)
		: `${currentTick === null ? '—' : currentTick} / ${lastTick}`
	const completed = status.completed === true || status.phase === 'completed'
	return `${completed ? 'Replay complete' : 'Replay'} · tick ${progress}`
}

function startReplayStatusPolling(run) {
	stopReplayStatusPolling()
	if (!run || run.mode !== 'replay') return

	const version = replayStatusPollVersion
	const poll = async () => {
		replayStatusPollTimer = 0
		if (version !== replayStatusPollVersion || runtimeState.activeRun !== run || runtimeState.phase !== 'running') return
		try {
			const status = await runnerWindow.getInputSessionStatus()
			if (version !== replayStatusPollVersion || runtimeState.activeRun !== run || runtimeState.phase !== 'running') return
			if (status.phase === 'aborted') {
				setRuntimeStatus(`Replay aborted: ${status.error || 'unknown error'}`, true)
				return
			}
			setRuntimeStatus(formatReplayStatus(status))
			if (status.completed === true || status.phase === 'completed') return
		} catch (error) {
			console.warn('Failed to read replay tick status', error)
			return
		}
		if (version === replayStatusPollVersion && runtimeState.activeRun === run && runtimeState.phase === 'running') {
			replayStatusPollTimer = window.setTimeout(poll, LAB_CONFIG.replay.statusPollIntervalMs)
		}
	}
	void poll()
}

function renderRuntimeControls() {
	const mode = INPUT_MODES[runtimeState.inputMode]
	const isStopped = runtimeState.phase === 'stopped'
	ui.inputMode.textContent = mode.label
	ui.inputMode.setAttribute('aria-label', `Input mode: ${mode.label}. Click to switch mode.`)
	ui.inputMode.disabled = !isStopped
	ui.startGame.disabled = !isStopped
	ui.stopGame.disabled = runtimeState.phase !== 'running'
	ui.captureDirectory.disabled = !isStopped
	ui.inputModeStatus.textContent = runtimeState.message
	ui.inputModeStatus.hidden = !runtimeState.message
	ui.inputModeStatus.style.color = runtimeState.isError ? '#ff9b9b' : '#fff'
}

renderRuntimeControls()

async function initializeRuntime() {
	try {
		configureRunner()
		await initEngine()
		runtimeState.phase = 'stopped'
		runtimeState.message = ''
		runtimeState.isError = false
		renderRuntimeControls()
	} catch (error) {
		runtimeState.phase = 'failed'
		setRuntimeStatus(`Engine initialization failed: ${error && error.message ? error.message : error}`, true)
		throw error
	}
}

function configureRunner() {
	const overrides = {}
	if (captureKeyParam !== null) {
		const captureKey = !captureKeyParam || /^(off|none)$/i.test(captureKeyParam) ? null : captureKeyParam
		overrides.input = {
			record: { captureKey },
			replay: { captureKey },
		}
	}
	const config = runnerWindow.spxConfigureRunner(overrides)
	runtimeState.inputMode = config.input.defaultMode
}

function animateBox() {
	const box = document.querySelector('.animation-box')
	if (!box) return
	const style = window.getComputedStyle(box)
	if (style.animationName !== 'none') return
	let position = 0
	let direction = 1
	setInterval(() => {
		if (position >= 250) direction = -1
		if (position <= 0) direction = 1
		position += direction * 2
		box.style.left = `${position}px`
	}, 20)
}

const captureState = {
	rootHandle: null,
	project: null,
	session: null,
	sessionCounter: 0,
	refreshTimer: 0,
	previewUrls: [],
}
const inputReplayBaselineFilename = LAB_CONFIG.replay.baselineFilename

function updateCaptureStatus(message, isError = false) {
	const status = document.getElementById('captureStatus')
	status.textContent = message
	status.style.color = isError ? '#ff9b9b' : '#fff'
}

function revokeCapturePreviewUrls() {
	captureState.previewUrls.forEach((url) => URL.revokeObjectURL(url))
	captureState.previewUrls = []
}

function setCaptureCompareEmpty(message, summary = message) {
	revokeCapturePreviewUrls()
	document.getElementById('captureCompareSummary').textContent = summary
	document.getElementById('captureCompareGrid').innerHTML = `<div class="capture-compare-empty">${message}</div>`
}

function scheduleCaptureCompareRefresh() {
	if (captureState.refreshTimer) {
		clearTimeout(captureState.refreshTimer)
	}
	captureState.refreshTimer = window.setTimeout(() => {
		captureState.refreshTimer = 0
		refreshCaptureCompare().catch((error) => {
			console.error('Failed to refresh capture compare', error)
			setCaptureCompareEmpty('Failed to read capture files. Check browser console for details.')
		})
	}, LAB_CONFIG.capture.refreshDelayMs)
}

function browserSupportsCaptureDirectory() {
	return typeof window.showDirectoryPicker === 'function'
}

function buildCaptureDirectoryPickerOptions() {
	const options = {
		mode: 'readwrite',
		id: 'spx-capture-root',
	}
	if (captureState.rootHandle) {
		options.startIn = captureState.rootHandle
		return options
	}
	options.startIn = 'downloads'
	return options
}

function sanitizeCaptureSegment(value, fallback = 'project') {
	let segment = typeof value === 'string' ? value.trim() : ''
	segment = segment.replace(/[\\/:*?"<>|\u0000-\u001F]+/g, '_')
	return segment || fallback
}

function formatCaptureTimestamp(date = new Date()) {
	const pad2 = (value) => `${value}`.padStart(2, '0')
	const milliseconds = `${date.getMilliseconds()}`.padStart(3, '0')
	return `${date.getFullYear()}${pad2(date.getMonth() + 1)}${pad2(date.getDate())}-${pad2(date.getHours())}${pad2(date.getMinutes())}${pad2(date.getSeconds())}-${milliseconds}`
}

function decodeZipText(unzipped, path) {
	const entry = unzipped[path]
	if (!entry) {
		return ''
	}
	return new TextDecoder('utf-8').decode(entry)
}

function parseCaptureProject(unzipped) {
	let builderMeta = null
	let projectIndex = null
	try {
		const builderMetaText = decodeZipText(unzipped, 'builder-meta.json')
		if (builderMetaText) {
			builderMeta = JSON.parse(builderMetaText)
		}
	} catch (error) {
		console.warn('Failed to parse builder-meta.json for capture host', error)
	}
	try {
		const projectIndexText = decodeZipText(unzipped, 'assets/index.json')
		if (projectIndexText) {
			projectIndex = JSON.parse(projectIndexText)
		}
	} catch (error) {
		console.warn('Failed to parse assets/index.json for capture host', error)
	}
	return {
		projectKey: sanitizeCaptureSegment(
			(builderMeta && builderMeta.displayName) ||
			(projectIndex && projectIndex.name) ||
			'project',
		),
	}
}

function beginCaptureSession(project, captureMode = 'auto') {
	captureState.sessionCounter++
	const runTimestamp = formatCaptureTimestamp()
	const context = Object.freeze({
		id: `${runTimestamp}-${captureState.sessionCounter}`,
		projectKey: project.projectKey,
		captureMode,
		runTimestamp,
	})
	captureState.project = project
	captureState.session = {
		...context,
		context,
		hosts: new Set(),
		canPublishBaselineMarker: Boolean(captureState.rootHandle),
		requiresInputReplay: captureMode === 'baseline',
		inputReplaySaved: captureMode !== 'baseline',
		captureTargetRootHandle: null,
		captureTargetPromise: null,
		finalizePromise: null,
	}
	scheduleCaptureCompareRefresh()
}

async function readBaselineMarker(projectDir) {
	try {
		const markerHandle = await projectDir.getFileHandle('.baseline.ready')
		const markerFile = await markerHandle.getFile()
		const marker = JSON.parse(await markerFile.text())
		const files = marker && marker.files
		const hasInputReplay = marker && marker.inputReplay === inputReplayBaselineFilename
		const hasValidFiles = Array.isArray(files) &&
			files.length === marker.captureCount &&
			files.every((name) => typeof name === 'string' && name.toLowerCase().endsWith('.png')) &&
			new Set(files).size === files.length
		if (!marker || !Number.isInteger(marker.captureCount) || marker.captureCount <= 0 || !hasValidFiles || !hasInputReplay) {
			return null
		}
		const baselineDir = await projectDir.getDirectoryHandle('baseline')
		await baselineDir.getFileHandle(marker.inputReplay)
		for (const filename of marker.files) {
			await baselineDir.getFileHandle(filename)
		}
		return marker
	} catch (error) {
		if (error && error.name === 'NotFoundError') {
			return null
		}
		if (error instanceof SyntaxError) {
			console.warn('Ignoring incomplete baseline marker', error)
			return null
		}
		throw error
	}
}

async function getOptionalDirectoryHandle(parentHandle, name) {
	try {
		return await parentHandle.getDirectoryHandle(name)
	} catch (error) {
		if (error && error.name === 'NotFoundError') {
			return null
		}
		throw error
	}
}

async function clearCaptureDirectory(directoryHandle) {
	const entries = []
	for await (const entry of directoryHandle.values()) {
		entries.push(entry)
	}
	for (const entry of entries) {
		await directoryHandle.removeEntry(entry.name, { recursive: entry.kind === 'directory' })
	}
}

async function removeBaselineReadyMarker(projectDir) {
	try {
		await projectDir.removeEntry('.baseline.ready')
	} catch (error) {
		if (!error || error.name !== 'NotFoundError') {
			throw error
		}
	}
}

async function prepareCaptureTarget(rootHandle, session) {
	const snapshotsDir = await rootHandle.getDirectoryHandle('spx-snapshots', { create: true })
	const projectDir = await snapshotsDir.getDirectoryHandle(session.projectKey, { create: true })
	if (session.captureMode === 'baseline') {
		// Invalidate the previous baseline before deleting any of its files.
		// This runs when the session starts, even if it produces no captures.
		await removeBaselineReadyMarker(projectDir)
		const baselineDir = await projectDir.getDirectoryHandle('baseline', { create: true })
		await clearCaptureDirectory(baselineDir)
		return { directory: baselineDir, destination: 'baseline', projectDir, isBaseline: true }
	}
	const runsDir = await projectDir.getDirectoryHandle('runs', { create: true })
	const runDir = await runsDir.getDirectoryHandle(session.runTimestamp, { create: true })
	return { directory: runDir, destination: `runs/${session.runTimestamp}`, projectDir, isBaseline: false }
}

async function prepareBaselineCaptureTarget(rootHandle, session) {
	if (!rootHandle || !session || session.captureMode !== 'baseline') {
		return null
	}
	if (session.captureTargetRootHandle === rootHandle && session.captureTargetPromise) {
		return session.captureTargetPromise
	}

	const targetPromise = prepareCaptureTarget(rootHandle, session.context)
	session.captureTargetRootHandle = rootHandle
	session.captureTargetPromise = targetPromise
	try {
		return await targetPromise
	} catch (error) {
		session.canPublishBaselineMarker = false
		if (session.captureTargetPromise === targetPromise) {
			session.captureTargetRootHandle = null
			session.captureTargetPromise = null
		}
		throw error
	}
}

function requireInputReplayCaptureDirectory() {
	if (!captureState.rootHandle) {
		throw new Error('choose a capture folder before recording or replaying input')
	}
}

async function saveInputReplayBaseline(replayBlob, session) {
	requireInputReplayCaptureDirectory()
	if (!session) {
		throw new Error('missing capture session for input recording')
	}
	if (!replayBlob || typeof replayBlob.size !== 'number' || replayBlob.size === 0 || typeof replayBlob.text !== 'function') {
		throw new Error('input recording did not return replay data')
	}
	try {
		JSON.parse(await replayBlob.text())
	} catch (error) {
		throw new Error(`input recording returned invalid JSON: ${error && error.message ? error.message : error}`)
	}
	const target = await prepareBaselineCaptureTarget(captureState.rootHandle, session)
	if (!target || !target.isBaseline) {
		throw new Error('input recording has no baseline capture target')
	}
	const file = await target.directory.getFileHandle(inputReplayBaselineFilename, { create: true })
	const writer = await file.createWritable()
	await writer.write(replayBlob)
	await writer.close()
}

async function loadInputReplayBaseline(project) {
	requireInputReplayCaptureDirectory()
	if (!project || !project.projectKey) {
		throw new Error('missing project metadata for input replay')
	}
	const snapshotsDir = await getOptionalDirectoryHandle(captureState.rootHandle, 'spx-snapshots')
	const projectDir = snapshotsDir && await getOptionalDirectoryHandle(snapshotsDir, project.projectKey)
	const baselineDir = projectDir && await getOptionalDirectoryHandle(projectDir, 'baseline')
	if (!baselineDir) {
		throw new Error(`no input baseline found for ${project.projectKey}; choose Record mode, Start, then Stop`)
	}
	try {
		const file = await baselineDir.getFileHandle(inputReplayBaselineFilename)
		return { data: await file.getFile(), name: `${project.projectKey}/baseline/${inputReplayBaselineFilename}` }
	} catch (error) {
		if (error && error.name === 'NotFoundError') {
			throw new Error(`no recorded input found for ${project.projectKey}; choose Record mode, Start, then Stop`)
		}
		throw error
	}
}

function createCaptureTargetResolver(rootHandle, session, preparedTargetPromise = null) {
	let targetPromise = preparedTargetPromise
	return () => {
		if (!targetPromise) {
			targetPromise = prepareCaptureTarget(rootHandle, session)
		}
		return targetPromise
	}
}

async function markBaselineReady(projectDir, session, summary) {
	try {
		const marker = await projectDir.getFileHandle('.baseline.ready', { create: true })
		const writer = await marker.createWritable()
		await writer.write(JSON.stringify({
			completedAt: new Date().toISOString(),
			projectKey: session.projectKey,
			sessionId: session.id,
			inputReplay: inputReplayBaselineFilename,
			captureCount: summary.succeeded,
			files: summary.files.slice().sort((left, right) => left.localeCompare(right)),
		}, null, 2))
		await writer.close()
	} catch (error) {
		try {
			await projectDir.removeEntry('.baseline.ready')
		} catch (removeError) {
			if (!removeError || removeError.name !== 'NotFoundError') {
				console.warn('Failed to remove incomplete baseline marker', removeError)
			}
		}
		throw error
	}
}

function buildCaptureFilename(request) {
	return request.filename
}

function formatCaptureFileMeta(file) {
	const sizeKB = (file.size / 1024).toFixed(1)
	const modifiedAt = new Date(file.lastModified).toLocaleString()
	return `${sizeKB} KB · ${modifiedAt}`
}

async function listCaptureFiles(directoryHandle) {
	if (!directoryHandle) {
		return new Map()
	}
	const entries = []
	for await (const entry of directoryHandle.values()) {
		if (entry.kind !== 'file' || !entry.name.toLowerCase().endsWith('.png')) {
			continue
		}
		entries.push(entry)
	}
	entries.sort((left, right) => left.name.localeCompare(right.name))

	const files = new Map()
	for (const entry of entries) {
		const file = await entry.getFile()
		const url = URL.createObjectURL(file)
		captureState.previewUrls.push(url)
		files.set(entry.name, {
			name: entry.name,
			file,
			url,
			meta: formatCaptureFileMeta(file),
		})
	}
	return files
}

async function findLatestRunDirectory(projectDir) {
	const runsDir = await getOptionalDirectoryHandle(projectDir, 'runs')
	if (!runsDir) {
		return null
	}
	let latestRun = null
	for await (const entry of runsDir.values()) {
		if (entry.kind !== 'directory') {
			continue
		}
		if (!latestRun || entry.name > latestRun.name) {
			latestRun = entry
		}
	}
	return latestRun
}

function formatCaptureSimilarityPercent(score) {
	return `${(score * 100).toFixed(2)}%`
}

function getCaptureSimilarityTone(score) {
	if (score >= 0.995) {
		return 'capture-compare-score-strong'
	}
	if (score >= 0.97) {
		return 'capture-compare-score-good'
	}
	if (score >= 0.9) {
		return 'capture-compare-score-warn'
	}
	return 'capture-compare-score-bad'
}

function formatCaptureBitmapSize(width, height) {
	return `${width}x${height}`
}

const captureQuickCompareSize = 128
const captureDetailedCompareSize = 320
const captureDetailedCompareThreshold = 0.985

async function loadCaptureBitmap(file) {
	if (typeof createImageBitmap === 'function') {
		return createImageBitmap(file)
	}
	return await new Promise((resolve, reject) => {
		const url = URL.createObjectURL(file)
		const image = new Image()
		image.onload = () => {
			URL.revokeObjectURL(url)
			resolve(image)
		}
		image.onerror = () => {
			URL.revokeObjectURL(url)
			reject(new Error(`Failed to decode ${file.name}`))
		}
		image.src = url
	})
}

function readCaptureImageData(imageSource, width, height) {
	const canvas = document.createElement('canvas')
	canvas.width = width
	canvas.height = height
	const context = canvas.getContext('2d', { willReadFrequently: true })
	context.drawImage(imageSource, 0, 0, width, height)
	return context.getImageData(0, 0, width, height).data
}

function readCaptureGrayscalePixels(imageSource, width, height) {
	const rgbaPixels = readCaptureImageData(imageSource, width, height)
	const grayscalePixels = new Uint8Array(rgbaPixels.length / 4)
	for (let src = 0, dest = 0; src < rgbaPixels.length; src += 4, dest++) {
		grayscalePixels[dest] = (
			rgbaPixels[src] * 77 +
			rgbaPixels[src + 1] * 150 +
			rgbaPixels[src + 2] * 29
		) >> 8
	}
	return grayscalePixels
}

function computeCaptureNormalizedDiff(leftPixels, rightPixels) {
	const pixelCount = Math.min(leftPixels.length, rightPixels.length)
	if (pixelCount === 0) {
		return 1
	}

	let totalDiff = 0
	for (let i = 0; i < pixelCount; i++) {
		totalDiff += Math.abs(leftPixels[i] - rightPixels[i])
	}
	return totalDiff / (pixelCount * 255)
}

function buildCaptureCompareSize(leftBitmap, rightBitmap, limit) {
	return {
		width: Math.max(1, Math.min(limit, leftBitmap.width, rightBitmap.width)),
		height: Math.max(1, Math.min(limit, leftBitmap.height, rightBitmap.height)),
	}
}

function buildCaptureSimilarityResult(score, detail) {
	return {
		status: 'ready',
		score,
		badgeClass: `capture-compare-score ${getCaptureSimilarityTone(score)}`,
		badgeText: `${formatCaptureSimilarityPercent(score)} match`,
		detail,
	}
}

async function computeCaptureSimilarity(leftRecord, rightRecord) {
	if (!leftRecord || !rightRecord) {
		if (leftRecord || rightRecord) {
			return {
				status: 'missing',
				badgeClass: 'capture-compare-score capture-compare-score-missing',
				badgeText: leftRecord ? 'Missing run' : 'Missing baseline',
				detail: '',
				score: null,
			}
		}
		return {
			status: 'missing',
			badgeClass: 'capture-compare-score capture-compare-score-missing',
			badgeText: 'No pair',
			detail: '',
			score: null,
		}
	}

	const leftBitmap = await loadCaptureBitmap(leftRecord.file)
	const rightBitmap = await loadCaptureBitmap(rightRecord.file)
	try {
		const leftSize = formatCaptureBitmapSize(leftBitmap.width, leftBitmap.height)
		const rightSize = formatCaptureBitmapSize(rightBitmap.width, rightBitmap.height)
		const sizeNote = leftSize === rightSize
			? ''
			: `Size mismatch: ${leftSize} vs ${rightSize}; `

		const quickSize = buildCaptureCompareSize(leftBitmap, rightBitmap, captureQuickCompareSize)
		const quickLeftPixels = readCaptureGrayscalePixels(leftBitmap, quickSize.width, quickSize.height)
		const quickRightPixels = readCaptureGrayscalePixels(rightBitmap, quickSize.width, quickSize.height)
		const quickScore = Math.max(0, 1 - computeCaptureNormalizedDiff(quickLeftPixels, quickRightPixels))
		const quickComparedSize = formatCaptureBitmapSize(quickSize.width, quickSize.height)

		if (quickScore >= captureDetailedCompareThreshold && leftSize === rightSize) {
			return buildCaptureSimilarityResult(
				quickScore,
				`${sizeNote}Quick grayscale compare at ${quickComparedSize}`,
			)
		}

		const detailedSize = buildCaptureCompareSize(leftBitmap, rightBitmap, captureDetailedCompareSize)
		const detailedLeftPixels = readCaptureImageData(leftBitmap, detailedSize.width, detailedSize.height)
		const detailedRightPixels = readCaptureImageData(rightBitmap, detailedSize.width, detailedSize.height)
		const detailedScore = Math.max(0, 1 - computeCaptureNormalizedDiff(detailedLeftPixels, detailedRightPixels))
		const detailedComparedSize = formatCaptureBitmapSize(detailedSize.width, detailedSize.height)

		return buildCaptureSimilarityResult(
			detailedScore,
			`${sizeNote}Quick ${formatCaptureSimilarityPercent(quickScore)} at ${quickComparedSize}; refined RGBA at ${detailedComparedSize}`,
		)
	} catch (error) {
		return {
			status: 'error',
			score: null,
			badgeClass: 'capture-compare-score capture-compare-score-missing',
			badgeText: 'Compare failed',
			detail: error && error.message ? error.message : 'Unknown image compare error',
		}
	} finally {
		if (typeof leftBitmap.close === 'function') {
			leftBitmap.close()
		}
		if (typeof rightBitmap.close === 'function') {
			rightBitmap.close()
		}
	}
}

async function buildCaptureComparisons(names, baselineFiles, runFiles) {
	return await Promise.all(names.map(async (name) => ({
		name,
		baseline: baselineFiles.get(name),
		run: runFiles.get(name),
		similarity: await computeCaptureSimilarity(baselineFiles.get(name), runFiles.get(name)),
	})))
}

function createCaptureCompareFileCell(name, similarity) {
	const cell = document.createElement('div')
	cell.className = 'capture-compare-file'

	const title = document.createElement('div')
	title.className = 'capture-compare-name'
	title.textContent = name
	cell.appendChild(title)

	const badge = document.createElement('div')
	badge.className = similarity.badgeClass
	badge.textContent = similarity.badgeText
	cell.appendChild(badge)

	if (similarity.detail) {
		const detail = document.createElement('div')
		detail.className = 'capture-compare-detail'
		detail.textContent = similarity.detail
		cell.appendChild(detail)
	}

	return cell
}

function createCaptureCompareCell(label, imageRecord) {
	const cell = document.createElement('div')
	cell.className = 'capture-compare-cell'

	const title = document.createElement('p')
	title.className = 'capture-compare-label'
	title.textContent = label
	cell.appendChild(title)

	if (!imageRecord) {
		const missing = document.createElement('div')
		missing.className = 'capture-compare-missing'
		missing.textContent = 'No image'
		cell.appendChild(missing)
		return cell
	}

	const link = document.createElement('a')
	link.className = 'capture-compare-link'
	link.href = imageRecord.url
	link.target = '_blank'
	link.rel = 'noopener noreferrer'

	const image = document.createElement('img')
	image.className = 'capture-compare-image'
	image.src = imageRecord.url
	image.alt = imageRecord.name
	image.loading = 'lazy'

	link.appendChild(image)
	cell.appendChild(link)

	const meta = document.createElement('div')
	meta.className = 'capture-compare-meta'
	meta.textContent = imageRecord.meta
	cell.appendChild(meta)

	return cell
}

function renderCaptureCompare(projectKey, runLabel, comparisons) {
	const grid = document.getElementById('captureCompareGrid')
	const summary = document.getElementById('captureCompareSummary')
	grid.innerHTML = ''

	if (comparisons.length === 0) {
		setCaptureCompareEmpty('No capture images found yet.', `${projectKey} · waiting for baseline or run`)
		return
	}

	const matchedComparisons = comparisons.filter((item) => item.similarity.status === 'ready')
	const incompleteCount = comparisons.length - matchedComparisons.length
	const averageSimilarity = matchedComparisons.length > 0
		? matchedComparisons.reduce((sum, item) => sum + item.similarity.score, 0) / matchedComparisons.length
		: null

	const summaryParts = [
		`${projectKey}`,
		`baseline vs ${runLabel}`,
		`${comparisons.length} frame(s)`,
	]
	if (averageSimilarity !== null) {
		summaryParts.push(`avg ${formatCaptureSimilarityPercent(averageSimilarity)}`)
	}
	if (incompleteCount > 0) {
		summaryParts.push(`${incompleteCount} incomplete`)
	}
	summary.textContent = summaryParts.join(' · ')

	for (const item of comparisons) {
		const row = document.createElement('div')
		row.className = 'capture-compare-row'

		row.appendChild(createCaptureCompareFileCell(item.name, item.similarity))
		row.appendChild(createCaptureCompareCell('Baseline', item.baseline))
		row.appendChild(createCaptureCompareCell(runLabel, item.run))
		grid.appendChild(row)
	}
}

async function refreshCaptureCompare() {
	if (!captureState.rootHandle) {
		setCaptureCompareEmpty('Choose a capture folder to browse baseline and run images.')
		return
	}
	if (!captureState.project) {
		setCaptureCompareEmpty('Start a project to load capture metadata.')
		return
	}

	const snapshotsDir = await getOptionalDirectoryHandle(captureState.rootHandle, 'spx-snapshots')
	if (!snapshotsDir) {
		setCaptureCompareEmpty('No spx-snapshots folder found in the selected directory.', `${captureState.project.projectKey} · no snapshots yet`)
		return
	}

	const projectDir = await getOptionalDirectoryHandle(snapshotsDir, captureState.project.projectKey)
	if (!projectDir) {
		setCaptureCompareEmpty(`No snapshots yet for ${captureState.project.projectKey}.`, `${captureState.project.projectKey} · waiting for first capture`)
		return
	}

	const baselineMarker = await readBaselineMarker(projectDir)
	const baselineDir = baselineMarker ? await getOptionalDirectoryHandle(projectDir, 'baseline') : null
	let runLabel = 'latest run'
	let runDir = null

	const runsDir = await getOptionalDirectoryHandle(projectDir, 'runs')
	if (runsDir && captureState.session) {
		runDir = await getOptionalDirectoryHandle(runsDir, captureState.session.runTimestamp)
		if (runDir) {
			runLabel = `run ${captureState.session.runTimestamp}`
		}
	}
	if (!runDir) {
		const latestRunDir = await findLatestRunDirectory(projectDir)
		if (latestRunDir) {
			runDir = latestRunDir
			runLabel = `run ${latestRunDir.name}`
		}
	}

	revokeCapturePreviewUrls()
	const baselineFiles = await listCaptureFiles(baselineDir)
	const runFiles = await listCaptureFiles(runDir)
	const names = Array.from(new Set([
		...baselineFiles.keys(),
		...runFiles.keys(),
	])).sort((left, right) => left.localeCompare(right))
	const comparisons = await buildCaptureComparisons(names, baselineFiles, runFiles)
	renderCaptureCompare(captureState.project.projectKey, runLabel, comparisons)
}

function createFileSystemCaptureHost(rootHandle, session, preparedTargetPromise = null) {
	const resolveTarget = createCaptureTargetResolver(rootHandle, session, preparedTargetPromise)
	const summary = {
		attempted: 0,
		succeeded: 0,
		failed: 0,
	}
	const capturedFiles = new Set()
	const handledRequests = new WeakSet()
	let finalized = false
	let finalizePromise = null

	return {
		getSummary() {
			return {
				attempted: summary.attempted,
				succeeded: summary.succeeded,
				failed: summary.failed,
				files: Array.from(capturedFiles),
			}
		},
		async handleCapture(request, blob) {
			if (finalized) {
				throw new Error(`Capture session ${session.id} is already finalized`)
			}

			handledRequests.add(request)
			summary.attempted++
			try {
				const target = await resolveTarget()
				const filename = buildCaptureFilename(request)
				if (capturedFiles.has(filename)) {
					throw new Error(`Duplicate capture filename in session ${session.id}: ${filename}`)
				}
				const file = await target.directory.getFileHandle(filename, { create: true })
				const writer = await file.createWritable()
				await writer.write(blob)
				await writer.close()
				summary.succeeded++
				capturedFiles.add(filename)
				console.log('[spx capture saved]', {
					projectKey: session.projectKey,
					sessionId: session.id,
					destination: target.destination,
					filename,
					request,
				})
				scheduleCaptureCompareRefresh()
			} catch (error) {
				summary.failed++
				throw error
			}
		},
		handleCaptureFailure(request) {
			if (handledRequests.has(request)) {
				return
			}
			handledRequests.add(request)
			summary.attempted++
			summary.failed++
		},
		completeSession(allowBaselineReady) {
			if (finalizePromise) {
				return finalizePromise
			}
			finalized = true
			finalizePromise = (async () => {
				const completedSummary = this.getSummary()
				if (!allowBaselineReady || completedSummary.attempted === 0 || completedSummary.failed > 0 || completedSummary.succeeded !== completedSummary.attempted) {
					return { baselineReady: false, ...completedSummary }
				}
				const target = await resolveTarget()
				if (!target.isBaseline) {
					return { baselineReady: false, ...completedSummary }
				}
				await markBaselineReady(target.projectDir, session, completedSummary)
				return { baselineReady: true, ...completedSummary }
			})()
			return finalizePromise
		},
	}
}

async function finalizeCaptureSession(session, allowBaselineReady = true) {
	if (!session) {
		return
	}
	if (session.finalizePromise) {
		return session.finalizePromise
	}

	session.finalizePromise = (async () => {
		const currentHost = typeof runnerWindow.spxGetCaptureHost === 'function' ? runnerWindow.spxGetCaptureHost() : null
		if (currentHost && session.hosts.has(currentHost) && typeof runnerWindow.spxSetCaptureHost === 'function') {
			runnerWindow.spxSetCaptureHost(null)
		}
		if (typeof runnerWindow.spxFlushCaptureQueue === 'function') {
			await runnerWindow.spxFlushCaptureQueue()
		}
		const hosts = Array.from(session.hosts)
		const activeHosts = hosts.filter((host) => host.getSummary().attempted > 0)
		const replayCommitted = !session.requiresInputReplay || session.inputReplaySaved
		const allSucceeded = allowBaselineReady && replayCommitted && session.canPublishBaselineMarker && activeHosts.length === 1 && activeHosts.every((host) => {
			const hostSummary = host.getSummary()
			return hostSummary.failed === 0 && hostSummary.succeeded === hostSummary.attempted
		})
		const results = []
		for (const host of hosts) {
			results.push(await host.completeSession(allSucceeded && host === activeHosts[0]))
		}
		scheduleCaptureCompareRefresh()
		return results
	})()
	return session.finalizePromise
}

function finalizeActiveCaptureSession() {
	return finalizeCaptureSession(captureState.session, true)
}

function syncCaptureHost() {
	if (typeof runnerWindow.spxSetCaptureHost !== 'function') {
		return false
	}
	if (!captureState.rootHandle || !captureState.session || captureState.session.finalizePromise) {
		runnerWindow.spxSetCaptureHost(null)
		return true
	}
	const session = captureState.session
	const preparedTargetPromise = session.captureTargetRootHandle === captureState.rootHandle
		? session.captureTargetPromise
		: null
	const host = createFileSystemCaptureHost(captureState.rootHandle, session.context, preparedTargetPromise)
	session.hosts.add(host)
	runnerWindow.spxSetCaptureHost(host)
	return true
}

async function selectCaptureDirectory() {
	if (runtimeState.phase !== 'stopped') {
		updateCaptureStatus('Output · Stop the game before changing folders', true)
		return
	}
	if (!browserSupportsCaptureDirectory()) {
		updateCaptureStatus('Output · Folder access unavailable', true)
		return
	}
	try {
		const rootHandle = await window.showDirectoryPicker(buildCaptureDirectoryPickerOptions())
		captureState.rootHandle = rootHandle
		const handleName = captureState.rootHandle && captureState.rootHandle.name ? captureState.rootHandle.name : 'selected'
		updateCaptureStatus(`Output · ${handleName}`)
		syncCaptureHost()
		scheduleCaptureCompareRefresh()
	} catch (error) {
		if (error && error.name === 'AbortError') {
			updateCaptureStatus('Output · Choose Downloads, Desktop, or another regular folder')
			return
		}
		console.error('Failed to select capture directory', error)
		updateCaptureStatus('Output · Selection failed; avoid system folders', true)
	}
}

function onProgress(value) {
	const complete = value >= 1
	const progress = document.getElementById('progress-bar')
	progress.value = value
	progress.hidden = complete
	document.getElementById('tab-game').hidden = !complete
	document.getElementById('tab-loader').hidden = complete
}

async function initEngine() {
	await runnerWindow.initEngine(null, { logLevel: LOG_LEVEL_VERBOSE })
	bindRunnerLifecycleEvents()
	syncCaptureHost()
}

let runnerLifecycleEventsBound = false

function bindRunnerLifecycleEvents() {
	if (runnerLifecycleEventsBound) return
	runnerLifecycleEventsBound = true

	runnerWindow.addEventListener('onProgress', (event) => {
		onProgress(event.detail.progress)
	})
	runnerWindow.onGameError((msg) => {
		console.error('onGameError', msg)
		if (captureState.session) {
			captureState.session.canPublishBaselineMarker = false
		}
	})
	runnerWindow.onGameExit((code) => {
		console.error('onGameExit', code)
		handleGameTermination(false, code)
	})
	runnerWindow.onEngineCrash((code) => {
		console.error('onEngineCrash', code)
		handleGameTermination(true, code)
	})
}

async function startGame(startOptions = {}) {
	await startProject(LAB_CONFIG.projects.primary, startOptions)
}

function normalizeStartOptions(startOptions) {
	if (!startOptions || typeof startOptions !== 'object' || Array.isArray(startOptions)) {
		throw new TypeError('startGame options must be an object')
	}
	const inputOptions = Object.prototype.hasOwnProperty.call(startOptions, 'input')
		? startOptions.input
		: undefined
	if (inputOptions === null) return { ...startOptions, input: null }
	if (inputOptions === undefined) {
		return { ...startOptions, input: runnerWindow.spxCreateInputSession() }
	}
	if (typeof inputOptions !== 'object' || Array.isArray(inputOptions)) {
		throw new TypeError('startGame input options must be an object')
	}
	return {
		...startOptions,
		input: runnerWindow.spxCreateInputSession(inputOptions.mode, inputOptions),
	}
}

async function createLaunchPlan(startOptions, project) {
	const options = normalizeStartOptions(startOptions)
	const input = options.input
	const modeName = input ? input.mode : 'normal'
	const mode = INPUT_MODES[modeName]
	if (!mode) throw new Error(`Unsupported input mode: ${modeName}`)
	if (modeName === 'normal') {
		return { mode: modeName, captureMode: mode.captureMode, runnerOptions: { ...options, input: null } }
	}

	requireInputReplayCaptureDirectory()
	if (modeName === 'record') {
		return {
			mode: modeName,
			captureMode: mode.captureMode,
			runnerOptions: options,
		}
	}

	const replay = input.data == null ? await loadInputReplayBaseline(project) : { data: input.data, name: 'provided replay' }
	return {
		mode: modeName,
		captureMode: mode.captureMode,
		replayName: replay.name,
		runnerOptions: {
			...options,
			input: { mode: 'replay', data: replay.data, captureKey: input.captureKey },
		},
	}
}

async function stopGame() {
	if (runtimeState.phase === 'stopped') return { inputReplay: null, stopped: false }
	if (runtimeState.phase === 'starting') {
		console.warn('Game startup is still in progress')
		return { inputReplay: null, stopped: false }
	}
	if (runtimeState.stopPromise) return runtimeState.stopPromise

	const run = runtimeState.activeRun
	if (!run) {
		stopReplayStatusPolling()
		runtimeState.phase = 'stopped'
		renderRuntimeControls()
		return { inputReplay: null, stopped: false }
	}
	stopReplayStatusPolling()
	runtimeState.phase = 'stopping'
	setRuntimeStatus(`Stopping ${run.mode} game...`)
	runtimeState.stopPromise = (async () => {
		try {
			const result = await runnerWindow.stopGame()
			if (run.mode === 'record') {
				await saveInputReplayBaseline(result && result.inputReplay, run.captureSession)
				run.captureSession.inputReplaySaved = true
			}
			await finalizeCaptureSession(run.captureSession, true)
			const suffix = run.mode === 'record' ? ' · input baseline saved' : ''
			setRuntimeStatus(`${INPUT_MODES[run.mode].label} game stopped${suffix}`)
			return result
		} catch (error) {
			if (run.captureSession) run.captureSession.canPublishBaselineMarker = false
			try {
				await finalizeCaptureSession(run.captureSession, false)
			} catch (finalizeError) {
				console.error('Failed to finalize capture session after stop error', finalizeError)
			}
			setRuntimeStatus(`Stop failed: ${error && error.message ? error.message : error}`, true)
			throw error
		} finally {
			runtimeState.phase = 'stopped'
			runtimeState.activeRun = null
			runtimeState.stopPromise = null
			renderRuntimeControls()
		}
	})()
	return runtimeState.stopPromise
}

function pauseGame() {
	if (runnerWindow.pauseGame) {
		runnerWindow.pauseGame()
	} else {
		console.warn('Pause function not available')
	}
}

function resumeGame() {
	if (runnerWindow.resumeGame) {
		runnerWindow.resumeGame()
	} else {
		console.warn('Resume function not available')
	}
}

function stepNextFrame() {
	if (runnerWindow.stepNextFrame) {
		runnerWindow.stepNextFrame()
	} else {
		console.warn('StepNextFrame function not available')
	}
}

function toggleSizeFrame() {
	const smallSize = { width: 480, height: 360 }
	const largeSize = { width: 960, height: 720 }
	const size = enlarged ? smallSize : largeSize
	ui.runner.width = size.width
	ui.runner.height = size.height
	enlarged = !enlarged
}

function handleGameTermination(crashed, code) {
	stopReplayStatusPolling()
	const run = runtimeState.activeRun
	if (!run) {
		runtimeState.phase = 'stopped'
		renderRuntimeControls()
		return
	}
	if (runtimeState.phase === 'stopping') {
		if (crashed) run.captureSession.canPublishBaselineMarker = false
		return
	}

	runtimeState.phase = 'stopped'
	runtimeState.activeRun = null
	const recordingWasDiscarded = run.mode === 'record'
	if (crashed || recordingWasDiscarded) {
		run.captureSession.canPublishBaselineMarker = false
	}
	finalizeCaptureSession(run.captureSession, !crashed && !recordingWasDiscarded).catch((error) => {
		console.error(`Failed to finalize capture session ${run.captureSession.id}`, error)
	})
	if (recordingWasDiscarded) {
		setRuntimeStatus('Record game ended without Stop; input baseline was discarded', true)
	} else {
		const reason = crashed ? `crashed (${code})` : 'ended'
		setRuntimeStatus(`${INPUT_MODES[run.mode].label} game ${reason}`, crashed)
	}
}

async function loadProjectArchive(zipUrl) {
	const response = await fetch(zipUrl)
	if (!response.ok) {
		throw new Error(`failed to load game package: ${response.status}`)
	}
	const zipped = await response.arrayBuffer()
	return fflate.unzipSync(new Uint8Array(zipped))
}

async function startProject(zipUrl, startOptions = {}) {
	if (runtimeState.phase !== 'stopped') {
		console.error('game is running')
		return
	}

	stopReplayStatusPolling()
	runtimeState.phase = 'starting'
	setRuntimeStatus('Starting game...')
	let run = null
	try {
		await finalizeActiveCaptureSession()
		const unzipped = await loadProjectArchive(zipUrl)
		const project = parseCaptureProject(unzipped)
		const launch = await createLaunchPlan(startOptions, project)
		beginCaptureSession(project, launch.captureMode)
		run = {
			mode: launch.mode,
			replayName: launch.replayName || '',
			captureSession: captureState.session,
		}
		runtimeState.activeRun = run
		await startProjectSession(unzipped, launch.runnerOptions, run.captureSession)
		if (runtimeState.activeRun === run && runtimeState.phase === 'starting') {
			runtimeState.phase = 'running'
			const input = launch.runnerOptions.input
			const captureHint = input && input.captureKey ? ` · screenshot ${input.captureKey}` : ''
			const replayHint = run.replayName ? ` ${run.replayName}` : ''
			setRuntimeStatus(`Running: ${run.mode}${replayHint}${captureHint}`)
			startReplayStatusPolling(run)
		}
	} catch (error) {
		stopReplayStatusPolling()
		if (run && run.captureSession) {
			run.captureSession.canPublishBaselineMarker = false
			try {
				await finalizeCaptureSession(run.captureSession, false)
			} catch (finalizeError) {
				console.error('Failed to finalize capture session after start error', finalizeError)
			}
		}
		runtimeState.phase = 'stopped'
		runtimeState.activeRun = null
		setRuntimeStatus(`Start failed: ${error && error.message ? error.message : error}`, true)
		throw error
	} finally {
		renderRuntimeControls()
	}
}

async function startProjectSession(unzipped, startOptions, captureSession) {
	runnerWindow.spxIsForceDebugLog = true
	await prepareBaselineCaptureTarget(captureState.rootHandle, captureSession)
	syncCaptureHost()
	configureAIInteraction(unzipped)
	if (captureState.rootHandle) {
		updateCaptureStatus(`Output · ${captureState.rootHandle.name} / ${captureSession.projectKey}`)
	}

	const files = {}
	for (const [path, data] of Object.entries(unzipped)) {
		if (!path.endsWith('/')) {
			files[path] = { lastModified: Date.now(), content: data.buffer }
		}
	}
	await runnerWindow.initGame(files)
	await runnerWindow.startGame(startOptions)
}

function resolveAIEndpointParam(value) {
	if (!value) return null
	try {
		const resolvedAIEndpoint = new URL(value, window.location.href)
		if (!['http:', 'https:'].includes(resolvedAIEndpoint.protocol)) {
			console.warn('Ignoring aiEndpoint query parameter with unsupported protocol', value)
			return null
		}
		if (resolvedAIEndpoint.origin !== window.location.origin) {
			console.warn('Ignoring cross-origin aiEndpoint query parameter', value)
			return null
		}
		return `${resolvedAIEndpoint.pathname}${resolvedAIEndpoint.search}${resolvedAIEndpoint.hash}`
	} catch (error) {
		console.warn('Ignoring invalid aiEndpoint query parameter', value, error)
		return null
	}
}

function configureAIInteraction(unzipped) {
	const params = new URLSearchParams(window.location.search)
	const aiEndpoint = resolveAIEndpointParam(params.get('aiEndpoint'))
	if (runnerWindow.xbuilder_set_ai_interaction_api_endpoint) {
		runnerWindow.xbuilder_set_ai_interaction_api_endpoint(aiEndpoint || '')
	}

	const descriptionFile = unzipped['builder-ai-description.md'] || unzipped['builder-ai-description']
	if (runnerWindow.xbuilder_set_ai_description) {
		const description = descriptionFile ? new TextDecoder('utf-8').decode(descriptionFile) : ''
		runnerWindow.xbuilder_set_ai_description(description)
	}
}
